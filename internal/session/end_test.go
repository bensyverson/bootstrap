package session

import (
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

func plainText(spans []screen.Span) string {
	var out string
	for _, s := range spans {
		out += s.Text
	}
	return out
}

func TestStoppingOnAnUnsettledScreenShowsTheEnd(t *testing.T) {
	for _, tc := range []struct {
		agent screen.Agent
		line  string
	}{
		{screen.ClaudeCode, "You can close this and go back to Claude Code."},
		{screen.OpenCode, "You can close this and go back to OpenCode."},
	} {
		store := Store{Dir: t.TempDir()}
		if _, err := store.Begin(tc.agent, 1, t0); err != nil {
			t.Fatal(err)
		}
		state := show(t, store, screen.Request{Screen: "list"}, t0)
		if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "lets-go", Tools: screen.AllToolIDs()}, t0); err != nil {
			t.Fatal(err)
		}
		state, err := store.Stop(t0)
		if err != nil {
			t.Fatal(err)
		}
		frame := state.Frame(t0.Add(time.Hour))
		if !frame.Ended || frame.Lost || frame.Screen == nil || frame.Screen.Screen != screen.End {
			t.Fatalf("%s: frame = %+v", tc.agent.ID, frame)
		}
		if got := plainText(frame.Screen.Title); got != "That’s all for now." {
			t.Errorf("%s: title = %q", tc.agent.ID, got)
		}
		if got := plainText(frame.Screen.Status); got != tc.line {
			t.Errorf("%s: line = %q", tc.agent.ID, got)
		}
		// An answered frame keeps the object busy, and nothing is left to wait for.
		if frame.Screen.Mode != screen.Settled || frame.Answered {
			t.Errorf("%s: the end settles the object: mode = %s, answered = %v", tc.agent.ID, frame.Screen.Mode, frame.Answered)
		}
		if frame.Gone != nil {
			t.Errorf("%s: an ended session has no lost screen to fall back on", tc.agent.ID)
		}
	}
}

func TestStoppingBeforeAnyScreenShowsTheEnd(t *testing.T) {
	store := begun(t)
	state, err := store.Stop(t0)
	if err != nil {
		t.Fatal(err)
	}
	if frame := state.Frame(t0); frame.Screen == nil || frame.Screen.Screen != screen.End {
		t.Errorf("frame = %+v", frame)
	}
}

func TestStoppingOnDoneKeepsDone(t *testing.T) {
	store := begun(t)
	show(t, store, screen.Request{Screen: "done"}, t0)
	state, err := store.Stop(t0)
	if err != nil {
		t.Fatal(err)
	}
	if frame := state.Frame(t0); !frame.Ended || frame.Screen == nil || frame.Screen.Screen != "done" {
		t.Errorf("frame = %+v", frame)
	}
}

func TestTheEndHoldsProgress(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	want := state.Progress
	state, err := store.Stop(t0)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Frame(t0).Screen; got.Screen != screen.End || got.Progress != want || want == 0 {
		t.Errorf("end = %s at %v, want end at %v", got.Screen, got.Progress, want)
	}
}
