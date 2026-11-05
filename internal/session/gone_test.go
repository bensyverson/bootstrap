package session

import (
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// When the server dies there's nobody left to ask, so the words have to be in the page already.
func TestEveryFrameCarriesTheLostScreen(t *testing.T) {
	store := begun(t)
	if frame := load(t, store).Frame(t0); frame.Gone == nil || frame.Gone.Screen != screen.Lost {
		t.Errorf("before any screen: gone = %+v", frame.Gone)
	}
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	frame := state.Frame(t0)
	if frame.Gone == nil || frame.Gone.Screen != screen.Lost {
		t.Fatalf("while working: gone = %+v", frame.Gone)
	}
	if frame.Lost {
		t.Error("carrying the lost screen isn't being lost")
	}
	if frame.Screen.Screen != "tool" {
		t.Errorf("screen = %s", frame.Screen.Screen)
	}
}

func TestTheLostScreenUsesTheAgentsNameAndKeepsProgress(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	gone := state.Frame(t0).Gone
	if got := gone.Title[0].Text; !strings.Contains(got, "Claude Code") {
		t.Errorf("title = %q", got)
	}
	if want := state.Frame(t0).Screen.Progress; gone.Progress != want {
		t.Errorf("progress = %v, want %v", gone.Progress, want)
	}
}

func TestAStoppedSessionCarriesNoLostScreen(t *testing.T) {
	store := begun(t)
	show(t, store, screen.Request{Screen: "welcome"}, t0)
	state, err := store.Stop(t0)
	if err != nil {
		t.Fatal(err)
	}
	if frame := state.Frame(t0.Add(time.Hour)); frame.Gone != nil {
		t.Errorf("gone = %+v", frame.Gone)
	}
}
