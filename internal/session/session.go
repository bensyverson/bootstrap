// Package session is the append-only log that the commands, the server and the
// page share. Every command reads and writes this file under a lock, so none of
// them needs to talk to any other, and the file is the whole truth.
package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// Timing the trial may change.
const (
	DefaultAllowance   = 3 * time.Minute
	DefaultWaitTimeout = 90 * time.Second
	LostLifetime       = time.Hour
)

// Sentinel errors a command turns into a teaching message.
var (
	ErrNoSession     = errors.New("no session")
	ErrEnded         = errors.New("session ended")
	ErrStale         = errors.New("that screen is gone")
	ErrAnswered      = errors.New("that screen already has an answer")
	ErrNoScreen      = errors.New("no screen shown yet")
	ErrNoChoices     = errors.New("this screen has nothing to answer")
	ErrWaitTimeout   = errors.New("still waiting")
	ErrPageGone      = errors.New("the page is closed")
	ErrServerRunning = errors.New("a server is already running")
)

// CorruptError is a log line that can't be read.
type CorruptError struct {
	Path string
	Line int
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("%s line %d can't be read", e.Path, e.Line)
}

// Store is bootstrap's private folder.
type Store struct {
	Dir string
}

// LogPath is the current session's log.
func (s Store) LogPath() string { return filepath.Join(s.Dir, "session.jsonl") }

// Shown is the screen on the page.
type Shown struct {
	Seq     int            `json:"seq"`
	Request screen.Request `json:"request"`
	At      time.Time      `json:"at"`
}

// Answered is a person's decision and the screen it was for.
type Answered struct {
	Seq     int            `json:"seq"`
	Request screen.Request `json:"request"`
	Answer  screen.Answer  `json:"answer"`
	At      time.Time      `json:"at"`
}

// State is the log replayed.
type State struct {
	ID       string
	Agent    screen.Agent
	Ended    bool
	Seq      int
	Current  *Shown
	Answers  []Answered
	Chosen   []string
	Deadline time.Time
	Tabs     int
	Progress float64
}

// Frame is what the server sends the page. Screen is nil before the agent's first show.
type Frame struct {
	Seq      int                 `json:"seq"`
	Lost     bool                `json:"lost"`
	Answered bool                `json:"answered"`
	Ended    bool                `json:"ended"`
	Screen   *screen.Description `json:"screen"`
}

// CurrentAnswer returns the answer to the screen on the page, if it has one.
// Answers to earlier screens stay in Answers for status, and are never current.
func (s State) CurrentAnswer() (Answered, bool) {
	if s.Current == nil {
		return Answered{}, false
	}
	for _, a := range s.Answers {
		if a.Seq == s.Current.Seq {
			return a, true
		}
	}
	return Answered{}, false
}

// Lost reports whether the page should say the agent has gone. Past the allowance
// that holds, unless a person may still be reading a screen they can click on.
func (s State) Lost(now time.Time) bool {
	if s.Ended {
		return false
	}
	if s.Current != nil {
		if screen.IsSettled(s.Current.Request.Screen) {
			return false
		}
		_, answered := s.CurrentAnswer()
		if screen.AwaitsClick(s.Current.Request.Screen) && !answered {
			return false
		}
	}
	return now.After(s.Deadline)
}

// PastLifetime reports whether the agent has been gone so long that the server should exit.
func (s State) PastLifetime(now time.Time) bool {
	return !s.Ended && now.After(s.Deadline.Add(LostLifetime))
}

func (s State) context() screen.Context {
	return screen.Context{Agent: s.Agent, Chosen: s.Chosen, Carried: s.Progress}
}

// Frame is what the page should show at this moment.
func (s State) Frame(now time.Time) Frame {
	frame := Frame{Ended: s.Ended}
	if s.Current != nil {
		frame.Seq = s.Current.Seq
		_, frame.Answered = s.CurrentAnswer()
	}
	if s.Lost(now) {
		lost := screen.DescribeLost(s.context())
		frame.Lost, frame.Screen = true, &lost
		return frame
	}
	if s.Current != nil {
		// The log only holds requests that passed Validate, so this can't fail for a log we wrote.
		if desc, err := screen.Describe(s.Current.Request, s.context()); err == nil {
			frame.Screen = &desc
		}
	}
	return frame
}
