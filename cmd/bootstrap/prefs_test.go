package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/cli"
)

// Later setups remember the person's answers (XbdzWw), and someone may want
// only the tools (VGCn9C).

func (w *world) writePrefs(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(w.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.dir, "preferences.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// prefs is what preferences.json holds now.
func (w *world) prefs(t *testing.T) (p struct {
	ProjectsIn string            `json:"projects_in"`
	SkipTools  []string          `json:"skip_tools"`
	Answers    map[string]string `json:"answers"`
}) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(w.dir, "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPrefsListsWhatIsRemembered(t *testing.T) {
	w := newWorld(t)
	w.writePrefs(t, `{"projects_in": "~/git", "skip_tools": ["woodcase"], "answers": {"settings": "yes", "github": "not-now"}}`)
	w.run(t, "prefs").want(t, 0, "skip_tools woodcase\nprojects_in ~/git\ngithub not-now\nsettings yes\n"+
		"next: bootstrap prefs reset <name> forgets one, so the next setup asks again\n", "")
	r := w.run(t, "prefs", "--json")
	var got struct {
		File        string `json:"file"`
		Preferences []struct {
			Name, Value string
		} `json:"preferences"`
		Next string `json:"next"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || r.code != 0 {
		t.Fatalf("exit %d, %v\n%s", r.code, err, r.stdout)
	}
	if got.File != filepath.Join(w.dir, "preferences.json") || len(got.Preferences) != 4 || got.Preferences[2].Name != "github" || got.Preferences[2].Value != "not-now" || got.Next == "" {
		t.Errorf("prefs --json = %+v", got)
	}
}

func TestPrefsWithNothingRemembered(t *testing.T) {
	w := newWorld(t)
	w.run(t, "prefs").want(t, 0, "nothing remembered yet\nnext: bootstrap new <name> remembers your answers for the next setup\n", "")
	if _, err := os.Stat(w.dir); err == nil {
		t.Error("reading the preferences made bootstrap's folder")
	}
}

func TestPrefsResetForgetsOne(t *testing.T) {
	w := newWorld(t)
	w.writePrefs(t, `{"projects_in": "~/git", "answers": {"github": "not-now"}}`)
	w.run(t, "prefs", "reset", "github").want(t, 0, "forgot github\nnext: bootstrap prefs\n", "")
	p := w.prefs(t)
	if _, ok := p.Answers["github"]; ok || p.ProjectsIn != "~/git" {
		t.Errorf("after reset github, preferences = %+v", p)
	}
	w.run(t, "prefs", "reset", "github").want(t, 0, "github wasn't remembered, so there was nothing to forget\nnext: bootstrap prefs\n", "")
}

func TestPrefsResetRefusesWhatIsNotAName(t *testing.T) {
	w := newWorld(t)
	w.run(t, "prefs", "reset", "gihtub").wantErr(t, 2, `bootstrap remembers nothing called "gihtub"`, "valid: skip_tools, projects_in, git, github", "next: bootstrap prefs")
	w.run(t, "prefs", "reset").wantErr(t, 2, "reset needs the name of what to forget", "next: bootstrap prefs")
	w.run(t, "prefs", "forget").wantErr(t, 2, `unknown prefs verb "forget"`, "valid: reset", "next: bootstrap prefs --help")
}

func TestPrefsNamesABrokenFile(t *testing.T) {
	w := newWorld(t)
	w.writePrefs(t, "nope")
	w.run(t, "prefs").wantErr(t, 5, "preferences.json", "next: fix or delete")
}

// The trial's bug (project/2026-11-06-clean-mac-trial-findings.md, run 2, finding 4):
// the person said not now to GitHub on the page, and a later new --yes signed in anyway.
func TestARememberedDeclineHoldsInNewWithYes(t *testing.T) {
	w := macForNew(t)
	w.signOut(t)
	w.writePrefs(t, `{"answers": {"github": "not-now", "identity": "skip"}}`)
	r := w.run(t, "new", "pine-bakery", "--yes")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	for _, c := range w.ran(t) {
		if strings.HasPrefix(c, "gh auth login") || c == "brew install gh" {
			t.Errorf("ran %q, though GitHub was declined", c)
		}
	}
	if !strings.Contains(r.stdout, "note person chose not-now on github\n") || !strings.Contains(r.stdout, "step github skipped") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
}

func TestAFlagOverridesTheRememberedAnswerAndIsRemembered(t *testing.T) {
	w := macForNew(t)
	w.writePrefs(t, `{"answers": {"settings": "not-now"}}`)
	r := w.run(t, "new", "pine-bakery", "--yes", "--settings", "--no-xcode")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if !slices.Contains(w.ran(t), "contextfill hook install --harness claude-code") {
		t.Errorf("--settings took back the remembered no, but the hooks didn't install:\n%s", strings.Join(w.ran(t), "\n"))
	}
	if p := w.prefs(t); p.Answers["settings"] != "yes" || p.Answers["xcode"] != "later" {
		t.Errorf("preferences = %+v, want the flags' answers", p)
	}
}

func TestNewWithYesNeverStartsGitHubsSignIn(t *testing.T) {
	w := macForNew(t)
	w.signOut(t)
	r := w.run(t, "new", "pine-bakery", "--yes", "--no-identity")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	for _, c := range w.ran(t) {
		if strings.HasPrefix(c, "gh auth login") {
			t.Errorf("ran %q, which waits on a code no one will type", c)
		}
	}
	if !strings.Contains(r.stdout, "step github skipped") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "next: GitHub's sign-in waits for a code typed in at github.com") || !strings.Contains(r.stdout, "bootstrap new pine-bakery at a terminal, without --yes") {
		t.Errorf("the next line doesn't say GitHub was skipped, or how to sign in:\n%s", r.stdout)
	}
}

func TestNewWithYesStopsBeforeApplesWindow(t *testing.T) {
	w := newWorld(t)
	r := w.run(t, "new", "pine-bakery", "--yes")
	r.wantFailed(t, 4, "Apple's tools need a click on Install in Apple's window", "bootstrap new pine-bakery at a terminal, without --yes")
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("new made ~/Projects before stopping")
	}
}

func TestNewJustTheTools(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "--just-tools", "--yes")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("just the tools made ~/Projects")
	}
	for _, c := range w.ran(t) {
		if c == "git init" || strings.HasPrefix(c, "agents ") || strings.HasPrefix(c, "job init") {
			t.Errorf("ran %q, a project step", c)
		}
	}
	for _, want := range []string{"git config --global user.name Ada Lovelace", "contextfill hook install --harness claude-code"} {
		if !slices.Contains(w.ran(t), want) {
			t.Errorf("never ran %q, a machine-wide step", want)
		}
	}
	if !strings.HasSuffix(r.stdout, "next: to make a project later: bootstrap new <name>\n") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
	w.noPage(t)
}

func TestNewJustTheToolsRefusesANameOrAProject(t *testing.T) {
	w := macForNew(t)
	w.run(t, "new", "pine-bakery", "--just-tools", "--yes").wantErr(t, 2, "--just-tools", "next: bootstrap new --help")
	if err := os.MkdirAll(filepath.Join(w.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.run(t, "new", "--just-tools", "--yes").wantErr(t, 2, "this folder is already a project")
}

func TestNewAtATerminalCanTakeJustTheTools(t *testing.T) {
	w := macForNew(t)
	var out bytes.Buffer
	code := cli.Run([]string{"new"}, w.terminal("\n\n\n\n", &out))
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "Project name (Enter for just the tools for now): ") {
		t.Errorf("the name's prompt doesn't offer just the tools:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("just the tools made ~/Projects")
	}
}
