package flow_test

import (
	"context"
	"github.com/bensyverson/bootstrap/internal/flow"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/flow/flowtest"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

const pkgURL = "https://example.invalid/Homebrew.pkg"

// asked is a screen a person can answer, by its screen and status.
type asked struct {
	Screen screen.ID
	Status string
}

// world is a pretend Mac, a session, a person who answers by script and an agent
// who resumes by script.
type world struct {
	t     *testing.T
	mac   *flowtest.Mac
	store session.Store
	probe detect.Probe
	home  string

	mu      sync.Mutex
	answers map[asked]screen.Answer
	shown   []asked
	// resume picks the agent's way forward from a handoff. Nil means no handoff is expected.
	resume   func(session.Handoff) session.Way
	handoffs []session.Handoff
	free     uint64
	running  func(string) bool
}

func defaultAnswers() map[asked]screen.Answer {
	return map[asked]screen.Answer{
		{"welcome", ""}:       {Choice: "see-plan"},
		{"list", ""}:          {Choice: "lets-go", Tools: []string{"sleepy"}},
		{"naming", ""}:        {Choice: "next", Name: "Pine Bakery"},
		{"git-ask", "repo"}:   {Choice: "yes"},
		{"git-ask", "ignore"}: {Choice: "yes"},
		{"git-ask", "commit"}: {Choice: "yes"},
		{"github", ""}:        {Choice: "yes"},
		{"making", ""}:        {Choice: "website"},
		{"live-yet", ""}:      {Choice: "not-yet"},
		{"settings", ""}:      {Choice: "yes"},
		{"phone", ""}:         {Choice: "yes"},
	}
}

func newWorld(t *testing.T) *world {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	w := &world{t: t, mac: flowtest.New(root), home: home, answers: defaultAnswers(), free: 100_000_000_000}
	w.running = func(string) bool { return false }
	w.probe = detect.Probe{
		Root: root, Home: home, Cwd: home,
		Free:    func(string) (uint64, error) { w.mu.Lock(); defer w.mu.Unlock(); return w.free, nil },
		Running: func(p string) bool { w.mu.Lock(); r := w.running; w.mu.Unlock(); return r(p) },
	}
	w.store = session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := w.store.Begin(screen.ClaudeCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *world) config() flow.Config {
	limits := flow.DefaultLimits()
	limits.Apple, limits.Homebrew = 2*time.Second, 2*time.Second
	return flow.Config{Store: w.store, Runner: w.mac, Probe: w.probe, HomebrewPkg: pkgURL, Hooks: flow.DefaultHooks(), Limits: limits, Poll: 2 * time.Millisecond}
}

// walk runs the flow to its end, playing the person and the agent.
func (w *world) walk() error {
	w.t.Helper()
	return w.walkWith(w.config())
}

func (w *world) walkWith(cfg flow.Config) error {
	w.t.Helper()
	f, err := flow.New(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- f.Run(ctx) }()
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-result:
			return err
		case <-tick.C:
			if problem := w.play(); problem != "" {
				cancel()
				<-result
				w.t.Fatal(problem)
			}
		}
	}
}

// play does whatever the person or the agent would do now, or names what went wrong.
func (w *world) play() string {
	state, err := w.store.Load()
	if err != nil {
		return err.Error()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if state.Open != nil {
		h := state.Open.Handoff
		if w.resume == nil {
			return "an unexpected handoff: " + h.What
		}
		w.handoffs = append(w.handoffs, h)
		if _, err := w.store.TakeHandoff(time.Now()); err != nil {
			return err.Error()
		}
		if _, err := w.store.Resume(w.resume(h), time.Now()); err != nil {
			return err.Error()
		}
		return ""
	}
	if state.Current == nil || !screen.AwaitsClick(state.Current.Request.Screen) {
		return ""
	}
	if _, done := state.CurrentAnswer(); done {
		return ""
	}
	key := asked{state.Current.Request.Screen, state.Current.Request.Status}
	answer, ok := w.answers[key]
	if !ok {
		return "the person was asked something unexpected: " + string(key.Screen) + " " + key.Status
	}
	w.shown = append(w.shown, key)
	if _, err := w.store.RecordAnswer(state.Current.Seq, answer, time.Now()); err != nil {
		return err.Error()
	}
	return ""
}

func (w *world) asked() []asked {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]asked(nil), w.shown...)
}

func (w *world) state() session.State {
	w.t.Helper()
	state, err := w.store.Load()
	if err != nil {
		w.t.Fatal(err)
	}
	return state
}

// snapshot is every file under a folder with its content, or link target, so a test can see what moved.
func snapshot(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case d.Type()&fs.ModeSymlink != 0:
				target, _ := os.Readlink(path)
				out[path] = "-> " + target
			case d.IsDir():
				out[path] = "/"
			default:
				body, _ := os.ReadFile(path)
				out[path] = string(body)
			}
			return nil
		})
	}
	return out
}

func notes(state session.State) string {
	var lines []string
	for _, n := range state.Notes {
		lines = append(lines, n.Text)
	}
	return strings.Join(lines, "\n")
}

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
