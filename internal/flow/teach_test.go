package flow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
)

// With the helpers on, bootstrap tells every later session how to start a
// project, in the agent's instructions file for the whole Mac (Ben, 2026-11-06, S2lZZq).

func (w *world) rules(agent screen.Agent) string {
	w.t.Helper()
	body, err := os.ReadFile(filepath.Join(w.home, agent.RulesFile()))
	if err != nil && !os.IsNotExist(err) {
		w.t.Fatal(err)
	}
	return string(body)
}

func TestTheHelpersTeachClaudeCodeToBootstrap(t *testing.T) {
	w := newWorld(t)
	had := "# Mine\n\n- Write American English.\n"
	put(t, filepath.Join(w.home, ".claude", "CLAUDE.md"), had)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	got := w.rules(screen.ClaudeCode)
	if !strings.HasPrefix(got, had) {
		t.Errorf("the lines that were there aren't kept:\n%s", got)
	}
	for _, want := range []string{"<!-- bootstrap:begin -->", "bootstrap new <name> --yes", "`next:` lines", "bootstrap new --help", "<!-- bootstrap:end -->"} {
		if !strings.Contains(got, want) {
			t.Errorf("the instructions lack %q:\n%s", want, got)
		}
	}
}

func TestTheHelpersTeachOpenCodeToBootstrap(t *testing.T) {
	w := newWorldFor(t, screen.OpenCode)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.rules(screen.OpenCode); !strings.Contains(got, "bootstrap new <name> --yes") {
		t.Errorf("OpenCode's instructions are:\n%s", got)
	}
}

func TestNoHelpersTeachNothing(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"settings", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.rules(screen.ClaudeCode); got != "" {
		t.Errorf("with the helpers off, ~/.claude/CLAUDE.md is:\n%s", got)
	}
}

func TestJustTheToolsWithTheHelpersSaysJustAsk(t *testing.T) {
	for _, tc := range []struct {
		settings, status string
	}{
		{"yes", screen.DoneJustToolsAsk},
		{"not-now", screen.DoneJustTools},
	} {
		t.Run(tc.settings, func(t *testing.T) {
			w := newWorld(t)
			w.answers[asked{"naming", ""}] = screen.Answer{Choice: screen.JustTools}
			w.answers[asked{"settings", ""}] = screen.Answer{Choice: tc.settings}
			if err := w.walk(); err != nil {
				t.Fatal(err)
			}
			state := w.state()
			if state.Current == nil || state.Current.Request != (screen.Request{Screen: "done", Status: tc.status}) {
				t.Errorf("ended on %+v, want done with status %q", state.Current, tc.status)
			}
		})
	}
}

// The block names bootstrap the way a new Terminal finds it: plain once
// Homebrew's copy is there, the download's full path while it stayed.
func TestTheInstructionsNameBootstrapWhereItIs(t *testing.T) {
	w := newWorld(t)
	w.downloaded()
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.rules(screen.ClaudeCode); !strings.Contains(got, "`bootstrap new <name> --yes`") {
		t.Errorf("with Homebrew's bootstrap, the instructions are:\n%s", got)
	}
}

func TestTheInstructionsNameTheDownloadWhenItStayed(t *testing.T) {
	w := newWorld(t)
	w.downloaded()
	w.mac.On("brew install bensyverson/tap/bootstrap", flow.Result{Exit: 1}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.rules(screen.ClaudeCode); !strings.Contains(got, "`~/.bootstrap/bin/bootstrap new <name> --yes`") {
		t.Errorf("with the download kept, the instructions are:\n%s", got)
	}
}
