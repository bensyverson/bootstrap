package screen

import (
	"errors"
	"slices"
	"testing"

	"github.com/bensyverson/bootstrap/catalog"
)

func TestProgressNeverGoesDown(t *testing.T) {
	required := []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill"}
	sets := map[string][]string{
		"before anyone chooses": nil,
		"all kept":              catalog.IDs(),
		"all skippable skipped": required,
		"one kept":              append(slices.Clone(required), "sleepy"),
		"two kept":              append(slices.Clone(required), "sleepy", "woodcase"),
	}
	for name, chosen := range sets {
		t.Run(name, func(t *testing.T) {
			last := -1.0
			for _, req := range PathOrder(chosen) {
				got := Progress(req, chosen, 0)
				if got < last {
					t.Errorf("%+v: progress %v went below %v", req, got, last)
				}
				if got < 0 || got > 1 {
					t.Errorf("%+v: progress %v out of range", req, got)
				}
				last = got
			}
			if last != 1 {
				t.Errorf("the path ends at %v, want 1", last)
			}
		})
	}
}

func TestProgressRisesAcrossTools(t *testing.T) {
	first := Progress(Request{Screen: "tool", Tool: "agents"}, nil, 0)
	second := Progress(Request{Screen: "tool", Tool: "jobs"}, nil, 0)
	if !(second > first) {
		t.Errorf("agents %v, jobs %v: progress must rise from tool to tool", first, second)
	}
}

func TestOffPathScreensHoldProgress(t *testing.T) {
	for _, req := range []Request{{Screen: "trouble", Tool: "jobs", Reason: "network"}, {Screen: "fixing"}, {Screen: "chat"}} {
		if got := Progress(req, nil, 0.42); got != 0.42 {
			t.Errorf("%s progress = %v, want the carried 0.42", req.Screen, got)
		}
	}
}

func TestPathOrderSkipsToolsNotChosen(t *testing.T) {
	required := []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill"}
	for _, req := range PathOrder(required) {
		if req.Tool == "sleepy" {
			t.Errorf("sleepy was skipped: %+v", req)
		}
	}
}

func TestCheckChosenRefusesASkippedTool(t *testing.T) {
	required := []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill"}
	err := CheckChosen(Request{Screen: "tool", Tool: "sleepy"}, required)
	var inv *InvalidError
	if !errors.As(err, &inv) || slices.Contains(inv.Valid, "sleepy") || !slices.Contains(inv.Valid, "jobs") {
		t.Errorf("err = %v", err)
	}
	if err := CheckChosen(Request{Screen: "tool", Tool: "jobs"}, required); err != nil {
		t.Errorf("jobs is chosen: %v", err)
	}
	if err := CheckChosen(Request{Screen: "tool", Tool: "sleepy"}, nil); err != nil {
		t.Errorf("nothing has been chosen yet: %v", err)
	}
}
