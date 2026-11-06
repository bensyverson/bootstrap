package flow_test

import (
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// OpenCode has no way to carry a session to a phone (Ben, 2026-11-06, PjAKYq),
// so the phone step asks nothing, hands nothing off and prints no tip there.

func TestOpenCodeIsNeverAskedAboutThePhone(t *testing.T) {
	w := newWorldFor(t, screen.OpenCode)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	for _, a := range w.asked() {
		if a.Screen == "phone" {
			t.Errorf("OpenCode was asked about the phone: %v", w.asked())
		}
	}
	state := w.state()
	for _, l := range state.Handoffs {
		if l.Handoff.Kind == session.HandoffPhone {
			t.Errorf("OpenCode handed off for the phone: %+v", l.Handoff)
		}
	}
	if got := state.Finished[flow.StepPhone]; got != session.StepDone {
		t.Errorf("phone = %q, want done", got)
	}
}

func TestOpenCodeAtATerminalGetsNoPhoneTip(t *testing.T) {
	w := newWorldFor(t, screen.OpenCode)
	if err := w.runAt(&person{}, flow.Given{Name: "Pine Bakery"}); err != nil {
		t.Fatal(err)
	}
	if n := notes(w.state()); strings.Contains(n, "phone") {
		t.Errorf("OpenCode got a phone tip:\n%s", n)
	}
}
