package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// running is what run --json reports.
type running struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Browser string `json:"browser"`
	Joined  bool   `json:"joined"`
	Next    string `json:"next"`
}

func (r running) page() served { return served{URL: r.URL, PID: r.PID} }

// start runs the setup with the real flow and the real runner, on the world's pretend Mac.
func (w *world) start(t *testing.T) running {
	t.Helper()
	r := w.run(t, "run", "--agent", "claude-code", "--json")
	if r.code != 0 {
		t.Fatalf("run exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	var got running
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	t.Cleanup(func() { syscall.Kill(got.PID, syscall.SIGKILL) })
	return got
}

// brewScript is a pretend Homebrew: sleepy fails to install the first time and
// installs on the next try; anything else it wasn't told about is an error.
const brewScript = `#!/bin/sh
case "$1 $2" in
"trust bensyverson/tap") exit 0 ;;
"install bensyverson/tap/sleepy")
  if [ -e %[1]q ]; then
    touch %[2]q && chmod +x %[2]q
    echo "==> Pouring sleepy--0.2.0.arm64_sequoia.bottle.tar.gz"
    echo "/opt/homebrew/Cellar/sleepy/0.2.0: 3 files, 4.1MB"
    exit 0
  fi
  touch %[1]q
  echo "Error: sleepy: no bottle available"
  exit 1 ;;
esac
echo "unexpected: brew $*"
exit 1
`

// macWithTools is a Mac with Apple's tools, Homebrew and every tool but sleepy,
// so the flow's first change is installing sleepy, which fails once.
func macWithTools(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	touchExec(t, filepath.Join(w.root, "Library/Developer/CommandLineTools/usr/bin/git"))
	for _, tool := range []string{"agents", "job", "contextfill", "peep", "woodcase"} {
		touchExec(t, filepath.Join(w.root, "opt/homebrew/bin", tool))
	}
	brew := filepath.Join(w.root, "opt/homebrew/bin/brew")
	touchExec(t, brew)
	body := fmt.Sprintf(brewScript, filepath.Join(w.root, "sleepy-tried"), filepath.Join(w.root, "opt/homebrew/bin/sleepy"))
	if err := os.WriteFile(brew, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return w
}

// screenOnPage waits for the flow to put a screen up, and returns its seq.
func (w *world) screenOnPage(t *testing.T, id string) int {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		var got struct {
			Screen *struct {
				Screen string
				Seq    int
			}
		}
		r := w.run(t, "status", "--json")
		if json.Unmarshal([]byte(r.stdout), &got) == nil && got.Screen != nil && got.Screen.Screen == id {
			return got.Screen.Seq
		}
	}
	t.Fatalf("the flow never showed %s", id)
	return 0
}

// skipTour clicks through the plan step, as a person would.
func (w *world) skipTour(t *testing.T, r running) {
	t.Helper()
	seq := w.screenOnPage(t, "welcome")
	if code, body := r.page().click(t, seq, "skip-tour", ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
}

// tidy replaces what changes from run to run.
func (w *world) tidy(s string) string {
	s = strings.ReplaceAll(s, w.dir, "$DIR")
	s = regexp.MustCompile(`url=\S+`).ReplaceAllString(s, "url=$$URL")
	return regexp.MustCompile(`pid=\d+`).ReplaceAllString(s, "pid=$$PID")
}

func (w *world) store() session.Store { return session.Store{Dir: w.dir} }

func TestRunStartsTheFlowAndReturnsAtOnce(t *testing.T) {
	w := macWithTools(t)
	began := time.Now()
	r := w.run(t, "run", "--agent", "claude-code")
	if took := time.Since(began); took > 5*time.Second {
		t.Errorf("run took %v", took)
	}
	t.Cleanup(func() {
		if info, err := w.store().ReadServerInfo(); err == nil {
			syscall.Kill(info.PID, syscall.SIGKILL)
		}
	})
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if got := w.tidy(r.stdout); got != "running url=$URL pid=$PID browser=opened joined=no\nnext: bootstrap wait\n" {
		t.Errorf("stdout:\n%s", got)
	}
	w.screenOnPage(t, "welcome")
}

func TestSecondRunJoinsTheFlow(t *testing.T) {
	w := macWithTools(t)
	first := w.start(t)
	second := w.start(t)
	if !second.Joined || second.PID != first.PID || second.Next != "bootstrap wait" {
		t.Errorf("second run = %+v", second)
	}
	body, err := os.ReadFile(filepath.Join(w.dir, "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(body), `"kind":"drive"`); n != 1 {
		t.Errorf("%d flows started", n)
	}
}

func TestRunNeedsToKnowTheAgent(t *testing.T) {
	newWorld(t).run(t, "run").wantErr(t, 2, "nothing says which app this is", "valid: claude-code, opencode", "next: bootstrap run --agent <app>")
}

func TestRunWhenAPageIsOpenWithoutTheFlow(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	w.run(t, "run", "--agent", "claude-code").wantErr(t, 5, "the page is open, but the setup isn't running in it", "next: bootstrap stop, then bootstrap run --agent claude-code")
}

// The transcript: a trouble handoff, a retry, then a wait that runs out of budget
// while the person names the project.
func TestTroubleRetryTranscript(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)

	trouble := w.run(t, "wait")
	if trouble.code != 0 || trouble.stderr != "" {
		t.Fatalf("exit %d\n%s\n%s", trouble.code, trouble.stdout, trouble.stderr)
	}
	resume := w.run(t, "resume", "--retry")
	resume.want(t, 0, "resumed way=retry step=tools tool=sleepy\nnext: bootstrap wait\n", "")
	budget := w.run(t, "wait", "--timeout", "2s")
	if budget.code != 3 || budget.stdout != "" {
		t.Fatalf("exit %d\n%s\n%s", budget.code, budget.stdout, budget.stderr)
	}
	golden(t, "flow-retry.txt", w.tidy("$ bootstrap wait\n"+trouble.stdout+"$ bootstrap resume --retry\n"+resume.stdout+"$ bootstrap wait --timeout 2s (exit 3, stderr)\n"+budget.stderr))
}

func TestTroubleSkip(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)
	if got := w.run(t, "wait"); got.code != 0 {
		t.Fatalf("exit %d\n%s", got.code, got.stderr)
	}
	w.run(t, "resume", "--skip").want(t, 0, "resumed way=skip step=tools tool=sleepy\nnext: bootstrap wait\n", "")
	got := w.run(t, "wait", "--timeout", "2s")
	if got.code != 3 || !strings.Contains(got.stderr, "step tools done 4 of 13\n") || strings.Contains(got.stderr, "installed sleepy") {
		t.Errorf("exit %d\n%s", got.code, got.stderr)
	}
	if state, _ := w.store().Load(); state.Chosen == nil || strings.Contains(strings.Join(state.Chosen, ","), "sleepy") {
		t.Errorf("skipping sleepy drops it from the picks: %v", state.Chosen)
	}
}

func TestWaitRepeatsAnOpenHandoff(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)
	first := w.run(t, "wait")
	again := w.run(t, "wait")
	if again.code != 0 || !strings.Contains(again.stdout, "handoff trouble step=tools tool=sleepy reason=exit\n") || !strings.Contains(again.stdout, "next: read the log") {
		t.Errorf("exit %d\n%s\n%s", again.code, again.stdout, again.stderr)
	}
	if strings.Contains(again.stdout, "person chose") {
		t.Errorf("a second wait retells what the first told:\n%s\nfirst:\n%s", again.stdout, first.stdout)
	}
}

func TestWaitTakesTheHandoff(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)
	w.run(t, "wait")
	if state, _ := w.store().Load(); state.Open == nil || !state.Open.Taken {
		t.Errorf("open = %+v", state.Open)
	}
}

func TestWaitBudgetWithNothingNew(t *testing.T) {
	w := macWithTools(t)
	w.start(t)
	w.screenOnPage(t, "welcome")
	w.run(t, "wait", "--timeout", "1s")
	w.run(t, "wait", "--timeout", "1s").wantErr(t, 3, "still working, run wait again", "state working step=plan 1 of 13 screen=welcome", "next: bootstrap wait")
}

func TestWaitJSON(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)
	got := w.run(t, "wait", "--json")
	var value struct {
		Updates []session.Update
		Open    *session.Open
		Next    []string
	}
	if err := json.Unmarshal([]byte(got.stdout), &value); err != nil || got.code != 0 {
		t.Fatalf("exit %d %v\n%s", got.code, err, got.stdout)
	}
	if value.Open == nil || value.Open.Handoff.Tool != "sleepy" || len(value.Next) != 3 || len(value.Updates) == 0 {
		t.Errorf("value = %+v", value)
	}
}

func TestWaitBudgetJSONCarriesTheUpdates(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.skipTour(t, r)
	w.run(t, "wait")
	w.run(t, "resume", "--retry")
	got := w.run(t, "wait", "--timeout", "2s", "--json")
	var value struct {
		Error   string
		Exit    int
		Updates []session.Update
		Next    string
	}
	if err := json.Unmarshal([]byte(got.stdout), &value); err != nil || got.code != 3 {
		t.Fatalf("exit %d %v\n%s", got.code, err, got.stdout)
	}
	if value.Exit != 3 || value.Next != "bootstrap wait" || len(value.Updates) == 0 || value.Updates[0].Note != "installed sleepy 0.2.0" {
		t.Errorf("value = %+v", value)
	}
}

// handOff writes a handoff into the running flow's log, as the flow does at its last steps.
func (w *world) handOff(t *testing.T, h session.Handoff) {
	t.Helper()
	if _, err := w.store().HandOff(h, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestPhoneHandoffSaysWhatToDo(t *testing.T) {
	w := macWithTools(t)
	w.start(t)
	w.screenOnPage(t, "welcome")
	w.run(t, "wait", "--timeout", "1s")
	w.handOff(t, session.Handoff{Kind: session.HandoffPhone, Step: "phone", What: "the person wants to use their agent from their phone: explain /remote-control in the chat", Ways: []session.Way{session.WayTalk}})
	w.run(t, "wait").want(t, 0, "handoff phone step=phone\n"+
		"what: the person wants to use their agent from their phone: explain /remote-control in the chat\n"+
		"state working step=plan 1 of 13 screen=welcome\n"+
		"next: do what the phone handoff says in the chat, then bootstrap wait\n", "")
}

func TestDoneHandoffSaysWhatToDoAndThenWaitReturnsAtOnce(t *testing.T) {
	w := macWithTools(t)
	w.start(t)
	w.screenOnPage(t, "welcome")
	w.run(t, "wait", "--timeout", "1s")
	w.handOff(t, session.Handoff{Kind: session.HandoffDone, Step: "done", What: "the setup is done: sum it up in the chat"})
	w.run(t, "wait").want(t, 0, "handoff done step=done\n"+
		"what: the setup is done: sum it up in the chat\n"+
		"state finished\n"+
		"next: do what the done handoff says in the chat; the setup is over, so there is nothing more to run\n", "")
	began := time.Now()
	w.run(t, "wait").want(t, 0, "state finished\nnext: the setup is over, so there is nothing more to run\n", "")
	if took := time.Since(began); took > 5*time.Second {
		t.Errorf("a wait after the end took %v", took)
	}
}

func TestTroubleWithoutSkipOffersNoSkip(t *testing.T) {
	w := macWithTools(t)
	w.start(t)
	w.screenOnPage(t, "welcome")
	w.handOff(t, session.Handoff{Kind: session.HandoffTrouble, Step: "apple", Tool: "clt", Reason: session.ReasonDiskShort,
		What: "only 12.0 GB is free, and Apple's tools need 20 GB", Ways: []session.Way{session.WayRetry, session.WayTalk}})
	got := w.run(t, "wait")
	if got.code != 0 || strings.Contains(got.stdout, "--skip") {
		t.Errorf("exit %d\n%s", got.code, got.stdout)
	}
	for _, want := range []string{
		"handoff trouble step=apple tool=clt reason=disk-short\n",
		"what: only 12.0 GB is free, and Apple's tools need 20 GB\n",
		"next: look into what happened, fix what you can, then bootstrap resume --retry\n",
		"next: or, to talk it over: ask the person in the chat (the page says \"Let’s talk in Claude Code.\"), then bootstrap resume --retry\n",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("lacks %q:\n%s", want, got.stdout)
		}
	}
	w.run(t, "resume", "--skip").wantErr(t, 2, `this handoff can't be resumed with "skip"`, "valid: retry", "next: bootstrap resume --retry")
}

func TestStatusDuringARunSaysWait(t *testing.T) {
	w := macWithTools(t)
	r := w.start(t)
	w.screenOnPage(t, "welcome")
	got := w.run(t, "status")
	if got.code != 0 || !strings.HasSuffix(got.stdout, "next: bootstrap wait\n") {
		t.Errorf("exit %d\n%s\n%s", got.code, got.stdout, got.stderr)
	}
	syscall.Kill(r.PID, syscall.SIGKILL)
	for deadline := time.Now().Add(5 * time.Second); w.store().ServerRunning() && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	got = w.run(t, "status")
	if got.code != 0 || !strings.HasSuffix(got.stdout, "next: bootstrap run --agent claude-code\n") {
		t.Errorf("after the server died: exit %d\n%s\n%s", got.code, got.stdout, got.stderr)
	}
}

func TestWaitErrors(t *testing.T) {
	t.Run("no session", func(t *testing.T) {
		newWorld(t).run(t, "wait").wantErr(t, 5, "no session", "next: bootstrap run --agent <app>")
	})
	t.Run("a page without the flow", func(t *testing.T) {
		w := newWorld(t)
		w.serve(t)
		w.run(t, "wait").wantErr(t, 5, "the page is open, but the setup isn't running in it", "next: bootstrap stop, then bootstrap run --agent claude-code")
	})
	t.Run("the server is gone", func(t *testing.T) {
		w := macWithTools(t)
		r := w.start(t)
		syscall.Kill(r.PID, syscall.SIGKILL)
		for deadline := time.Now().Add(5 * time.Second); w.store().ServerRunning() && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		}
		w.run(t, "wait").wantErr(t, 5, "the page is closed, so the setup isn't running", "next: bootstrap run --agent claude-code")
	})
	t.Run("stopped", func(t *testing.T) {
		w := macWithTools(t)
		w.start(t)
		w.run(t, "stop")
		w.run(t, "wait").wantErr(t, 5, "session ended", "next: bootstrap run --agent claude-code")
	})
	t.Run("the flow halted", func(t *testing.T) {
		w := newWorld(t)
		store := w.store()
		if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Drive(time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Halt("the screens are missing", time.Now()); err != nil {
			t.Fatal(err)
		}
		w.run(t, "wait").wantErr(t, 5, "the setup stopped: the screens are missing", "next: bootstrap run --agent claude-code")
	})
	t.Run("bad timeout", func(t *testing.T) {
		newWorld(t).run(t, "wait", "--timeout", "soon").wantErr(t, 2, `--timeout "soon"`, "next: bootstrap wait --help")
	})
}

func TestResumeErrors(t *testing.T) {
	t.Run("no way", func(t *testing.T) {
		newWorld(t).run(t, "resume").wantErr(t, 2, "resume needs --retry or --skip", "valid: --retry, --skip", "next: bootstrap wait")
	})
	t.Run("both ways", func(t *testing.T) {
		newWorld(t).run(t, "resume", "--retry", "--skip").wantErr(t, 2, "resume takes one of --retry or --skip", "valid: --retry, --skip", "next: bootstrap wait")
	})
	t.Run("a flag with a value", func(t *testing.T) {
		newWorld(t).run(t, "resume", "--retry=no").wantErr(t, 2, "--retry takes no value", "next: bootstrap resume --help")
	})
	t.Run("no session", func(t *testing.T) {
		newWorld(t).run(t, "resume", "--retry").wantErr(t, 5, "no session", "next: bootstrap run --agent <app>")
	})
	t.Run("nothing to resume", func(t *testing.T) {
		w := macWithTools(t)
		w.start(t)
		w.run(t, "resume", "--retry").wantErr(t, 3, "nothing is waiting on the agent", "next: bootstrap wait")
	})
}

// A handoff for an install that ran past its time shows the limit and how long it ran.
func TestWaitShowsTheLimitPassed(t *testing.T) {
	w := newWorld(t)
	store := w.store()
	if _, err := store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Drive(time.Now()); err != nil {
		t.Fatal(err)
	}
	release, err := store.LockServer()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	h := session.Handoff{Kind: session.HandoffTrouble, Step: "apple", Tool: "clt", Reason: session.ReasonTimeout, What: "Apple's tools didn't arrive within 1h0m0s",
		Evidence: session.Evidence{Limit: time.Hour, Elapsed: time.Hour + time.Second}, Ways: []session.Way{session.WayRetry}}
	if _, err := store.HandOff(h, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := w.run(t, "wait")
	if r.code != 0 || !strings.Contains(r.stdout, "limit: 1h0m0s, ran 1h0m1s\n") {
		t.Errorf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
}
