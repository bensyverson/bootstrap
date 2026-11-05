package flow_test

import (
	"errors"
	"github.com/bensyverson/bootstrap/internal/flow"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// skipOrRetry resumes with skip when it's offered, and otherwise retries after
// clearing the trouble, as an agent that fixed it would.
func (w *world) skipOrRetry(h session.Handoff) session.Way {
	if slices.Contains(h.Ways, session.WaySkip) {
		return session.WaySkip
	}
	w.mac.Clear()
	w.free = 100_000_000_000
	w.running = func(string) bool { return false }
	w.mac.QuietInstalls = false
	return session.WayRetry
}

func (w *world) only(t *testing.T) session.Handoff {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.handoffs) != 1 {
		t.Fatalf("handoffs = %+v", w.handoffs)
	}
	return w.handoffs[0]
}

// Every trouble handoff must teach: what happened, the evidence, and the ways forward.
func checkTeaches(t *testing.T, h session.Handoff) {
	t.Helper()
	if h.Kind != session.HandoffTrouble || h.What == "" || h.Reason == "" || h.Step == "" {
		t.Errorf("handoff = %+v", h)
	}
	if !slices.Contains(h.Ways, session.WayRetry) || !slices.Contains(h.Ways, session.WayTalk) {
		t.Errorf("ways = %v, want retry and talk at least", h.Ways)
	}
}

func TestUnexpectedResultsHandOff(t *testing.T) {
	cases := []struct {
		name    string
		match   string
		result  flow.Result
		reason  session.HandoffReason
		tool    string
		what    string
		skip    bool
		exit    int
		hasTail bool
	}{
		{name: "a skippable tool exits 1", match: "brew install bensyverson/tap/sleepy", result: flow.Result{Exit: 1, Output: []byte("==> Fetching\nError: no bottle available\n")},
			reason: session.ReasonExit, tool: "sleepy", what: "brew install bensyverson/tap/sleepy exited 1", skip: true, exit: 1, hasTail: true},
		{name: "a required tool exits 1", match: "brew install bensyverson/tap/job", result: flow.Result{Exit: 1, Output: []byte("Error: no bottle\n")},
			reason: session.ReasonExit, tool: "jobs", what: "brew install bensyverson/tap/job exited 1", exit: 1, hasTail: true},
		{name: "a tool times out", match: "brew install bensyverson/tap/sleepy", result: flow.Result{TimedOut: true, Output: []byte("==> Downloading\n")},
			reason: session.ReasonTimeout, tool: "sleepy", what: "brew install bensyverson/tap/sleepy ran past its 10m0s limit", skip: true, hasTail: true},
		{name: "a tool won't start", match: "brew install bensyverson/tap/sleepy", result: flow.Result{Err: errors.New("fork/exec: permission denied")},
			reason: session.ReasonNoStart, tool: "sleepy", what: "brew install bensyverson/tap/sleepy couldn't start: fork/exec: permission denied", skip: true},
		{name: "a tool says yes but isn't there", match: "brew install bensyverson/tap/sleepy", result: flow.Result{Output: []byte("Warning: already installed\n")},
			reason: session.ReasonMissing, tool: "sleepy", what: "brew install bensyverson/tap/sleepy exited 0, but /opt/homebrew/bin/sleepy isn't there", skip: true, hasTail: true},
		{name: "a check exits with a code it never uses", match: "gh auth status", result: flow.Result{Exit: 4, Output: []byte("panic\n")},
			reason: session.ReasonExit, what: "gh auth status exited 4", skip: true, exit: 4, hasTail: true},
		{name: "git init fails", match: "git init", result: flow.Result{Exit: 128, Output: []byte("fatal: cannot mkdir\n")},
			reason: session.ReasonExit, what: "git init exited 128", skip: true, exit: 128, hasTail: true},
		{name: "job init fails", match: "job init", result: flow.Result{Exit: 2, Output: []byte("error\n")},
			reason: session.ReasonExit, what: "job init --as claude exited 2", exit: 2, hasTail: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.mac.On(c.match, c.result, 0)
			w.resume = w.skipOrRetry
			if err := w.walk(); err != nil {
				t.Fatal(err)
			}
			h := w.only(t)
			checkTeaches(t, h)
			if h.Reason != c.reason || h.Tool != c.tool || h.What != c.what {
				t.Errorf("got reason=%s tool=%s what=%q", h.Reason, h.Tool, h.What)
			}
			if got := slices.Contains(h.Ways, session.WaySkip); got != c.skip {
				t.Errorf("skip offered = %v, want %v", got, c.skip)
			}
			if h.Evidence.Command == "" || h.Evidence.Exit != c.exit {
				t.Errorf("evidence = %+v", h.Evidence)
			}
			if c.hasTail {
				if len(h.Evidence.Tail) == 0 {
					t.Error("no log tail")
				}
				body, err := os.ReadFile(h.Evidence.Log)
				if err != nil || !strings.Contains(string(body), string(c.result.Output)) {
					t.Errorf("log %s = %q, %v", h.Evidence.Log, body, err)
				}
			}
			state := w.state()
			if state.Current == nil || state.Current.Request.Screen != "done" {
				t.Errorf("the walk didn't go on to done: %+v", state.Current)
			}
		})
	}
}

func TestAHandoffShowsTheChatScreen(t *testing.T) {
	w := newWorld(t)
	w.mac.On("brew install bensyverson/tap/sleepy", flow.Result{Exit: 1}, 0)
	var screenAtHandoff string
	w.resume = func(h session.Handoff) session.Way {
		state, _ := w.store.Load()
		screenAtHandoff = string(state.Current.Request.Screen)
		return session.WaySkip
	}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if screenAtHandoff != "chat" {
		t.Errorf("the page showed %q during the handoff", screenAtHandoff)
	}
	if _, err := os.Stat(filepath.Join(w.mac.Root, "/opt/homebrew/bin/sleepy")); err == nil {
		t.Error("installed a skipped tool")
	}
	if slices.Contains(w.state().Chosen, "sleepy") {
		t.Error("skipping a tool drops it from the picks")
	}
}

func TestRetryRunsTheStepAgain(t *testing.T) {
	w := newWorld(t)
	w.mac.On("brew install bensyverson/tap/job", flow.Result{Exit: 1}, 1)
	w.resume = func(session.Handoff) session.Way { return session.WayRetry }
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.mac.Root, "/opt/homebrew/bin/job")); err != nil {
		t.Error("the retry didn't install jobs")
	}
}

func TestAppleTrouble(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(w *world)
		reason session.HandoffReason
	}{
		{"too little space", func(w *world) { w.free = 18_700_000_000 }, session.ReasonDiskShort},
		{"the person cancels", func(w *world) {
			w.mac.QuietInstalls = true
			calls := 0
			w.running = func(string) bool { calls++; return calls <= 2 }
		}, session.ReasonCanceled},
		{"it never finishes", func(w *world) { w.mac.QuietInstalls = true }, session.ReasonTimeout},
		{"xcode-select fails", func(w *world) { w.mac.On("xcode-select --install", flow.Result{Exit: 1}, 1) }, session.ReasonExit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			c.setup(w)
			w.resume = w.skipOrRetry
			cfg := w.config()
			cfg.Limits.Apple = 300 * time.Millisecond
			if err := w.walkWith(cfg); err != nil {
				t.Fatal(err)
			}
			h := w.only(t)
			checkTeaches(t, h)
			if h.Step != flow.StepApple || h.Reason != c.reason || slices.Contains(h.Ways, session.WaySkip) {
				t.Errorf("handoff = %+v", h)
			}
		})
	}
}

func TestHomebrewTrouble(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(w *world)
		reason session.HandoffReason
	}{
		{"the wrong signer", func(w *world) { w.mac.Signer = "Developer ID Installer: Someone Else (ABCDE12345)" }, session.ReasonUnsigned},
		{"the download fails", func(w *world) { w.mac.On("curl", flow.Result{Exit: 22}, 1) }, session.ReasonExit},
		{"it never finishes", func(w *world) {
			w.mac.QuietInstalls = true
			w.mac.Place("/Library/Developer/CommandLineTools/usr/bin/git")
		}, session.ReasonTimeout},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			c.setup(w)
			w.resume = func(h session.Handoff) session.Way {
				w.mac.Signer = "Developer ID Installer: Homebrew Inc (927JGANW46)"
				return w.skipOrRetry(h)
			}
			cfg := w.config()
			cfg.Limits.Homebrew = 300 * time.Millisecond
			if err := w.walkWith(cfg); err != nil {
				t.Fatal(err)
			}
			h := w.only(t)
			checkTeaches(t, h)
			if h.Step != flow.StepHomebrew || h.Reason != c.reason {
				t.Errorf("handoff = %+v", h)
			}
		})
	}
}

func TestAFlowStopsWhenTheSessionEnds(t *testing.T) {
	w := newWorld(t)
	w.answers = nil
	f, err := flow.New(w.config())
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		w.store.Stop(time.Now())
	}()
	if err := f.Run(t.Context()); !errors.Is(err, session.ErrEnded) {
		t.Errorf("err = %v", err)
	}
}

func TestNewRefusesAnIncompleteConfig(t *testing.T) {
	w := newWorld(t)
	for name, cfg := range map[string]flow.Config{
		"no runner":   {Store: w.store, Probe: w.probe, HomebrewPkg: pkgURL},
		"no store":    {Runner: w.mac, Probe: w.probe, HomebrewPkg: pkgURL},
		"no package":  {Store: w.store, Runner: w.mac, Probe: w.probe},
		"no start":    {Store: w.store, Runner: w.mac, Probe: w.probe, HomebrewPkg: pkgURL, Hooks: []string{"jobs"}},
		"an odd hook": {Store: w.store, Runner: w.mac, Probe: w.probe, HomebrewPkg: pkgURL, Hooks: []string{"clt"}},
	} {
		if name == "no start" {
			cfg.Probe.Cwd = ""
		}
		if _, err := flow.New(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// An agent that shows a screen over the flow's question doesn't stop the setup:
// the flow puts its question back and waits for the person.
func TestAQuestionCoveredByAnotherScreenIsAskedAgain(t *testing.T) {
	w := newWorld(t)
	w.interject = map[asked]screen.Request{{"list", ""}: {Screen: "fast"}}
	if err := w.walk(); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if got := w.state().Current.Request.Screen; got != "done" {
		t.Errorf("ended on %s, want done", got)
	}
}
