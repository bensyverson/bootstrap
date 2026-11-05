package session

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
)

// Event is what happened to an install.
type Event string

const (
	// Started is the person's go-ahead: for Apple's tools, clicking Agree.
	Started Event = "started"
	// Arrived is the tool's file showing up, which means it's done.
	Arrived Event = "arrived"
	// Canceled is the installer quitting before the tool arrived.
	Canceled Event = "canceled"
)

// Progress is the first thing that happened, and the path that proved an arrival.
type Progress struct {
	Event Event
	Path  string
}

// WaitInstall returns the first of a tool's arrival, its start, or its cancel. A start
// counts only for a folder that appears during this wait, so a folder left by an
// earlier try doesn't fire. Nor does any folder while an install is already under way
// (a folder there and the installer running when the wait began): Apple can download
// two versions into two folders a moment apart, and the second isn't a new start. A
// cancel counts only for an installer this wait saw running. It renews a session when it can, but needs
// none, so the chat fallback can use it where this folder can't even be written.
func (s Store) WaitInstall(ctx context.Context, probe detect.Probe, toolID string, timeout time.Duration) (Progress, error) {
	tool, ok := catalog.Find(toolID)
	if !ok {
		return Progress{}, errors.New("unknown tool " + toolID)
	}
	var before map[string]bool
	watchStart := tool.Started != nil
	if watchStart {
		before = detect.Downloads(probe, *tool.Started)
		watchStart = len(before) == 0 || !detect.InstallerRunning(probe, tool)
	}
	seen := false
	var got Progress
	renew := func() error {
		s.Renew(time.Now())
		return nil
	}
	err := poll(ctx, timeout, renew, func() (bool, error) {
		if path, ok := detect.Locate(probe, tool.Detect); ok {
			got = Progress{Event: Arrived, Path: path}
			return true, nil
		}
		if watchStart {
			now := detect.Downloads(probe, *tool.Started)
			maps.DeleteFunc(now, func(dir string, _ bool) bool { return before[dir] })
			if len(now) > 0 {
				got = Progress{Event: Started}
				return true, nil
			}
		}
		running := detect.InstallerRunning(probe, tool)
		if seen && !running {
			got = Progress{Event: Canceled}
			return true, nil
		}
		seen = seen || running
		return false, nil
	})
	return got, err
}
