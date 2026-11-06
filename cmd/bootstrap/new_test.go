package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/cli"
)

// The pretend programs bootstrap new runs. Each writes its command line to the
// calls file, so a test can see every command the setup ran.
const (
	gitScript = `#!/bin/sh
echo "git $*" >> %[1]q
config=%[2]q
if [ "$1 $2 $3" = "config --global --get" ]; then
  value=$(grep "^$4=" "$config" 2>/dev/null | tail -1 | cut -d= -f2-)
  [ -n "$value" ] || exit 1
  echo "$value"
  exit 0
fi
if [ "$1 $2" = "config --global" ]; then
  echo "$3=$4" >> "$config"
  exit 0
fi
if [ "$1" = "init" ]; then
  mkdir -p .git
  exit 0
fi
echo "unexpected: git $*"
exit 1
`
	ghScript = `#!/bin/sh
echo "gh $*" >> %[1]q
case "$1 $2" in
"auth status")
  [ -e %[2]q ] && { echo "You are not logged into any GitHub hosts."; exit 1; }
  echo "✓ Logged in to github.com account ada"; exit 0 ;;
"api user") echo '{"login":"ada","id":1234,"name":"Ada Lovelace"}'; exit 0 ;;
esac
echo "unexpected: gh $*"
exit 1
`
	agentsScript = `#!/bin/sh
echo "agents $*" >> %[1]q
case "$1" in
init) printf '# rules\n' > AGENTS.md; exit 0 ;;
add) exit 0 ;;
esac
echo "unexpected: agents $*"
exit 1
`
	jobScript = `#!/bin/sh
echo "job $*" >> %[1]q
case "$1" in
init) touch .jobs.db; exit 0 ;;
gitignore) echo ".jobs.db" >> .gitignore; exit 0 ;;
hook) exit 0 ;;
esac
echo "unexpected: job $*"
exit 1
`
	hookScript = `#!/bin/sh
echo "%[2]s $*" >> %[1]q
[ "$1 $2" = "hook install" ] && exit 0
exit 1
`
)

// macForNew is a Mac with Apple's tools, Homebrew, every tool from the tap and a
// GitHub sign-in, as pretend programs that do just enough of what the real ones do.
func macForNew(t *testing.T) *world {
	t.Helper()
	w := macWithTools(t)
	calls := w.calls()
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(w.root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("Library/Developer/CommandLineTools/usr/bin/git", fmt.Sprintf(gitScript, calls, filepath.Join(w.root, "gitconfig")))
	write("opt/homebrew/bin/gh", fmt.Sprintf(ghScript, calls, w.signedOut()))
	write("opt/homebrew/bin/agents", fmt.Sprintf(agentsScript, calls))
	write("opt/homebrew/bin/job", fmt.Sprintf(jobScript, calls))
	write("opt/homebrew/bin/contextfill", fmt.Sprintf(hookScript, calls, "contextfill"))
	touchExec(t, filepath.Join(w.root, "opt/homebrew/bin/sleepy"))
	return w
}

func (w *world) calls() string { return filepath.Join(w.root, "calls.log") }

// signedOut is the file that signs the pretend gh out of GitHub.
func (w *world) signedOut() string { return filepath.Join(w.root, "gh-signed-out") }

func (w *world) signOut(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(w.signedOut(), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// wantFailed is a run that stopped with code, having streamed its work to stdout, and whose stderr holds each fragment.
func (r result) wantFailed(t *testing.T, code int, fragments ...string) {
	t.Helper()
	if r.code != code {
		t.Errorf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", r.code, code, r.stdout, r.stderr)
	}
	for _, f := range append([]string{"next: "}, fragments...) {
		if !strings.Contains(r.stderr, f) {
			t.Errorf("stderr lacks %q:\n%s", f, r.stderr)
		}
	}
}

// ran is every command the pretend programs saw, in order.
func (w *world) ran(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(w.calls())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
}

func (w *world) project() string { return filepath.Join(w.home, "Projects", "pine-bakery") }

// tidyNew replaces what changes from run to run in new's output.
func (w *world) tidyNew(s string) string {
	s = w.tidy(s)
	// The log's folder is named for a hash of the project's path, which is in a temporary folder.
	s = regexp.MustCompile(`/new/pine-bakery-[0-9a-f]{8}/`).ReplaceAllString(s, "/new/pine-bakery-$$HASH/")
	return strings.ReplaceAll(s, w.home, "$HOME")
}

// noPage checks that nothing touched the page's session or started a server.
func (w *world) noPage(t *testing.T) {
	t.Helper()
	for _, name := range []string{"session.jsonl", "server.json", "server.lock"} {
		if _, err := os.Stat(filepath.Join(w.dir, name)); err == nil {
			t.Errorf("new wrote the page's %s", name)
		}
	}
}

func TestNewWithYesSetsUpTheProjectWithTheDefaultsAndNoPage(t *testing.T) {
	w := macForNew(t)
	r := w.run(t, "new", "pine-bakery", "--yes")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	golden(t, "new-yes.txt", w.tidyNew("$ bootstrap new pine-bakery --yes\n"+r.stdout))
	for _, name := range []string{".git", ".gitignore", "AGENTS.md", "local", ".jobs.db"} {
		if _, err := os.Lstat(filepath.Join(w.project(), name)); err != nil {
			t.Errorf("the project lacks %s", name)
		}
	}
	ran := w.ran(t)
	for _, want := range []string{
		"git init", "git config --global user.name Ada Lovelace", "git config --global user.email 1234+ada@users.noreply.github.com",
		"agents init github.com/bensyverson/agents-md", "agents add stage-build", "job init --as claude",
		"contextfill hook install --harness claude-code", "job hook install --harness claude-code",
	} {
		if !slices.Contains(ran, want) {
			t.Errorf("never ran %q:\n%s", want, strings.Join(ran, "\n"))
		}
	}
	w.noPage(t)
}

func TestNewAgainChangesNothingAndAsksNothing(t *testing.T) {
	w := macForNew(t)
	if r := w.run(t, "new", "pine-bakery", "--yes"); r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	before := len(w.ran(t))
	files := snapshotDir(t, w.project())
	gitconfig, _ := os.ReadFile(filepath.Join(w.root, "gitconfig"))

	// A person at a terminal, who has nothing to type: any question would read the end of input and fail.
	var out bytes.Buffer
	code := cli.Run([]string{"new", "pine-bakery"}, w.terminal("", &out))
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if again := w.ran(t)[before:]; !slices.Equal(again, []string{"gh auth status", "git config --global --get user.name", "git config --global --get user.email"}) {
		t.Errorf("a second run ran more than inspections:\n%s", strings.Join(again, "\n"))
	}
	if got := snapshotDir(t, w.project()); !equalMaps(got, files) {
		t.Errorf("a second run changed the project")
	}
	if now, _ := os.ReadFile(filepath.Join(w.root, "gitconfig")); !bytes.Equal(now, gitconfig) {
		t.Errorf("a second run changed Git's settings")
	}
	for _, asked := range []string{"Press Enter", "Type ", "Choose"} {
		if strings.Contains(out.String(), asked) {
			t.Errorf("a second run asked something:\n%s", out.String())
		}
	}
}

func TestNewWithNoNameOutsideAProject(t *testing.T) {
	w := macForNew(t)
	w.run(t, "new").wantErr(t, 2, "this folder isn't a project, so new needs a name for one", "next: bootstrap new <name>")
	w.run(t, "new", "--yes").wantErr(t, 2, "next: bootstrap new <name>")
	w.noPage(t)
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("a refused new made ~/Projects")
	}
}

func TestNewWithANameInsideAProject(t *testing.T) {
	w := macForNew(t)
	if err := os.MkdirAll(filepath.Join(w.cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.run(t, "new", "other", "--yes").wantErr(t, 2, "this folder is already a project, so it's set up where it is", "next: bootstrap new with no name sets up this folder; to make a new project, cd out of it first")
}

func TestNewMakesWhatTheFilesSay(t *testing.T) {
	for _, tc := range []struct{ file, making, add string }{
		{"package.json", "website", "agents add web"},
		{"Package.swift", "app", "agents add swift swiftui"},
		{"notes.txt", "other", ""},
	} {
		t.Run(tc.file, func(t *testing.T) {
			w := macForNew(t)
			if err := os.MkdirAll(filepath.Join(w.cwd, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(w.cwd, tc.file), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			r := w.run(t, "new", "--yes", "--no-xcode")
			if r.code != 0 {
				t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
			}
			if !strings.Contains(r.stdout, "note person chose "+tc.making+" on making\n") {
				t.Errorf("making isn't %s:\n%s", tc.making, r.stdout)
			}
			adds := slices.DeleteFunc(w.ran(t), func(c string) bool { return !strings.HasPrefix(c, "agents add") })
			if tc.add == "" && len(adds) != 0 || tc.add != "" && !slices.Equal(adds, []string{tc.add}) {
				t.Errorf("ran %q, want %q", adds, tc.add)
			}
		})
	}
}

func TestNewStopsAtTroubleAndARunAgainPicksUp(t *testing.T) {
	w := macForNew(t)
	if err := os.Remove(filepath.Join(w.root, "opt/homebrew/bin/sleepy")); err != nil {
		t.Fatal(err)
	}
	trouble := w.run(t, "new", "pine-bakery", "--yes")
	if trouble.code != 4 {
		t.Fatalf("exit %d\n%s\n%s", trouble.code, trouble.stdout, trouble.stderr)
	}
	again := w.run(t, "new", "pine-bakery", "--yes")
	if again.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", again.code, again.stdout, again.stderr)
	}
	golden(t, "new-trouble.txt", w.tidyNew("$ bootstrap new pine-bakery --yes (exit 4)\n"+trouble.stdout+"(stderr)\n"+trouble.stderr+
		"$ bootstrap new pine-bakery --yes\n"+again.stdout))
}

func TestNewSkipAfterTrouble(t *testing.T) {
	w := macForNew(t)
	if err := os.Remove(filepath.Join(w.root, "opt/homebrew/bin/sleepy")); err != nil {
		t.Fatal(err)
	}
	trouble := w.run(t, "new", "pine-bakery", "--yes")
	trouble.wantFailed(t, 4, "next: read the log, fix what you can, then bootstrap new pine-bakery; or, to go on without sleepy: bootstrap new pine-bakery --skip sleepy")
	if r := w.run(t, "new", "pine-bakery", "--yes", "--skip", "sleepy"); r.code != 0 || strings.Contains(r.stdout, "installed sleepy") {
		t.Errorf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
}

func TestNewAQuestionWithNoDefault(t *testing.T) {
	w := macForNew(t)
	w.signOut(t)
	w.run(t, "new", "pine-bakery", "--yes", "--no-github").wantFailed(t, 2,
		"nothing answers “What name should Git put on your work?”",
		"next: bootstrap new pine-bakery --git-name <name> --git-email <email>, or add --no-identity")
	r := w.run(t, "new", "pine-bakery", "--yes", "--no-github", "--git-name", "Ada Lovelace", "--git-email", "ada@example.com")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if ran := w.ran(t); !slices.Contains(ran, "git config --global user.email ada@example.com") {
		t.Errorf("ran:\n%s", strings.Join(ran, "\n"))
	}
}

func TestNewRefusesBadFlags(t *testing.T) {
	w := macForNew(t)
	for _, tc := range []struct {
		args      []string
		fragments []string
	}{
		{[]string{"new", "pine-bakery", "--making", "game"}, []string{`--making: unknown choice "game"`, "valid: website, app, code, other", "next: bootstrap new --help"}},
		{[]string{"new", "pine-bakery", "--skip", "jobs"}, []string{"--skip: jobs is required, so it can't be skipped", "valid: sleepy, peep, woodcase"}},
		{[]string{"new", "pine-bakery", "--skip", "nope"}, []string{`--skip: there's no tool "nope" to skip`}},
		{[]string{"new", "../up"}, []string{`"../up" can't be a folder name`}},
		{[]string{"new", "a", "b"}, []string{`unexpected argument "b"`, "next: bootstrap new --help"}},
		{[]string{"new", "pine-bakery", "--agent", "nope"}, []string{`unknown agent "nope"`, "next: bootstrap new --help"}},
		{[]string{"new", "pine-bakery", "--git-email", "nope"}, []string{`--git-email: "nope" doesn't look like an email address`}},
	} {
		w.run(t, tc.args...).wantErr(t, 2, tc.fragments...)
	}
	w.noPage(t)
}

func TestNewAtATerminal(t *testing.T) {
	w := macForNew(t)
	var out bytes.Buffer
	// The name, then Enter for Git, Enter for the GitHub identity, 1 for a website, then Enter twice.
	code := cli.Run([]string{"new"}, w.terminal("Pine Bakery\n\n\n1\n\n\n", &out))
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	golden(t, "new-terminal.txt", w.tidyNew("$ bootstrap new\n"+out.String()))
	if !slices.Contains(w.ran(t), "agents add web stage-build") {
		t.Errorf("typing 1 didn't make a website:\n%s", strings.Join(w.ran(t), "\n"))
	}
}

func TestNewAtATerminalAsksAgainAfterARefusal(t *testing.T) {
	w := macForNew(t)
	w.signOut(t)
	var out bytes.Buffer
	code := cli.Run([]string{"new", "pine-bakery", "--no-github"}, w.terminal("\nAda\nnope\nada@example.com\n\n\n\n", &out))
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), `"nope" doesn't look like an email address`) {
		t.Errorf("no refusal:\n%s", out.String())
	}
}

// terminal is this world as a person at a terminal sees it, run in this process.
func (w *world) terminal(input string, out *bytes.Buffer) cli.Env {
	lookup := func(key string) string {
		for _, kv := range slices.Backward(w.env) {
			if k, v, ok := strings.Cut(kv, "="); ok && k == key {
				return v
			}
		}
		return ""
	}
	return cli.Env{
		Getenv:   lookup,
		Environ:  func() []string { return w.env },
		Cwd:      func() (string, error) { return w.cwd, nil },
		Exe:      func() (string, error) { return binary, nil },
		Stdin:    strings.NewReader(input),
		Terminal: func() bool { return true },
		Stdout:   out,
		Stderr:   out,
	}
}

// snapshotDir is every file under dir with its content, so a test can see what changed.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			out[path] = "/"
			return nil
		}
		body, err := os.ReadFile(path)
		out[path] = string(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
