package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/ from the binary's output")

var binary string

func TestMain(m *testing.M) {
	flag.Parse()
	dir, err := os.MkdirTemp("", "bootstrap-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "bootstrap")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("%v\n%s", err, out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// world is one private set of folders for the binary under test.
type world struct {
	home, dir, root, cwd string
	env                  []string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := &world{home: filepath.Join(base, "home"), dir: filepath.Join(base, "state"), root: filepath.Join(base, "root")}
	w.cwd = filepath.Join(w.home, "pine-bakery")
	for _, d := range []string{w.home, w.root, w.cwd} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.env = []string{"PATH=/usr/bin:/bin", "HOME=" + w.home, "BOOTSTRAP_DIR=" + w.dir, "BOOTSTRAP_ROOT=" + w.root, "BOOTSTRAP_OPEN=/usr/bin/true"}
	return w
}

func (w *world) with(env ...string) *world {
	copy := *w
	copy.env = append(append([]string{}, w.env...), env...)
	return &copy
}

type result struct {
	stdout, stderr string
	code           int
}

func (w *world) run(t *testing.T, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env, cmd.Dir = w.env, w.cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{stdout.String(), stderr.String(), code}
}

func (r result) want(t *testing.T, code int, stdout, stderr string) {
	t.Helper()
	if r.code != code || r.stdout != stdout || r.stderr != stderr {
		t.Errorf("got exit %d\nstdout:\n%s\nstderr:\n%s\nwant exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr, code, stdout, stderr)
	}
}

func (r result) wantErr(t *testing.T, code int, fragments ...string) {
	t.Helper()
	if r.code != code {
		t.Errorf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", r.code, code, r.stdout, r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("an error wrote to stdout: %q", r.stdout)
	}
	if !strings.Contains(r.stderr, "next: ") {
		t.Errorf("an error must name the next command:\n%s", r.stderr)
	}
	for _, f := range fragments {
		if !strings.Contains(r.stderr, f) {
			t.Errorf("stderr lacks %q:\n%s", f, r.stderr)
		}
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (regenerate with go test ./cmd/bootstrap -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs:\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

func touchExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPrimer(t *testing.T) {
	w := newWorld(t)
	r := w.run(t)
	if r.code != 0 || r.stderr != "" {
		t.Errorf("exit %d, stderr %q", r.code, r.stderr)
	}
	golden(t, "primer.txt", r.stdout)
	for _, verb := range []string{"detect", "tools", "screens", "status", "serve", "show", "wait", "stop"} {
		if !strings.Contains(r.stdout, "bootstrap "+verb) {
			t.Errorf("primer lacks %s", verb)
		}
	}
	if !strings.Contains(r.stdout, "Run first: bootstrap status") {
		t.Errorf("primer lacks the first command:\n%s", r.stdout)
	}
}

func TestPrimerNeedsNoSessionAndLeavesNoTrace(t *testing.T) {
	w := newWorld(t)
	if r := w.run(t); r.code != 0 {
		t.Fatalf("exit %d", r.code)
	}
	if _, err := os.Stat(w.dir); !os.IsNotExist(err) {
		t.Errorf("the primer touched the session folder: %v", err)
	}
}

func TestPrimerNeedsNoNetwork(t *testing.T) {
	sandbox, err := exec.LookPath("sandbox-exec")
	if err != nil {
		t.Skip("no sandbox-exec")
	}
	w := newWorld(t)
	cmd := exec.Command(sandbox, "-p", "(version 1)(allow default)(deny network*)", binary)
	cmd.Env, cmd.Dir = w.env, w.cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Run first: bootstrap status") {
		t.Errorf("output:\n%s", out)
	}
}

func TestEveryVerbHasHelpWithAnExample(t *testing.T) {
	w := newWorld(t)
	for _, verb := range []string{"detect", "tools", "screens", "status", "serve", "show", "wait", "stop"} {
		t.Run(verb, func(t *testing.T) {
			r := w.run(t, verb, "--help")
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
			}
			for _, want := range []string{"Class: ", "Example:\n  bootstrap " + verb} {
				if !strings.Contains(r.stdout, want) {
					t.Errorf("help lacks %q:\n%s", want, r.stdout)
				}
			}
		})
	}
	if _, err := os.Stat(w.dir); !os.IsNotExist(err) {
		t.Errorf("help touched the session folder: %v", err)
	}
}

func TestUnknownVerb(t *testing.T) {
	newWorld(t).run(t, "nope").wantErr(t, 2, `unknown command "nope"`, "valid: detect, tools, screens, status, serve, show, wait, stop", "next: bootstrap")
}

func TestUnknownFlag(t *testing.T) {
	newWorld(t).run(t, "detect", "--nope").wantErr(t, 2, `unknown flag --nope`, "valid: --json, --help", "next: bootstrap detect --help")
}

func TestFlagMissingItsValue(t *testing.T) {
	newWorld(t).run(t, "show", "tool", "jobs", "--status").wantErr(t, 2, `--status needs a value`, "next: bootstrap show --help")
}

func TestExtraArgument(t *testing.T) {
	newWorld(t).run(t, "detect", "extra").wantErr(t, 2, `unexpected argument "extra"`, "next: bootstrap detect --help")
}

func TestDetectRows(t *testing.T) {
	w := newWorld(t)
	touchExec(t, filepath.Join(w.root, "usr/bin/git")) // the stub that fools command -v
	touchExec(t, filepath.Join(w.root, "opt/homebrew/bin/brew"))
	touchExec(t, filepath.Join(w.home, "go/bin/job"))
	r := w.run(t, "detect")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	disk := regexp.MustCompile(`(?m)^disk free_gb=[0-9.]+ apple_step=(enough|short)\n`)
	if !disk.MatchString(r.stdout) {
		t.Errorf("no disk row:\n%s", r.stdout)
	}
	lines := strings.SplitN(disk.ReplaceAllString(r.stdout, ""), "\n", 2)
	if !strings.HasPrefix(lines[0], "chip=") || !strings.HasSuffix(lines[0], "macos=unknown") {
		t.Errorf("first row = %q", lines[0])
	}
	want := fmt.Sprintf(`folder kind=project files=no git=no path=%s
tool clt missing
tool homebrew present path=/opt/homebrew/bin/brew
tool git missing
tool agents missing
tool jobs present path=~/go/bin/job
tool contextfill missing
tool sleepy missing
tool peep missing
tool woodcase missing
next: bootstrap tools
`, w.cwd)
	if lines[1] != want {
		t.Errorf("got:\n%s\nwant:\n%s", lines[1], want)
	}
}

func TestDetectQuotesAPathWithASpace(t *testing.T) {
	w := newWorld(t)
	w.cwd = filepath.Join(w.home, "Claude Code")
	if err := os.Mkdir(w.cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	r := w.run(t, "detect")
	if want := fmt.Sprintf("folder kind=generic files=no git=no path=%q\n", w.cwd); !strings.Contains(r.stdout, want) {
		t.Errorf("got:\n%s\nwant row %s", r.stdout, want)
	}
}

func TestDetectJSON(t *testing.T) {
	w := newWorld(t)
	w.cwd = w.home
	r := w.run(t, "detect", "--json")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	var got struct {
		Chip, Arch, MacOS string
		Folder            struct{ Path, Kind, Files, Git string }
		Tools             []struct{ ID, State, Path string }
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if got.Folder.Kind != "home" || len(got.Tools) != 9 || got.Tools[0].ID != "clt" || got.Tools[0].State != "missing" || got.Arch == "" {
		t.Errorf("got %+v", got)
	}
}

func TestToolsRows(t *testing.T) {
	w := newWorld(t)
	r := w.run(t, "tools", "--agent", "opencode")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	golden(t, "tools.txt", r.stdout)
	if !strings.Contains(r.stdout, "sleepy skippable") || !strings.Contains(r.stdout, "jobs required") {
		t.Errorf("tools must say what can be skipped:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, `name=Jobs`) {
		t.Errorf("the tool is called Jobs:\n%s", r.stdout)
	}
}

func TestToolsLeavesThePlaceholderWithNoAgentKnown(t *testing.T) {
	r := newWorld(t).run(t, "tools")
	if !strings.Contains(r.stdout, "Lets {ai} see websites.") {
		t.Errorf("output:\n%s", r.stdout)
	}
	r = newWorld(t).with("CLAUDECODE=1").run(t, "tools")
	if !strings.Contains(r.stdout, "Lets Claude see websites.") {
		t.Errorf("output:\n%s", r.stdout)
	}
}

func TestToolsJSON(t *testing.T) {
	r := newWorld(t).run(t, "tools", "--json", "--agent", "claude-code")
	var got struct {
		Tools []struct {
			ID, Name, URL, Say string
			Required           bool
		}
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if len(got.Tools) != 9 || got.Tools[4].Name != "Jobs" || !got.Tools[4].Required || got.Tools[4].Say != "Lets Claude manage big projects." {
		t.Errorf("got %+v", got)
	}
}

func TestScreensRows(t *testing.T) {
	r := newWorld(t).run(t, "screens")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	golden(t, "screens.txt", r.stdout)
	for _, want := range []string{"statuses=downloading,installing", "reasons=network,other", "choices=see-plan,skip-tour"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestScreensJSON(t *testing.T) {
	r := newWorld(t).run(t, "screens", "--json")
	var got struct {
		Screens []struct {
			Screen, Mode string
			Statuses     []string
			Internal     bool
		}
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if len(got.Screens) != 20 || got.Screens[0].Screen != "welcome" || !got.Screens[19].Internal {
		t.Errorf("got %+v", got)
	}
}

func TestReadsNeedNoSessionAndLeaveNoTrace(t *testing.T) {
	w := newWorld(t)
	for _, verb := range []string{"detect", "tools", "screens"} {
		if r := w.run(t, verb); r.code != 0 {
			t.Errorf("%s: exit %d", verb, r.code)
		}
	}
	if _, err := os.Stat(w.dir); !os.IsNotExist(err) {
		t.Errorf("a read touched the session folder: %v", err)
	}
}

func TestOutOfClosedLists(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"unknown screen", []string{"show", "nope"}, []string{`unknown screen "nope"`, "valid: welcome, list, fast, apple-ask", "next: bootstrap screens"}},
		{"the old Mac screen is gone", []string{"show", "oldMac"}, []string{`unknown screen "oldMac"`, "valid: welcome"}},
		{"unknown tool", []string{"show", "tool", "nope"}, []string{`can't show tool "nope"`, "valid: agents, jobs, contextfill, sleepy, peep, woodcase", "next: bootstrap tools"}},
		{"tool needs a tool", []string{"show", "tool"}, []string{"needs a tool", "valid: agents, jobs"}},
		{"unknown status", []string{"show", "tool", "jobs", "--status", "nope"}, []string{`unknown status "nope"`, "valid: downloading, installing", "next: bootstrap screens"}},
		{"status on a screen with none", []string{"show", "welcome", "--status", "x"}, []string{"takes no status", "next: bootstrap screens"}},
		{"unknown reason", []string{"show", "trouble", "jobs", "--reason", "nope"}, []string{`unknown reason "nope"`, "valid: network, other"}},
		{"trouble needs a reason", []string{"show", "trouble", "jobs"}, []string{"needs a reason", "valid: network, other"}},
		{"lost can't be shown", []string{"show", "lost"}, []string{"appears on its own", "next: bootstrap screens"}},
		{"bad expect", []string{"show", "welcome", "--expect", "soon"}, []string{`--expect "soon"`, "20m", "next: bootstrap show --help"}},
		{"negative expect", []string{"show", "welcome", "--expect", "-5m"}, []string{`--expect "-5m"`, "next: bootstrap show --help"}},
		{"no screen", []string{"show"}, []string{"needs a screen", "valid: welcome", "next: bootstrap screens"}},
		{"unknown wait", []string{"wait", "nope"}, []string{`unknown wait target "nope"`, "valid: answer, clt, brew", "next: bootstrap wait --help"}},
		{"no wait target", []string{"wait"}, []string{"needs a target", "valid: answer, clt, brew"}},
		{"bad timeout", []string{"wait", "clt", "--timeout", "soon"}, []string{`--timeout "soon"`, "next: bootstrap wait --help"}},
		{"unknown agent", []string{"serve", "--agent", "nope"}, []string{`unknown agent "nope"`, "valid: claude-code, opencode", "next: bootstrap serve --help"}},
		{"unknown agent on tools", []string{"tools", "--agent", "nope"}, []string{`unknown agent "nope"`, "valid: claude-code, opencode"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newWorld(t).run(t, c.args...).wantErr(t, 2, c.want...)
		})
	}
}

func TestJSONErrorsAreOnStdout(t *testing.T) {
	r := newWorld(t).run(t, "show", "nope", "--json")
	if r.code != 2 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	var got struct {
		Error string
		Exit  int
		Valid []string
		Next  string
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if got.Exit != 2 || got.Error == "" || got.Next == "" || len(got.Valid) == 0 {
		t.Errorf("got %+v", got)
	}
}
