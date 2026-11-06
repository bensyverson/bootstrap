package flow_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// downloaded puts the copy SETUP.md downloads at ~/.bootstrap/bin/bootstrap
// and returns its path.
func (w *world) downloaded() string {
	path := filepath.Join(w.home, ".bootstrap", "bin", "bootstrap")
	put(w.t, path, "the pinned download")
	return path
}

func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Ben, 2026-11-06: once Homebrew is in, the tools step installs bootstrap from
// the tap, with no card of its own, and deletes the downloaded copy.
func TestTheToolsStepMovesBootstrapOntoHomebrew(t *testing.T) {
	w := newWorld(t)
	own := w.downloaded()
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !present(filepath.Join(w.mac.Root, "opt/homebrew/bin/bootstrap")) {
		t.Error("Homebrew's bootstrap wasn't installed")
	}
	if !strings.Contains(strings.Join(w.mac.Changes(), "\n"), "brew install bensyverson/tap/bootstrap") {
		t.Errorf("never ran brew install bensyverson/tap/bootstrap:\n%s", strings.Join(w.mac.Changes(), "\n"))
	}
	if present(own) {
		t.Error("the downloaded copy is still there")
	}
	if !strings.Contains(notes(w.state()), "installed bootstrap 0.1.0") {
		t.Errorf("notes:\n%s", notes(w.state()))
	}
}

// Before the formula is published, or when brew fails, the run goes on with
// the downloaded copy: nothing the person must see or the agent must fix.
func TestAFailedBootstrapInstallKeepsTheDownloadedCopy(t *testing.T) {
	w := newWorld(t)
	own := w.downloaded()
	w.mac.On("brew install bensyverson/tap/bootstrap", flow.Result{Exit: 1, Output: []byte("Error: No available formula with the name \"bensyverson/tap/bootstrap\".")}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !present(own) {
		t.Error("the downloaded copy was deleted though Homebrew's never came")
	}
	if !strings.Contains(notes(w.state()), "kept ~/.bootstrap/bin/bootstrap") {
		t.Errorf("notes:\n%s", notes(w.state()))
	}
	if slices.ContainsFunc(w.state().Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffTrouble }) {
		t.Error("a failed bootstrap install handed off as trouble")
	}
}

func TestHomebrewsBootstrapIsUpgradedAndTheDownloadDeleted(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := w.mac.Place("/opt/homebrew/bin/bootstrap"); err != nil {
		t.Fatal(err)
	}
	own := w.downloaded()
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(w.brewUpgrades(), "bensyverson/tap/bootstrap") {
		t.Errorf("Homebrew's bootstrap wasn't upgraded: %v", w.brewUpgrades())
	}
	if present(own) {
		t.Error("the downloaded copy is still there")
	}
}

func TestAFailedBootstrapUpgradeIsNotTrouble(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := w.mac.Place("/opt/homebrew/bin/bootstrap"); err != nil {
		t.Fatal(err)
	}
	w.mac.On("brew upgrade bensyverson/tap/bootstrap", flow.Result{Exit: 1, Output: []byte("Error: offline")}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(w.brewUpgrades(), "bensyverson/tap/bootstrap") {
		t.Errorf("Homebrew's bootstrap wasn't upgraded: %v", w.brewUpgrades())
	}
}

// With just the tools, the done handoff tells the person how to run bootstrap later.
func TestTheDoneHandoffNamesHomebrewsBootstrap(t *testing.T) {
	w := newWorld(t)
	w.downloaded()
	w.answers[asked{"naming", ""}] = screen.Answer{Choice: screen.JustTools}
	// With the helpers on, the handoff says to just ask instead (S2lZZq; teach_test.go).
	w.answers[asked{"settings", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	i := slices.IndexFunc(state.Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffDone })
	if i < 0 {
		t.Fatal("no done handoff")
	}
	// Ben, 2026-11-06: Homebrew puts its bin on every new Terminal's PATH
	// (/etc/paths.d/homebrew), so the person runs plain bootstrap new.
	what := state.Handoffs[i].Handoff.What
	if !strings.Contains(what, "bootstrap new <name> in a new Terminal window") || strings.Contains(what, "/opt/homebrew") {
		t.Errorf("the done handoff says %q", what)
	}
}

// Until Homebrew's copy is there, the download is off the PATH, so the
// person needs its full path.
func TestTheDoneHandoffNamesTheDownloadWhenItStayed(t *testing.T) {
	w := newWorld(t)
	w.downloaded()
	w.answers[asked{"naming", ""}] = screen.Answer{Choice: screen.JustTools}
	// With the helpers on, the handoff says to just ask instead (S2lZZq; teach_test.go).
	w.answers[asked{"settings", ""}] = screen.Answer{Choice: "not-now"}
	w.mac.On("brew install bensyverson/tap/bootstrap", flow.Result{Exit: 1}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	i := slices.IndexFunc(state.Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffDone })
	if i < 0 {
		t.Fatal("no done handoff")
	}
	if what := state.Handoffs[i].Handoff.What; !strings.Contains(what, "~/.bootstrap/bin/bootstrap new <name>") {
		t.Errorf("the done handoff says %q", what)
	}
}
