package session

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"time"
)

// Update is one thing the flow did: a step, a note or a handoff.
type Update struct {
	Seq     int      `json:"seq"`
	Step    *Step    `json:"step,omitempty"`
	Note    string   `json:"note,omitempty"`
	Handoff *Handoff `json:"handoff,omitempty"`
}

// Told is what one wait tells the agent.
type Told struct {
	State State
	// Updates are the flow's work since the agent's last wait, oldest first.
	Updates []Update
	// Open is a handoff that waits on the agent. It's repeated on every wait until a resume.
	Open *Open
	// Finished is a setup whose end an earlier wait already told.
	Finished bool
}

// Unread returns the session and the flow's work since the agent was last told.
//
// The cursor is a told event in the log rather than a file of its own, so it is
// written under the same lock as everything else, survives a restart, and shows
// in the history when the agent was told what.
func (s Store) Unread() (State, []Update, error) {
	body, err := os.ReadFile(s.LogPath())
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil, ErrNoSession
	}
	if err != nil {
		return State{}, nil, err
	}
	var state State
	var updates []Update
	err = walk(s.LogPath(), body, func(e event) {
		state.apply(e)
		switch e.Kind {
		case kindStep:
			updates = append(updates, Update{Seq: e.Seq, Step: e.Step})
		case kindNote:
			updates = append(updates, Update{Seq: e.Seq, Note: e.Text})
		case kindHandoff:
			updates = append(updates, Update{Seq: e.Seq, Handoff: e.Handoff})
		case kindTold:
			updates = slices.DeleteFunc(updates, func(u Update) bool { return u.Seq <= e.For })
		}
	})
	if err != nil {
		return State{}, nil, err
	}
	return state, updates, nil
}

// MarkTold records that the agent has been told everything up to seq.
func (s Store) MarkTold(seq int, now time.Time) error {
	_, err := s.appendLive(now, func(State) (event, error) { return event{Kind: kindTold, For: seq}, nil })
	return err
}

// WaitFlow blocks until the flow hands something to the agent, or timeout passes,
// and returns the flow's work since the last wait. Running out of time returns
// ErrWaitTimeout with what happened meanwhile. A blocking handoff is taken, and
// everything returned is marked told, so the next wait starts after it.
func (s Store) WaitFlow(ctx context.Context, timeout time.Duration) (Told, error) {
	var told Told
	look := func() (bool, error) {
		state, updates, err := s.Unread()
		if err != nil {
			return false, err
		}
		if state.Ended {
			return false, ErrEnded
		}
		if state.Halted != "" {
			return false, haltErr(state)
		}
		told = Told{State: state, Updates: updates, Open: state.Open}
		if state.Open != nil || slices.ContainsFunc(updates, func(u Update) bool { return u.Handoff != nil }) {
			return true, nil
		}
		if slices.ContainsFunc(state.Handoffs, func(l Logged) bool { return l.Handoff.Kind == HandoffDone }) {
			told.Finished = true
			return true, nil
		}
		if !s.ServerRunning() {
			return false, ErrPageGone
		}
		return false, nil
	}
	renew := func() error { return s.Renew(time.Now()) }
	err := poll(ctx, timeout, renew, look)
	if errors.Is(err, ErrWaitTimeout) {
		// What happened while waiting is still news, so it's read once more and told.
		handedOff, lookErr := look()
		if lookErr != nil {
			return Told{}, lookErr
		}
		if handedOff {
			err = nil
		}
	} else if err != nil {
		return Told{}, err
	}
	if told.Open != nil && !told.Open.Taken {
		state, takeErr := s.TakeHandoff(time.Now())
		if takeErr != nil {
			return Told{}, takeErr
		}
		told.Open = state.Open
	}
	if n := len(told.Updates); n > 0 {
		if markErr := s.MarkTold(told.Updates[n-1].Seq, time.Now()); markErr != nil {
			return Told{}, markErr
		}
	}
	return told, err
}
