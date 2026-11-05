package server

import (
	"context"
	"errors"
	"fmt"
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
	if state, _ := store.Load(); state.Halted != "" {
		t.Errorf("a flow the server stopped halted: %q", state.Halted)
	}
}

// A flow that stops early says why in the log and takes the server down with it,
// so a wait reports it at once instead of "still working" until the server exits.
// (This test used to expect ExitNoTab: the server kept serving a page with no flow behind it.)
func TestAFlowErrorHaltsTheServer(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	broken := errors.New("the screens are missing")
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: time.Hour, FirstTabGrace: time.Hour,
		Flow: func(context.Context) error { return broken }})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ExitReason, 1)
	go func() { done <- srv.Run() }()
	select {
	case got := <-done:
		if got != ExitFlowStopped {
			t.Errorf("exit = %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server outlived its flow")
	}
	if !errors.Is(srv.FlowErr(), broken) {
		t.Errorf("flow err = %v", srv.FlowErr())
	}
	if state, err := store.Load(); err != nil || state.Halted != broken.Error() {
		t.Errorf("halted = %q, err = %v", state.Halted, err)
	}
}

// A flow that reaches the end leaves the page up for the end screen.
func TestAFinishedFlowKeepsTheServerUp(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: 5 * time.Millisecond, FirstTabGrace: 100 * time.Millisecond,
		Flow: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if got := srv.Run(); got != ExitNoTab {
		t.Errorf("exit = %v", got)
	}
	if state, _ := store.Load(); state.Halted != "" {
		t.Errorf("halted = %q", state.Halted)
	}
}

// A write through the server's bell reaches the page at once, not at the next tick.
func TestTheBellSendsAFrameWithoutATick(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap"), Bell: session.NewBell()}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: time.Hour, FirstTabGrace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Run()
	t.Cleanup(srv.Close)
	f := fixture{srv: srv, store: store, host: fmt.Sprintf("127.0.0.1:%d", srv.Port())}
	e := f.connect(t)
	defer e.close()
	e.next(t)
	if _, err := store.Show(screen.Request{Screen: "list"}, 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := e.next(t); got.Screen == nil || got.Screen.Screen != "list" {
		t.Errorf("frame = %+v", got)
	}
}
