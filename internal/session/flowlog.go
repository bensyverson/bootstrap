package session

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// The flow's own errors.
var (
	ErrNoHandoff   = errors.New("nothing is waiting on the agent")
	ErrHandoffOpen = errors.New("a handoff is already waiting on the agent")
)

// StepID names one step of the setup flow. The steps themselves are internal/flow's.
type StepID string

// StepStatus is where a step has got to.
type StepStatus string

const (
	StepStarted   StepStatus = "started"
	StepDone      StepStatus = "done"
	StepSkipped   StepStatus = "skipped"
	StepHandedOff StepStatus = "handed-off"
)

// Step is the flow's place in the setup: Index of Of, counting from 1.
type Step struct {
	ID     StepID     `json:"id"`
	Status StepStatus `json:"status"`
	Index  int        `json:"index"`
	Of     int        `json:"of"`
}

// Note is a line the agent can pass on as it is, such as "installed jobs".
type Note struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// HandoffKind is why the flow turned to the agent.
type HandoffKind string

const (
	// HandoffTrouble is something the flow didn't expect. It waits for a resume.
	HandoffTrouble HandoffKind = "trouble"
	// HandoffPhone is the person asking to use the agent from their phone, which the agent explains.
	HandoffPhone HandoffKind = "phone"
	// HandoffDone is the end of the setup, which the agent sums up in the chat.
	HandoffDone HandoffKind = "done"
)

// HandoffReason is what went wrong, for trouble.
type HandoffReason string

const (
	ReasonExit      HandoffReason = "exit"       // a command exited with a code the flow doesn't expect
	ReasonTimeout   HandoffReason = "timeout"    // a command or an install ran far past its time
	ReasonNoStart   HandoffReason = "no-start"   // a command couldn't be started at all
	ReasonCanceled  HandoffReason = "canceled"   // an installer quit before its tool arrived
	ReasonDiskShort HandoffReason = "disk-short" // too little free space for Apple's tools
	ReasonUnsigned  HandoffReason = "unsigned"   // a download isn't signed by who it should be
	ReasonMissing   HandoffReason = "missing"    // a command said it worked, but what it makes isn't there
	ReasonInTheWay  HandoffReason = "in-the-way" // something that isn't a folder sits where the project goes
)

// Way is a way forward from a handoff.
type Way string

const (
	WayRetry Way = "retry"
	WaySkip  Way = "skip"
	// WayTalk is talking it over with the person in the chat. It's offered, but
	// it isn't a resume: the agent resumes with retry or skip afterwards.
	WayTalk Way = "talk"
)

// Evidence is what the agent needs to see what happened without running anything.
type Evidence struct {
	Command string        `json:"command,omitempty"`
	Exit    int           `json:"exit,omitempty"`
	Elapsed time.Duration `json:"elapsed,omitempty"`
	Limit   time.Duration `json:"limit,omitempty"`
	Log     string        `json:"log,omitempty"`
	Tail    []string      `json:"tail,omitempty"`
}

// Handoff is the flow turning to the agent: what happened, the evidence, and the ways forward.
type Handoff struct {
	Kind     HandoffKind   `json:"kind"`
	Step     StepID        `json:"step"`
	Tool     string        `json:"tool,omitempty"`
	Reason   HandoffReason `json:"reason,omitempty"`
	What     string        `json:"what"`
	Evidence Evidence      `json:"evidence,omitzero"`
	Ways     []Way         `json:"ways,omitempty"`
}

// Blocks reports whether the flow waits for the agent to resume.
func (h Handoff) Blocks() bool { return h.Kind == HandoffTrouble }

// Logged is a handoff and where it sits in the log.
type Logged struct {
	Seq     int       `json:"seq"`
	At      time.Time `json:"at"`
	Handoff Handoff   `json:"handoff"`
}

// Open is a blocking handoff the agent hasn't resumed. Taken says the agent has seen it.
type Open struct {
	Logged
	Taken bool `json:"taken"`
}

// Resumed is the agent's last way forward, and the handoff it answered.
type Resumed struct {
	Seq int `json:"seq"`
	For int `json:"for"`
	Way Way `json:"way"`
}

var (
	kinds   = []HandoffKind{HandoffTrouble, HandoffPhone, HandoffDone}
	reasons = []HandoffReason{ReasonExit, ReasonTimeout, ReasonNoStart, ReasonCanceled, ReasonDiskShort, ReasonUnsigned, ReasonMissing, ReasonInTheWay}
	ways    = []Way{WayRetry, WaySkip, WayTalk}
)

// check refuses a handoff that couldn't teach the agent anything.
func (h Handoff) check() error {
	switch {
	case !slices.Contains(kinds, h.Kind):
		return fmt.Errorf("unknown handoff kind %q", h.Kind)
	case h.Step == "":
		return errors.New("a handoff names its step")
	case h.What == "":
		return errors.New("a handoff says what happened")
	case h.Blocks() && !slices.Contains(reasons, h.Reason):
		return fmt.Errorf("unknown handoff reason %q", h.Reason)
	case !h.Blocks() && h.Reason != "":
		return fmt.Errorf("a %s handoff takes no reason", h.Kind)
	case h.Blocks() && len(h.Ways) == 0:
		return errors.New("trouble offers at least one way forward")
	}
	for _, w := range h.Ways {
		if !slices.Contains(ways, w) {
			return fmt.Errorf("unknown way %q", w)
		}
	}
	return nil
}

// resumable is what resume takes for this handoff: every way offered, except talk.
func (h Handoff) resumable() []string {
	var out []string
	for _, w := range h.Ways {
		if w != WayTalk {
			out = append(out, string(w))
		}
	}
	return out
}

func (s *State) applyFlow(e event) {
	switch e.Kind {
	case kindDrive:
		s.Driving = true
		s.Halted = ""
	case kindHalt:
		s.Driving = false
		s.Halted = e.Text
	case kindStep:
		step := *e.Step
		s.Step = &step
		if step.Status == StepDone || step.Status == StepSkipped {
			if s.Finished == nil {
				s.Finished = map[StepID]StepStatus{}
			}
			s.Finished[step.ID] = step.Status
		}
	case kindNote:
		s.Notes = append(s.Notes, Note{Seq: e.Seq, At: e.At, Text: e.Text})
	case kindHandoff:
		logged := Logged{Seq: e.Seq, At: e.At, Handoff: *e.Handoff}
		s.Handoffs = append(s.Handoffs, logged)
		if logged.Handoff.Blocks() {
			s.Open = &Open{Logged: logged}
			// A handoff is a new promise that the agent will look, like a new screen.
			s.Deadline = e.At.Add(DefaultAllowance)
		}
	case kindTold:
		s.renew(e.At)
	case kindTake:
		if s.Open != nil {
			s.Open.Taken = true
		}
		s.renew(e.At)
	case kindResume:
		if s.Open == nil {
			return
		}
		h := s.Open.Handoff
		s.Resumed = &Resumed{Seq: e.Seq, For: s.Open.Seq, Way: e.Way}
		s.Open = nil
		if e.Way == WaySkip && h.Tool != "" {
			s.dropTool(h.Tool)
		}
		s.renew(e.At)
	}
}

// dropTool takes a tool out of the person's picks, as skipping it does.
func (s *State) dropTool(id string) {
	kept := s.Chosen
	if kept == nil {
		kept = screen.KeepRequired(screen.AllToolIDs())
	}
	s.Chosen = slices.DeleteFunc(slices.Clone(kept), func(t string) bool { return t == id })
}

func (s Store) appendLive(now time.Time, build func(State) (event, error)) (State, error) {
	return s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		return build(state)
	})
}

// Drive records that a flow is running the setup in the server.
func (s Store) Drive(now time.Time) (State, error) {
	return s.appendLive(now, func(State) (event, error) { return event{Kind: kindDrive}, nil })
}

// RecordStep records the flow's place.
func (s Store) RecordStep(step Step, now time.Time) (State, error) {
	return s.appendLive(now, func(State) (event, error) { return event{Kind: kindStep, Step: &step}, nil })
}

// Note records a line for the agent to retell.
func (s Store) Note(text string, now time.Time) (State, error) {
	return s.appendLive(now, func(State) (event, error) { return event{Kind: kindNote, Text: text}, nil })
}

// HandOff records the flow turning to the agent. Only one handoff that blocks can be open.
func (s Store) HandOff(h Handoff, now time.Time) (State, error) {
	if err := h.check(); err != nil {
		return State{}, err
	}
	return s.appendLive(now, func(state State) (event, error) {
		if h.Blocks() && state.Open != nil {
			return event{}, ErrHandoffOpen
		}
		return event{Kind: kindHandoff, Handoff: &h}, nil
	})
}

// TakeHandoff records that the agent has seen the open handoff. From then on the
// chat screen waits on the person, so the agent's silence means nothing.
func (s Store) TakeHandoff(now time.Time) (State, error) {
	return s.appendLive(now, func(state State) (event, error) {
		if state.Open == nil {
			return event{}, ErrNoHandoff
		}
		return event{Kind: kindTake}, nil
	})
}

// Resume answers the open handoff with one of the ways it offered. Skipping a
// tool drops it from the person's picks.
func (s Store) Resume(way Way, now time.Time) (State, error) {
	return s.appendLive(now, func(state State) (event, error) {
		if state.Open == nil {
			return event{}, ErrNoHandoff
		}
		valid := state.Open.Handoff.resumable()
		if !slices.Contains(valid, string(way)) {
			return event{}, &screen.InvalidError{Message: fmt.Sprintf("this handoff can't be resumed with %q", way), Valid: valid, Next: "bootstrap status"}
		}
		return event{Kind: kindResume, Way: way}, nil
	})
}
