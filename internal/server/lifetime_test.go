package server

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

func TestLostPastTheAllowanceWhileWorking(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	e := f.connect(t)
	defer e.close()
	if got := e.next(t); got.Lost {
		t.Fatal("too soon")
	}
	f.walk(t0, t0.Add(session.DefaultAllowance))
	select {
	case got := <-e.frames:
		t.Fatalf("not past the allowance yet: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
	f.srv.Step(t0.Add(session.DefaultAllowance + time.Second))
	got := e.next(t)
	if !got.Lost || got.Screen.Screen != screen.Lost {
		t.Fatalf("frame = %+v", got)
	}
	if title := got.Screen.Title[0].Text; title != "Whoops! We lost your Claude Code session." {
		t.Errorf("title = %q", title)
	}
}

func TestNextCommandBringsThePageBack(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	later := t0.Add(10 * time.Minute)
	f.walk(t0, later)
	if !e.next(t).Lost {
		t.Fatal("should be lost")
	}
	if err := f.store.Renew(later); err != nil {
		t.Fatal(err)
	}
	f.srv.Step(later)
	got := e.next(t)
	if got.Lost || got.Screen.Screen != "tool" {
		t.Errorf("frame = %+v", got)
	}
}

func TestLostWhileWaitingOnlyAfterAClick(t *testing.T) {
	f := start(t)
	state := f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	later := t0.Add(30 * time.Minute)
	f.clock.Set(later)
	f.walk(t0, later)
	select {
	case got := <-e.frames:
		t.Fatalf("a person is reading, but got %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
	code, body := f.post(t, fmt.Sprintf(`{"seq":%d,"choice":"see-plan"}`, state.Current.Seq))
	if code != 200 {
		t.Fatalf("status %d: %s", code, body)
	}
	if got := e.next(t); !got.Lost || got.Screen.Screen != screen.Lost {
		t.Errorf("frame = %+v", got)
	}
	// The click is saved, so a later status reports it.
	loaded, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Answers) != 1 || loaded.Answers[0].Answer.Choice != "see-plan" {
		t.Errorf("answers = %+v", loaded.Answers)
	}
}

func TestClickWithAnAgentMarksTheFrameAnswered(t *testing.T) {
	f := start(t)
	state := f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	code, _ := f.post(t, fmt.Sprintf(`{"seq":%d,"choice":"see-plan"}`, state.Current.Seq))
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if got := e.next(t); !got.Answered || got.Lost {
		t.Errorf("frame = %+v", got)
	}
}

func TestStopSendsEndedAndExits(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	if _, err := f.store.Stop(t0); err != nil {
		t.Fatal(err)
	}
	if got := f.srv.Step(t0); got != ExitStopped {
		t.Errorf("exit = %v", got)
	}
	if got := e.next(t); !got.Ended {
		t.Errorf("frame = %+v", got)
	}
}

func TestExitsWhenTheTabCloses(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	e.next(t)
	if got := f.srv.Step(t0.Add(time.Hour)); got != ExitNone {
		t.Fatalf("a tab is open and nothing is up: %v", got)
	}
	e.close()
	waitFor(t, func() bool { return f.srv.Tabs() == 0 })
	closedAt := t0.Add(time.Hour)
	f.srv.Step(closedAt)
	if got := f.srv.Step(closedAt.Add(ClosedTabGrace)); got != ExitNone {
		t.Fatalf("a reload is not a close: %v", got)
	}
	// Only one reading right after the grace, so the sleep rule can't absorb it.
	if got := f.srv.Step(closedAt.Add(ClosedTabGrace + time.Second)); got != ExitTabClosed {
		t.Errorf("exit = %v", got)
	}
}

func TestAReloadWithinTheGraceDoesNotExit(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	e.next(t)
	e.close()
	waitFor(t, func() bool { return f.srv.Tabs() == 0 })
	f.srv.Step(t0.Add(time.Second))
	again := f.connect(t)
	defer again.close()
	again.next(t)
	if got := f.srv.Step(t0.Add(2 * time.Second)); got != ExitNone {
		t.Errorf("exit = %v", got)
	}
	if got := f.srv.Step(t0.Add(ClosedTabGrace + 3*time.Second)); got != ExitNone {
		t.Errorf("exit = %v", got)
	}
}

func TestExitsWhenNoTabEverOpens(t *testing.T) {
	f := start(t)
	if err := f.store.Renew(t0.Add(FirstTabGrace)); err != nil {
		t.Fatal(err)
	}
	if got := f.walk(t0, t0.Add(FirstTabGrace)); got != ExitNone {
		t.Fatalf("exit = %v", got)
	}
	if got := f.srv.Step(t0.Add(FirstTabGrace + time.Second)); got != ExitNoTab {
		t.Errorf("exit = %v", got)
	}
}

func TestExitsWhenTheHourIsUp(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	deadline := t0.Add(session.DefaultAllowance)
	if got := f.walk(t0, deadline.Add(session.LostLifetime)); got != ExitNone {
		t.Fatalf("exit = %v", got)
	}
	if got := f.srv.Step(deadline.Add(session.LostLifetime + time.Second)); got != ExitHourUp {
		t.Errorf("exit = %v", got)
	}
}

func TestChatOutlastsTheHour(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "chat"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	end := t0.Add(session.DefaultAllowance + session.LostLifetime + time.Hour)
	if got := f.walk(t0, end); got != ExitNone {
		t.Errorf("exit = %v, and an exit makes the page draw the lost screen", got)
	}
}

func TestAnHourWithALiveAgentIsFine(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	at := t0
	for i := range 80 {
		at = at.Add(time.Minute)
		if err := f.store.Renew(at); err != nil {
			t.Fatal(err)
		}
		if got := f.srv.Step(at); got != ExitNone {
			t.Fatalf("minute %d: exit = %v", i, got)
		}
	}
}

func TestSleepDoesNotCountAsTheAgentLeaving(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	f.srv.Step(t0.Add(time.Second))
	// The machine sleeps for eight hours: one reading long after the last.
	woke := t0.Add(8 * time.Hour)
	if got := f.srv.Step(woke); got != ExitNone {
		t.Fatalf("exit = %v", got)
	}
	select {
	case got := <-e.frames:
		t.Fatalf("waking up isn't losing the agent: %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
	// Awake time past the allowance still counts.
	if f.walk(woke, woke.Add(session.DefaultAllowance+10*time.Second)); !e.next(t).Lost {
		t.Error("the agent really is gone now")
	}
}

func TestSleepCreditEndsWhenTheAgentChecksIn(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	f.srv.Step(t0.Add(time.Second))
	woke := t0.Add(8 * time.Hour)
	f.srv.Step(woke)
	if err := f.store.Renew(woke); err != nil {
		t.Fatal(err)
	}
	f.srv.Step(woke.Add(time.Second))
	f.srv.Step(woke.Add(20 * time.Second))
	f.srv.Step(woke.Add(40 * time.Second))
	f.srv.Step(woke.Add(60 * time.Second))
	f.srv.Step(woke.Add(80 * time.Second))
	f.srv.Step(woke.Add(100 * time.Second))
	f.srv.Step(woke.Add(120 * time.Second))
	f.srv.Step(woke.Add(140 * time.Second))
	f.srv.Step(woke.Add(160 * time.Second))
	f.srv.Step(woke.Add(180 * time.Second))
	f.srv.Step(woke.Add(200 * time.Second))
	if !e.next(t).Lost {
		t.Error("8 hours of old credit must not stretch a new allowance")
	}
}

func TestRunExitsOnItsOwn(t *testing.T) {
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Store: store, Token: token, Page: fstest.MapFS{}, Tick: 5 * time.Millisecond, FirstTabGrace: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ExitReason, 1)
	go func() { done <- srv.Run() }()
	select {
	case got := <-done:
		if got != ExitNoTab {
			t.Errorf("exit = %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the server never exited")
	}
	conn, err := net.DialTimeout("tcp", srv.Addr().String(), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("the port is still open")
	}
}
