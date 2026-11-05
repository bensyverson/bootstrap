package session

import (
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

func TestLostWhileWorking(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	if state.Lost(t0.Add(DefaultAllowance)) {
		t.Error("not lost at the allowance")
	}
	if !state.Lost(t0.Add(DefaultAllowance + time.Second)) {
		t.Error("lost past the allowance")
	}
}

func TestExpectStretchesTheAllowance(t *testing.T) {
	store := begun(t)
	state, err := store.Show(screen.Request{Screen: "tool", Tool: "jobs"}, 20*time.Minute, t0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Lost(t0.Add(19 * time.Minute)) {
		t.Error("the agent said 20 minutes")
	}
	if !state.Lost(t0.Add(21 * time.Minute)) {
		t.Error("lost past what it said")
	}
}

func TestACheckDoesNotShortenAnExpectation(t *testing.T) {
	store := begun(t)
	if _, err := store.Show(screen.Request{Screen: "tool", Tool: "jobs"}, 20*time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	if err := store.Renew(t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if load(t, store).Lost(t0.Add(10 * time.Minute)) {
		t.Error("a status check cut a 20 minute expectation down to 3")
	}
	if !load(t, store).Lost(t0.Add(21 * time.Minute)) {
		t.Error("the expectation still ends")
	}
}

func TestANewScreenReplacesTheExpectation(t *testing.T) {
	store := begun(t)
	if _, err := store.Show(screen.Request{Screen: "tool", Tool: "jobs"}, 20*time.Minute, t0); err != nil {
		t.Fatal(err)
	}
	state := show(t, store, screen.Request{Screen: "tool", Tool: "sleepy"}, t0.Add(time.Minute))
	if !state.Lost(t0.Add(5 * time.Minute)) {
		t.Error("the 20 minutes were about the last screen")
	}
}

func TestRenewBringsThePageBack(t *testing.T) {
	store := begun(t)
	show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	later := t0.Add(10 * time.Minute)
	if !load(t, store).Lost(later) {
		t.Fatal("should be lost")
	}
	if err := store.Renew(later); err != nil {
		t.Fatal(err)
	}
	state := load(t, store)
	if state.Lost(later) {
		t.Error("the next command brings the page back")
	}
	if frame := state.Frame(later); frame.Lost || frame.Screen.Screen != "tool" {
		t.Errorf("frame = %+v", frame)
	}
}

func TestLostWhileWaitingOnlyAfterAClick(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	later := t0.Add(2 * time.Hour)
	if state.Lost(later) {
		t.Error("a person can read as long as they like")
	}
	state, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, later)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Lost(later) {
		t.Error("the click found no agent")
	}
	if frame := state.Frame(later); !frame.Lost || frame.Screen.Screen != screen.Lost {
		t.Errorf("frame = %+v", frame)
	}
}

func TestClickWithALiveAgentIsNotLost(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	state, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if state.Lost(t0.Add(time.Minute)) {
		t.Error("the agent checked recently")
	}
	if frame := state.Frame(t0.Add(time.Minute)); !frame.Answered {
		t.Errorf("frame = %+v", frame)
	}
}

func TestYourTurnScreensCanBeLost(t *testing.T) {
	for _, id := range []screen.ID{"apple-ask", "terminal"} {
		store := begun(t)
		state := show(t, store, screen.Request{Screen: id}, t0)
		if !state.Lost(t0.Add(DefaultAllowance + time.Second)) {
			t.Errorf("%s: no agent is running wait", id)
		}
	}
}

func TestDoneIsNeverLost(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "done"}, t0)
	if state.Current == nil || state.Current.Request.Screen != "done" || state.Progress != 1 {
		t.Fatalf("state = %+v", state)
	}
	if state.Lost(t0.Add(24 * time.Hour)) {
		t.Error("nothing is left to happen")
	}
}

func TestLostHoldsProgress(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	frame := state.Frame(t0.Add(time.Hour))
	if frame.Screen.Progress != state.Progress || frame.Screen.Progress == 0 {
		t.Errorf("lost progress = %v, state = %v", frame.Screen.Progress, state.Progress)
	}
}

func TestFrameNamesTheAgent(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if _, err := store.Begin(screen.OpenCode, 1, t0); err != nil {
		t.Fatal(err)
	}
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	if got := state.Frame(t0).Screen.Title[0].Text; !strings.Contains(got, "OpenCode") {
		t.Errorf("title = %q", got)
	}
}

func TestFrameBeforeAnyScreenHasNoScreen(t *testing.T) {
	store := begun(t)
	state := load(t, store)
	if state.ID == "" || state.Agent != screen.ClaudeCode {
		t.Fatalf("state = %+v", state)
	}
	if frame := state.Frame(t0); frame.Screen != nil || frame.Lost {
		t.Errorf("frame = %+v", frame)
	}
}

func TestStoppedFrameSaysEnded(t *testing.T) {
	store := begun(t)
	show(t, store, screen.Request{Screen: "welcome"}, t0)
	state, err := store.Stop(t0)
	if err != nil {
		t.Fatal(err)
	}
	if frame := state.Frame(t0.Add(time.Hour)); !frame.Ended || frame.Lost {
		t.Errorf("frame = %+v", frame)
	}
}

func TestPastLifetime(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	deadline := t0.Add(DefaultAllowance)
	if state.PastLifetime(deadline.Add(LostLifetime)) {
		t.Error("not yet")
	}
	if !state.PastLifetime(deadline.Add(LostLifetime + time.Second)) {
		t.Error("the hour is up")
	}
}
