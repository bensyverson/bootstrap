package session

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// kind names a log event.
type kind string

const (
	kindServer kind = "server" // a server started, which also opens a new session
	kindShow   kind = "show"
	kindBeat   kind = "beat" // a command ran, so the agent is alive
	kindAnswer kind = "answer"
	kindTabs   kind = "tabs"
	kindStop   kind = "stop"
	kindAgent  kind = "agent" // a serve from a different app, which changes the name people read
)

// event is one line of the log. Only the fields for its kind are set.
type event struct {
	Seq  int       `json:"seq"`
	At   time.Time `json:"at"`
	Kind kind      `json:"kind"`

	Session string `json:"session,omitempty"`
	Agent   string `json:"agent,omitempty"`
	PID     int    `json:"pid,omitempty"`

	Request *screen.Request `json:"request,omitempty"`
	Expect  time.Duration   `json:"expect,omitempty"`

	For    int            `json:"for,omitempty"`
	Answer *screen.Answer `json:"answer,omitempty"`

	Tabs int `json:"tabs,omitempty"`
}

func (s *State) apply(e event) {
	s.Seq = e.Seq
	switch e.Kind {
	case kindServer:
		s.ID = e.Session
		s.Tabs = 0
		if agent, err := screen.FindAgent(e.Agent); err == nil {
			s.Agent = agent
		}
		s.renew(e.At)
	case kindShow:
		req := *e.Request
		s.Current = &Shown{Seq: e.Seq, Request: req, At: e.At}
		s.Progress = screen.Progress(req, s.Chosen, s.Progress)
		allowance := DefaultAllowance
		if e.Expect > 0 {
			allowance = e.Expect
		}
		// A new screen replaces the last promise, so this is the one place the deadline can move earlier.
		s.Deadline = e.At.Add(allowance)
	case kindBeat:
		s.renew(e.At)
	case kindAnswer:
		s.applyAnswer(e)
	case kindTabs:
		s.Tabs = e.Tabs
	case kindStop:
		s.Ended = true
	case kindAgent:
		if agent, err := screen.FindAgent(e.Agent); err == nil {
			s.Agent = agent
		}
		s.renew(e.At)
	}
}

func (s *State) renew(at time.Time) {
	if deadline := at.Add(DefaultAllowance); deadline.After(s.Deadline) {
		s.Deadline = deadline
	}
}

func (s *State) applyAnswer(e event) {
	if s.Current == nil || s.Current.Seq != e.For {
		return
	}
	s.Answers = append(s.Answers, Answered{Seq: e.For, Request: s.Current.Request, Answer: *e.Answer, At: e.At})
	switch {
	case s.Current.Request.Screen == "list":
		s.Chosen = screen.KeepRequired(e.Answer.Tools)
	case s.Current.Request.Screen == "trouble" && e.Answer.Choice == "skip":
		kept := s.Chosen
		if kept == nil {
			kept = screen.KeepRequired(screen.AllToolIDs())
		}
		s.Chosen = slices.DeleteFunc(slices.Clone(kept), func(id string) bool { return id == s.Current.Request.Tool })
	}
}

// finished is the part of the log whose lines are whole. A line with no newline yet
// is a write in progress, or one a crash cut short: either way it never happened.
func finished(body []byte) []byte {
	return body[:bytes.LastIndexByte(body, '\n')+1]
}

func replay(path string, body []byte) (State, error) {
	var state State
	scanner := bufio.NewScanner(bytes.NewReader(finished(body)))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return State{}, &CorruptError{Path: path, Line: line}
		}
		state.apply(e)
	}
	return state, scanner.Err()
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// openLog opens the log without following a symlink, so nothing can point our writes elsewhere.
func (s Store) openLog(flags int) (*os.File, error) {
	f, err := os.OpenFile(s.LogPath(), flags|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if flags&os.O_CREATE != 0 {
		if err := f.Chmod(0o600); err != nil {
			f.Close()
			return nil, err
		}
	}
	return f, nil
}

// Load reads the session. It returns ErrNoSession when there has never been one.
func (s Store) Load() (State, error) {
	body, err := os.ReadFile(s.LogPath())
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, ErrNoSession
	}
	if err != nil {
		return State{}, err
	}
	return replay(s.LogPath(), body)
}

// mutate appends the event that build makes from the current state, with the
// log locked for the whole read and write, then returns the state after it.
func (s Store) mutate(flags int, now time.Time, build func(State) (event, error)) (State, error) {
	f, err := s.openLog(os.O_RDWR | os.O_APPEND | flags)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, ErrNoSession
	}
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return State{}, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	body, err := io.ReadAll(f)
	if err != nil {
		return State{}, err
	}
	state, err := replay(s.LogPath(), body)
	if err != nil {
		return State{}, err
	}
	// Holding the lock, a short line can only be a crash's leftovers, and appending
	// after it would glue two events into one unreadable line.
	if whole := finished(body); len(whole) < len(body) {
		if err := f.Truncate(int64(len(whole))); err != nil {
			return State{}, err
		}
	}
	e, err := build(state)
	if err != nil {
		return state, err
	}
	e.Seq, e.At = state.Seq+1, now
	line, err := json.Marshal(e)
	if err != nil {
		return State{}, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return State{}, err
	}
	state.apply(e)
	return state, nil
}

func newSessionID() (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// Begin records a server starting. It continues the session in the log, or starts
// a fresh one if the last was stopped, keeping the old log beside it.
func (s Store) Begin(agent screen.Agent, pid int, now time.Time) (State, error) {
	if err := ensureDir(s.Dir); err != nil {
		return State{}, err
	}
	previous, err := s.Load()
	if err != nil && !errors.Is(err, ErrNoSession) {
		return State{}, err
	}
	if err == nil && previous.Ended {
		archive := filepath.Join(s.Dir, fmt.Sprintf("session-%s.jsonl", previous.ID))
		if err := os.Rename(s.LogPath(), archive); err != nil {
			return State{}, err
		}
		previous = State{}
	}
	id := previous.ID
	if id == "" {
		if id, err = newSessionID(); err != nil {
			return State{}, err
		}
	}
	return s.mutate(os.O_CREATE, now, func(State) (event, error) {
		return event{Kind: kindServer, Session: id, Agent: agent.ID, PID: pid}, nil
	})
}

func requireLive(state State) error {
	if state.Ended {
		return ErrEnded
	}
	return nil
}

// Show puts a screen on the page.
func (s Store) Show(req screen.Request, expect time.Duration, now time.Time) (State, error) {
	if err := screen.Validate(req); err != nil {
		return State{}, err
	}
	return s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		if err := screen.CheckChosen(req, state.Chosen); err != nil {
			return event{}, err
		}
		return event{Kind: kindShow, Request: &req, Expect: expect}, nil
	})
}

// Renew notes that a command ran. With no session it does nothing at all, so
// reads that need no session leave no trace.
func (s Store) Renew(now time.Time) error {
	_, err := s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		return event{Kind: kindBeat}, nil
	})
	if errors.Is(err, ErrNoSession) || errors.Is(err, ErrEnded) {
		return nil
	}
	return err
}

// RecordAnswer saves a person's decision. The first answer to a screen stands, and
// an answer to any screen but the current one is refused.
func (s Store) RecordAnswer(seq int, ans screen.Answer, now time.Time) (State, error) {
	return s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		if state.Current == nil || state.Current.Seq != seq {
			return event{}, ErrStale
		}
		if _, answered := state.CurrentAnswer(); answered {
			return event{}, ErrAnswered
		}
		desc, err := screen.Describe(state.Current.Request, state.context())
		if err != nil {
			return event{}, err
		}
		if name, ok := screen.FolderName(ans.Name); ok && desc.Field != nil {
			ans.Name = name
		}
		if err := screen.ValidateAnswer(desc, ans); err != nil {
			return event{}, err
		}
		return event{Kind: kindAnswer, For: seq, Answer: &ans}, nil
	})
}

// SetAgent changes the app the page names, and counts as the agent checking in.
func (s Store) SetAgent(agent screen.Agent, now time.Time) error {
	_, err := s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		return event{Kind: kindAgent, Agent: agent.ID}, nil
	})
	return err
}

// Stop ends the session. Stopping a session that already ended returns ErrEnded.
func (s Store) Stop(now time.Time) (State, error) {
	return s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		return event{Kind: kindStop}, nil
	})
}

// SetTabs records how many tabs have the page open, so a later serve knows whether to open one.
func (s Store) SetTabs(n int, now time.Time) error {
	_, err := s.mutate(0, now, func(state State) (event, error) {
		if err := requireLive(state); err != nil {
			return event{}, err
		}
		return event{Kind: kindTabs, Tabs: n}, nil
	})
	return err
}
