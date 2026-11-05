package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// A flow runs for as long as the server does, under the same exit rules, and the
// server's exit stops it.
func TestRunDrivesTheFlowAndStopsItOnExit(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	stopped := make(chan error, 1)
	flow := func(ctx context.Context) error {
		if _, err := store.Show(screen.Request{Screen: "fast"}, 0, time.Now()); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		stopped <- ctx.Err()
		return ctx.Err()
	}
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: 5 * time.Millisecond, FirstTabGrace: 100 * time.Millisecond, Flow: flow})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ExitReason, 1)
	go func() { done <- srv.Run() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the flow never started")
	}
	select {
	case got := <-done:
		if got != ExitNoTab {
			t.Errorf("exit = %v: a flow doesn't change the tab rules", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server never exited")
	}
	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("flow ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the flow outlived its server")
	}
}

func TestAFlowErrorIsKept(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	broken := errors.New("the log can't be written")
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: 5 * time.Millisecond, FirstTabGrace: 50 * time.Millisecond,
		Flow: func(context.Context) error { return broken }})
	if err != nil {
		t.Fatal(err)
	}
	if got := srv.Run(); got != ExitNoTab {
		t.Errorf("exit = %v", got)
	}
	if !errors.Is(srv.FlowErr(), broken) {
		t.Errorf("flow err = %v", srv.FlowErr())
	}
}
