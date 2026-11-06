package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// The rules bootstrap new keeps when no one is there to ask, when a page setup is
// running, and for answers kept from an earlier run (OWv6U2), and where new
// projects go (gpvHd3).

// untouched checks that a refused new ran nothing and made nothing.
func (w *world) untouched(t *testing.T) {
	t.Helper()
	if ran := w.ran(t); len(ran) != 0 {
		t.Errorf("a refused new ran:\n%s", strings.Join(ran, "\n"))
	}
	for _, path := range []string{filepath.Join(w.home, "Projects"), filepath.Join(w.dir, "new")} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("a refused new made %s", path)
		}
	}
}

func TestNewWithNoTerminalAndNoYesRefuses(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "pine-bakery")
	r.wantErr(t, 2,
		"error: no one is at a terminal to answer 6 questions",
		"question git-ask “Should Git keep track of this project?” answer=--git|--no-git\n",
		"question github “Want a place to keep your work online?” answer=--github|--no-github\n",
		"question identity answer=\"--identity|--no-identity|--git-name <name> --git-email <email>\"\n",
		"question making “What are you working on?” answer=\"--making <website|app|code|other>\"\n",
		"question xcode “This one needs Xcode.” answer=--xcode|--no-xcode\n",
		"question settings “Turn on Claude’s helpers?” answer=--settings|--no-settings\n",
		"next: bootstrap new pine-bakery --yes takes the defaults; or answer each with its flag, or run new at a terminal")
	w.untouched(t)
	w.noPage(t)
}

func TestNewWithJSONAndNoYesRefuses(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "pine-bakery", "--json", "--skip", "sleepy", "--no-git")
	if r.code != 2 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	var body struct {
		Exit      int
		Questions []struct{ Screen, Title, Answer string }
	}
	if err := json.Unmarshal([]byte(r.stdout), &body); err != nil {
		t.Fatal(err)
	}
	var screens []string
	for _, q := range body.Questions {
		screens = append(screens, q.Screen)
	}
	// With no Git, nothing asks who signs the work.
	if want := []string{"github", "making", "xcode", "settings"}; !slices.Equal(screens, want) {
		t.Errorf("questions = %v, want %v", screens, want)
	}
	w.untouched(t)
}

func TestNewWithEveryAnswerGivenNeedsNoTerminal(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "pine-bakery", "--skip", "peep", "--git", "--github", "--identity", "--making", "code", "--settings")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
}

func TestNewAgainNeedsNoTerminalOrYes(t *testing.T) {
	w := macForNew(t)
	if r := w.run(t, "new", "pine-bakery", "--yes"); r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if r := w.run(t, "new", "pine-bakery"); r.code != 0 {
		t.Errorf("answers kept for the project should answer everything: exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
}

func TestNewAYesFlagReversesAKeptNo(t *testing.T) {
	w := macForNew(t)
	if r := w.run(t, "new", "pine-bakery", "--yes", "--no-git", "--no-settings"); r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(w.project(), ".git")); err == nil {
		t.Fatal("--no-git turned on Git")
	}
	// With Git on, who signs the work is open again, so it needs its flag too.
	r := w.run(t, "new", "pine-bakery", "--git", "--settings", "--identity")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(w.project(), ".git")); err != nil {
		t.Error("--git didn't turn on Git after an earlier --no-git")
	}
	if !slices.Contains(w.ran(t), "job hook install --harness claude-code") {
		t.Error("--settings didn't turn on the helpers after an earlier --no-settings")
	}
}

func TestNewRefusesAYesAndANoTogether(t *testing.T) {
	w := macForNew(t)
	for _, offer := range []string{"git", "github", "identity", "xcode", "settings"} {
		w.run(t, "new", "pine-bakery", "--yes", "--"+offer, "--no-"+offer).wantErr(t, 2,
			"--"+offer+" and --no-"+offer+" say opposite things", "next: bootstrap new --help")
	}
	w.untouched(t)
}

func TestNewRefusesWhileAPageSetupRuns(t *testing.T) {
	w := macForNew(t)
	if r := w.run(t, "run", "--agent", "claude-code"); r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	t.Cleanup(func() {
		if info, err := w.store().ReadServerInfo(); err == nil {
			syscall.Kill(info.PID, syscall.SIGKILL)
		}
	})
	before := len(w.ran(t))
	w.run(t, "new", "pine-bakery", "--yes").wantErr(t, 3,
		"a setup is running on the page on this Mac",
		"next: bootstrap wait until it's done, or bootstrap stop to end it; then bootstrap new pine-bakery")
	if len(w.ran(t)) != before {
		t.Error("a refused new ran something")
	}
	if _, err := os.Stat(filepath.Join(w.dir, "new")); err == nil {
		t.Error("a refused new made its log")
	}
}

func TestNewInAFolder(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "pine-bakery", "--yes", "--in", "~/git")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	project := filepath.Join(w.home, "git", "pine-bakery")
	if _, err := os.Stat(filepath.Join(project, "AGENTS.md")); err != nil {
		t.Error("new --in ~/git didn't make ~/git/pine-bakery")
	}
	if !strings.Contains(r.stdout, "project path=~/git/pine-bakery ") || !strings.Contains(r.stdout, "cd ~/git/pine-bakery,") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("made ~/Projects for a project in ~/git")
	}
	// The next new needs no flag.
	if r := w.run(t, "new", "olive-cafe", "--yes"); r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(w.home, "git", "olive-cafe")); err != nil {
		t.Error("the next new didn't use the remembered ~/git")
	}
	body, err := os.ReadFile(filepath.Join(w.dir, "preferences.json"))
	if err != nil || !strings.Contains(string(body), `"projects_in": "~/git"`) {
		t.Errorf("preferences.json = %s, %v", body, err)
	}
}

func TestNewInRefusesABadFolder(t *testing.T) {
	w := macForNew(t)
	w.run(t, "new", "pine-bakery", "--yes", "--in", "~/My Projects").wantErr(t, 2, `--in: "~/My Projects" can't hold projects`, "next: bootstrap new --help")
	if err := os.MkdirAll(filepath.Join(w.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.run(t, "new", "--yes", "--in", "~/git").wantErr(t, 2, "this folder is already a project, so it's set up where it is")
}

func TestThePrimerStartsWithPeople(t *testing.T) {
	r := newWorld(t).run(t)
	people := strings.Index(r.stdout, "To set up a project: bootstrap new <name>")
	agents := strings.Index(r.stdout, "Run first: bootstrap status")
	if people < 0 || agents < 0 || people > agents {
		t.Errorf("the primer should tell people how to set up a project before the agent's first command:\n%s", r.stdout)
	}
}

func TestNewHelpSaysItRefusesWithNoTerminal(t *testing.T) {
	r := newWorld(t).run(t, "new", "--help")
	for _, want := range []string{"--in <folder>", "--git ", "--no-settings", "refuses", "a page setup"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("new --help lacks %q:\n%s", want, r.stdout)
		}
	}
}

// holdSetup stands in for a bootstrap new that is setting up a project on this Mac.
func (w *world) holdSetup(t *testing.T) {
	t.Helper()
	release, err := w.store().LockSetup()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

func TestNewRefusesWhileAnotherNewSetsUp(t *testing.T) {
	w := macForNew(t)
	w.holdSetup(t)
	w.run(t, "new", "pine-bakery", "--yes").wantErr(t, 3,
		"another bootstrap new is setting up a project on this Mac, and two setups can't install at once",
		"next: wait for it to finish, then bootstrap new pine-bakery")
	if len(w.ran(t)) != 0 {
		t.Error("a refused new ran something")
	}
}

func TestRunRefusesWhileANewSetsUp(t *testing.T) {
	w := macForNew(t)
	w.holdSetup(t)
	t.Cleanup(func() {
		if info, err := w.store().ReadServerInfo(); err == nil {
			syscall.Kill(info.PID, syscall.SIGKILL)
		}
	})
	w.run(t, "run", "--agent", "claude-code").wantErr(t, 3,
		"a bootstrap new is setting up a project on this Mac, and two setups can't install at once",
		"next: wait for it to finish, then bootstrap run --agent claude-code")
	if w.store().ServerRunning() {
		t.Error("a refused run left its page running")
	}
}

// Ben, 2026-11-06: nothing used the live-yet answer, so the question is gone.
func TestNewHasNoLiveQuestion(t *testing.T) {
	w := macForNew(t)
	w.run(t, "new", "pine-bakery", "--yes", "--live").wantErr(t, 2, "unknown flag --live")
	w.untouched(t)
}
