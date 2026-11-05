package session

import (
	"context"
	"errors"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
)

const (
	pollInterval = 100 * time.Millisecond
	// beatInterval keeps a long wait counting as an agent that's still there.
	beatInterval = 30 * time.Second
)

// poll calls check until it says done, calling renew along the way.
func poll(ctx context.Context, timeout time.Duration, renew func() error, check func() (bool, error)) error {
	if err := renew(); err != nil {
		return err
	}
	deadline := time.After(timeout)
	beat := time.NewTicker(beatInterval)
	defer beat.Stop()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		done, err := check()
		if done || err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return ErrWaitTimeout
		case <-beat.C:
			if err := renew(); err != nil {
				return err
			}
		case <-tick.C:
		}
	}
}

// WaitAnswer returns the answer to the screen on the page, which may have arrived
// before this ran. It re-reads the log every time, so a screen that moves on takes
// the wait with it, and an answer to an earlier screen can't be returned.
func (s Store) WaitAnswer(ctx context.Context, timeout time.Duration) (Answered, error) {
	var found Answered
	renew := func() error { return s.Renew(time.Now()) }
	err := poll(ctx, timeout, renew, func() (bool, error) {
		state, err := s.Load()
		if err != nil {
			return false, err
		}
		if state.Ended {
			return false, ErrEnded
		}
		if state.Current == nil {
			return false, ErrNoScreen
		}
		if !screen.AwaitsClick(state.Current.Request.Screen) {
			return false, ErrNoChoices
		}
		if answer, ok := state.CurrentAnswer(); ok {
			found = answer
			return true, nil
		}
		if !s.ServerRunning() {
			return false, ErrPageGone
		}
		return false, nil
	})
	return found, err
}

// WaitArrival returns the path that proves a tool has landed. It renews a session
// when it can, but needs none, so the chat fallback can use it where this folder
// can't even be written.
func (s Store) WaitArrival(ctx context.Context, probe detect.Probe, toolID string, timeout time.Duration) (string, error) {
	tool, ok := catalog.Find(toolID)
	if !ok {
		return "", errors.New("unknown tool " + toolID)
	}
	var path string
	renew := func() error {
		s.Renew(time.Now())
		return nil
	}
	err := poll(ctx, timeout, renew, func() (bool, error) {
		found, ok := detect.Locate(probe, tool.Detect)
		path = found
		return ok, nil
	})
	return path, err
}
