package flow_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/session"
)

// brewUpgrades lists the formulae the flow ran brew upgrade on, in order.
func (w *world) brewUpgrades() []string {
	var out []string
	for _, c := range w.mac.Calls() {
		if c.Path == "/opt/homebrew/bin/brew" && len(c.Args) == 2 && c.Args[0] == "upgrade" {
			if c.Kind != flow.Change {
				w.t.Errorf("%s isn't marked a change", c)
			}
			out = append(out, c.Args[1])
		}
	}
	return out
}

// Ben, 2026-11-06: a tool already installed may be too old for the setup (job
// 0.1.0 has no hook install), so the tools step brings Homebrew's copies up to date.
func TestToolsAlreadyThereFromHomebrewAreUpgraded(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.mac.On("brew upgrade bensyverson/tap/job", flow.Result{Output: []byte("==> Upgrading job\n🍺  /opt/homebrew/Cellar/job/0.2.0: 6 files, 16MB")}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	got := w.brewUpgrades()
	for _, want := range []string{"bensyverson/tap/agents", "bensyverson/tap/job", "bensyverson/tap/contextfill", "bensyverson/tap/sleepy"} {
		if !slices.Contains(got, want) {
			t.Errorf("no brew upgrade %s; upgraded %v", want, got)
		}
	}
	if !strings.Contains(notes(w.state()), "updated jobs 0.2.0") {
		t.Errorf("notes:\n%s", notes(w.state()))
	}
}

func TestAToolOutsideHomebrewIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	for _, path := range []string{"/Library/Developer/CommandLineTools/usr/bin/git", "/opt/homebrew/bin/brew",
		"/opt/homebrew/bin/agents", "/opt/homebrew/bin/job", "/opt/homebrew/bin/contextfill"} {
		if err := w.mac.Place(path); err != nil {
			t.Fatal(err)
		}
	}
	// sleepy built from source, in ~/go/bin.
	own := filepath.Join(w.home, "go", "bin", "sleepy")
	if err := os.MkdirAll(filepath.Dir(own), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	got := w.brewUpgrades()
	if slices.Contains(got, "bensyverson/tap/sleepy") {
		t.Errorf("sleepy, built from source, was upgraded with brew: %v", got)
	}
	if !slices.Contains(got, "bensyverson/tap/job") {
		t.Errorf("Homebrew's job wasn't upgraded: %v", got)
	}
}

func TestAFailedUpgradeHandsOff(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.mac.On("brew upgrade bensyverson/tap/sleepy", flow.Result{Exit: 1, Output: []byte("Error: sleepy: no bottle available")}, 1)
	var handed bool
	w.resume = func(h session.Handoff) session.Way {
		if h.Tool == "sleepy" {
			handed = true
		}
		return session.WayRetry
	}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !handed {
		t.Error("a failed upgrade didn't hand off")
	}
}
