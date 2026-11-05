package flow_test

import (
	"github.com/bensyverson/bootstrap/internal/flow"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

func TestStepsRunInTheApprovedOrder(t *testing.T) {
	var got []session.StepID
	for _, s := range flow.Steps() {
		got = append(got, s.ID)
	}
	want := []session.StepID{flow.StepPlan, flow.StepApple, flow.StepHomebrew, flow.StepTools, flow.StepFolder, flow.StepGit, flow.StepGitHub, flow.StepIdentity, flow.StepStage, flow.StepRules, flow.StepSettings, flow.StepPhone, flow.StepDone}
	if !slices.Equal(got, want) {
		t.Errorf("steps = %v, want %v", got, want)
	}
	for _, s := range flow.Steps() {
		for _, id := range slices.Concat(s.Screens, s.Asks) {
			valid := slices.Contains(screen.ScreenIDs(), string(id))
			if !valid {
				t.Errorf("%s shows %s, which isn't a screen", s.ID, id)
			}
		}
		for _, id := range s.Asks {
			if !screen.AwaitsClick(id) {
				t.Errorf("%s asks on %s, which has nothing to click", s.ID, id)
			}
		}
	}
}

func TestWalkFromNothingToDone(t *testing.T) {
	w := newWorld(t)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/Library/Developer/CommandLineTools/usr/bin/git", "/opt/homebrew/bin/brew",
		"/opt/homebrew/bin/agents", "/opt/homebrew/bin/job", "/opt/homebrew/bin/contextfill", "/opt/homebrew/bin/sleepy",
		"/opt/homebrew/bin/gh",
	} {
		if _, err := os.Stat(filepath.Join(w.mac.Root, path)); err != nil {
			t.Errorf("%s wasn't installed", path)
		}
	}
	for _, skipped := range []string{"peep", "woodcase"} {
		if _, err := os.Stat(filepath.Join(w.mac.Root, "/opt/homebrew/bin", skipped)); err == nil {
			t.Errorf("%s was installed though the person left it off", skipped)
		}
	}
	project := filepath.Join(w.home, "Projects", "pine-bakery")
	for _, name := range []string{".git", ".gitignore", "AGENTS.md", "CLAUDE.md", "local", ".jobs.db"} {
		if _, err := os.Lstat(filepath.Join(project, name)); err != nil {
			t.Errorf("the project lacks %s", name)
		}
	}
	ignore, _ := os.ReadFile(filepath.Join(project, ".gitignore"))
	if got, want := string(ignore), ".DS_Store\nnode_modules/\n.env\ndist/\nlocal/\n.jobs.db\n"; got != want {
		t.Errorf(".gitignore = %q, want %q", got, want)
	}
	if hooks := w.mac.Hooks(); !slices.Equal(hooks, []string{"contextfill", "job"}) {
		t.Errorf("hooks = %v", hooks)
	}
	changes := strings.Join(w.mac.Changes(), "\n")
	for _, want := range []string{
		"xcode-select --install\nopen -a '/System/Library/CoreServices/Install Command Line Developer Tools.app'",
		"brew trust bensyverson/tap", "brew install bensyverson/tap/job", "job init --as claude",
		"git config --global init.defaultBranch main\ngit init",
		"gh auth login --hostname github.com --web --git-protocol https --skip-ssh-key\ngh auth setup-git",
		"git config --global user.name 'Ada Lovelace'", "git config --global user.email 1234+ada@users.noreply.github.com",
		"agents init github.com/bensyverson/agents-md",
	} {
		if !strings.Contains(changes, want) {
			t.Errorf("never ran %q:\n%s", want, changes)
		}
	}
	for _, never := range []string{"git commit", "git add", "shellenv"} {
		if strings.Contains(changes, never) {
			t.Errorf("ran %q:\n%s", never, changes)
		}
	}
	if globals := w.mac.Globals(); globals["user.name"] != "Ada Lovelace" || globals["user.email"] != "1234+ada@users.noreply.github.com" || globals["init.defaultBranch"] != "main" {
		t.Errorf("git's settings = %v", globals)
	}
	for _, c := range w.mac.Calls() {
		if !filepath.IsAbs(c.Path) {
			t.Errorf("%s is run by name, not by full path", c.Path)
		}
	}

	state := w.state()
	if state.Current == nil || state.Current.Request.Screen != "done" {
		t.Errorf("the page ends on %+v", state.Current)
	}
	if state.Step == nil || state.Step.ID != flow.StepDone || state.Step.Status != session.StepDone || state.Step.Of != len(flow.Steps()) {
		t.Errorf("last step = %+v", state.Step)
	}
	var kinds []session.HandoffKind
	for _, h := range state.Handoffs {
		kinds = append(kinds, h.Handoff.Kind)
	}
	if !slices.Equal(kinds, []session.HandoffKind{session.HandoffPhone, session.HandoffDone}) {
		t.Errorf("handoffs = %v", kinds)
	}
	told := notes(state)
	for _, want := range []string{"person chose see-plan", "installed Apple's tools", "installed Homebrew", "installed jobs", "made ~/Projects/pine-bakery"} {
		if !strings.Contains(told, want) {
			t.Errorf("the notes don't say %q:\n%s", want, told)
		}
	}
	if !state.Driving || state.Lost(state.Deadline.Add(1e12)) {
		t.Error("the flow drives, so the agent's absence loses nothing")
	}
}

func TestSecondWalkChangesNothing(t *testing.T) {
	w := newWorld(t)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, w.mac.Root, w.home)
	calls := len(w.mac.Calls())
	asks := len(w.asked())
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, w.mac.Root, w.home); !maps.Equal(before, after) {
		t.Error("the second walk changed files")
	}
	for _, c := range w.mac.Calls()[calls:] {
		if c.Kind != flow.Inspect {
			t.Errorf("the second walk ran %q", c.String())
		}
	}
	if again := w.asked()[asks:]; len(again) > 0 {
		t.Errorf("the second walk asked again: %v", again)
	}
	if got := w.state().Handoffs; len(got) != 2 {
		t.Errorf("the second walk handed off again: %+v", got)
	}
}

func TestAStartingFolderThatIsARepoIsSetUpInPlace(t *testing.T) {
	w := newWorld(t)
	start := filepath.Join(w.home, "code", "bakery")
	put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
	put(t, filepath.Join(start, "index.html"), "<h1>Pine</h1>\n")
	put(t, filepath.Join(start, "notes", "todo.txt"), "bread\n")
	w.probe.Cwd = start
	before := snapshot(t, start)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, start)
	for path, body := range before {
		if path == filepath.Join(start, ".git") || strings.HasPrefix(path, filepath.Join(start, ".git")+"/") {
			continue
		}
		if after[path] != body {
			t.Errorf("%s moved or changed", path)
		}
	}
	for _, name := range []string{"AGENTS.md", "local", ".jobs.db"} {
		if _, err := os.Lstat(filepath.Join(start, name)); err != nil {
			t.Errorf("the project wasn't set up in place: no %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("made ~/Projects for a folder that was already a project")
	}
	for _, a := range w.asked() {
		if a.Screen == "naming" || a.Screen == "git-ask" {
			t.Errorf("asked %v about a folder that's already a repo", a)
		}
	}
	for _, c := range w.mac.Calls() {
		if c.Dir != "" && c.Dir != start {
			t.Errorf("%q ran in %s", c.String(), c.Dir)
		}
	}
}

func TestTheHomeFolderIsNeverSetUpInPlace(t *testing.T) {
	w := newWorld(t)
	put(t, filepath.Join(w.home, ".git", "HEAD"), "ref: refs/heads/main\n")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects", "pine-bakery", "AGENTS.md")); err != nil {
		t.Error("a home folder that's a repo still gets its own project folder")
	}
	if _, err := os.Stat(filepath.Join(w.home, "AGENTS.md")); err == nil {
		t.Error("set up the home folder")
	}
}

func TestAnExistingProjectFolderIsUsedAsItIs(t *testing.T) {
	w := newWorld(t)
	project := filepath.Join(w.home, "Projects", "pine-bakery")
	put(t, filepath.Join(project, "recipe.txt"), "flour\n")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(project, "recipe.txt")); string(body) != "flour\n" {
		t.Error("the person's file changed")
	}
}

func TestSkipTheTourKeepsEveryTool(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"welcome", ""}] = screen.Answer{Choice: "skip-tour"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(w.asked(), asked{"list", ""}) {
		t.Error("skip the tour skips the list")
	}
	for _, f := range []string{"peep", "woodcase", "sleepy"} {
		if _, err := os.Stat(filepath.Join(w.mac.Root, "/opt/homebrew/bin", f)); err != nil {
			t.Errorf("%s wasn't installed", f)
		}
	}
}

func TestDecliningLeavesThingsAlone(t *testing.T) {
	w := newWorld(t)
	for _, key := range []asked{{"git-ask", ""}, {"github", ""}, {"settings", ""}, {"phone", ""}} {
		w.answers[key] = screen.Answer{Choice: "not-now"}
	}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	changes := strings.Join(w.mac.Changes(), "\n")
	for _, never := range []string{"git init", "git config", "gh auth login", "hook install"} {
		if strings.Contains(changes, never) {
			t.Errorf("ran %q after the person said not now", never)
		}
	}
	for _, a := range w.asked() {
		if strings.HasPrefix(string(a.Screen), "identity") {
			t.Errorf("asked %v with no repo to sign", a)
		}
	}
	for _, h := range w.state().Handoffs {
		if h.Handoff.Kind == session.HandoffPhone {
			t.Error("handed off the phone after not now")
		}
	}
}

func TestAnAppAsksAboutXcode(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"making", ""}] = screen.Answer{Choice: "app"}
	w.answers[asked{"xcode", ""}] = screen.Answer{Choice: "app-store"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(w.mac.Changes(), "\n"), "open macappstore://") {
		t.Errorf("didn't open the App Store: %v", w.mac.Changes())
	}
	ignore, _ := os.ReadFile(filepath.Join(w.home, "Projects", "pine-bakery", ".gitignore"))
	if got, want := string(ignore), ".DS_Store\n.build/\n.swiftpm/\nDerivedData/\nxcuserdata/\nlocal/\n.jobs.db\n"; got != want {
		t.Errorf(".gitignore = %q, want %q", got, want)
	}
}

func TestOtherWorkIgnoresOnlyWhatsKnown(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"making", ""}] = screen.Answer{Choice: "code"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	ignore, _ := os.ReadFile(filepath.Join(w.home, "Projects", "pine-bakery", ".gitignore"))
	if got, want := string(ignore), ".DS_Store\nlocal/\n.jobs.db\n"; got != want {
		t.Errorf(".gitignore = %q, want %q", got, want)
	}
}

func TestAnExistingGitignoreKeepsItsLines(t *testing.T) {
	w := newWorld(t)
	start := filepath.Join(w.home, "code", "bakery")
	put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
	put(t, filepath.Join(start, ".gitignore"), "secrets.txt\nnode_modules/")
	w.probe.Cwd = start
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	ignore, _ := os.ReadFile(filepath.Join(start, ".gitignore"))
	if got, want := string(ignore), "secrets.txt\nnode_modules/\n.DS_Store\n.env\ndist/\nlocal/\n.jobs.db\n"; got != want {
		t.Errorf(".gitignore = %q, want %q", got, want)
	}
}

func TestAProjectWithItsOwnClaudeFileIsMigrated(t *testing.T) {
	w := newWorld(t)
	start := filepath.Join(w.home, "code", "bakery")
	put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
	put(t, filepath.Join(start, "CLAUDE.md"), "# my rules\n")
	w.probe.Cwd = start
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	changes := strings.Join(w.mac.Changes(), "\n")
	if !strings.Contains(changes, "agents migrate") {
		t.Errorf("didn't migrate:\n%s", changes)
	}
	if body, _ := os.ReadFile(filepath.Join(start, "AGENTS.md")); string(body) != "# my rules\n" {
		t.Errorf("AGENTS.md = %q", body)
	}
}
