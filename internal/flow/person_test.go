package flow_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

var errNoDefault = errors.New("no default")

// person is someone at a terminal: by default they press Enter on every
// question, and a question with no default stops the flow.
type person struct {
	mu    sync.Mutex
	asked []flow.Question
	seen  []screen.ID
	// answer, when set, answers in place of pressing Enter.
	answer func(flow.Question) (screen.Answer, error)
}

func (p *person) Ask(_ context.Context, q flow.Question) (screen.Answer, error) {
	p.mu.Lock()
	p.asked = append(p.asked, q)
	answer := p.answer
	p.mu.Unlock()
	if answer != nil {
		return answer(q)
	}
	if q.Default == nil {
		return screen.Answer{}, errNoDefault
	}
	return *q.Default, nil
}

func (p *person) See(desc screen.Description) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, desc.Screen)
}

func (p *person) askedScreens() []screen.ID {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []screen.ID
	for _, q := range p.asked {
		out = append(out, q.Request.Screen)
	}
	return out
}

func (p *person) question(id screen.ID) (flow.Question, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, q := range p.asked {
		if q.Request.Screen == id {
			return q, true
		}
	}
	return flow.Question{}, false
}

// runAt runs the flow once with a person at a terminal, and no page or agent.
func (w *world) runAt(p *person, given flow.Given) error {
	w.t.Helper()
	cfg := w.config()
	cfg.Person, cfg.Given = p, given
	f, err := flow.New(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return f.Run(ctx)
}

// withTools is a Mac that already has Apple's tools, Homebrew and every tool from the tap.
func (w *world) withTools() {
	w.t.Helper()
	for _, path := range []string{"/Library/Developer/CommandLineTools/usr/bin/git", "/opt/homebrew/bin/brew",
		"/opt/homebrew/bin/agents", "/opt/homebrew/bin/job", "/opt/homebrew/bin/contextfill",
		"/opt/homebrew/bin/sleepy", "/opt/homebrew/bin/peep", "/opt/homebrew/bin/woodcase"} {
		if err := w.mac.Place(path); err != nil {
			w.t.Fatal(err)
		}
	}
}

func TestATerminalRunTakesTheDefaults(t *testing.T) {
	w := newWorld(t)
	p := &person{}
	if err := w.runAt(p, flow.Given{Name: "Pine Bakery"}); err != nil {
		t.Fatal(err)
	}
	want := []screen.ID{"list", "git-ask", "github", "identity", "making", "settings"}
	if got := p.askedScreens(); !slices.Equal(got, want) {
		t.Errorf("asked %v, want %v", got, want)
	}
	for _, q := range p.asked {
		if q.Default == nil {
			t.Errorf("%s had no default", q.Request.Screen)
		}
		if q.Screen.Screen != q.Request.Screen || len(q.Screen.Title) == 0 {
			t.Errorf("%s came without its words: %+v", q.Request.Screen, q.Screen)
		}
	}
	project := filepath.Join(w.home, "Projects", "pine-bakery")
	for _, name := range []string{".git", "AGENTS.md", "local", ".jobs.db"} {
		if _, err := os.Lstat(filepath.Join(project, name)); err != nil {
			t.Errorf("the project lacks %s", name)
		}
	}
	for _, tool := range []string{"sleepy", "peep", "woodcase"} {
		if _, err := os.Stat(filepath.Join(w.mac.Root, "opt/homebrew/bin", tool)); err != nil {
			t.Errorf("every tool is kept by default, but %s wasn't installed", tool)
		}
	}
	if hooks := w.mac.Hooks(); !slices.Equal(hooks, []string{"contextfill", "job"}) {
		t.Errorf("settings are on by default, but hooks = %v", hooks)
	}
	if g := w.mac.Globals(); g["user.name"] != "Ada Lovelace" {
		t.Errorf("the identity comes from GitHub by default: %v", g)
	}
	for _, id := range []screen.ID{"apple-ask", "brew-ask", "github-code", "done"} {
		if !slices.Contains(p.seen, id) {
			t.Errorf("the person never saw %s: %v", id, p.seen)
		}
	}
	state := w.state()
	for _, a := range state.Answers {
		if a.Request.Screen == "making" && a.Answer.Choice != "other" {
			t.Errorf("an empty folder is making %q, want other", a.Answer.Choice)
		}
		if a.Request.Screen == "phone" {
			t.Errorf("the phone question is the page's, but it was answered %+v", a.Answer)
		}
	}
	for _, s := range state.Handoffs {
		if s.Handoff.Kind == session.HandoffPhone {
			t.Errorf("a terminal run handed off to an agent: %+v", s.Handoff)
		}
	}
	if !strings.Contains(notes(state), "from your phone") {
		t.Errorf("the phone question becomes a hint, but the notes are:\n%s", notes(state))
	}
	if state.Finished[flow.StepPhone] != session.StepDone {
		t.Errorf("phone = %q", state.Finished[flow.StepPhone])
	}
	if slices.Contains(p.seen, "chat") || slices.Contains(p.seen, "welcome") {
		t.Errorf("seen = %v", p.seen)
	}
}

func TestATerminalRunMakesWhatTheFilesSay(t *testing.T) {
	for _, tc := range []struct {
		file, want, modules string
	}{
		{"package.json", "website", "agents add web"},
		{"Package.swift", "app", "agents add swift swiftui"},
		{"README.md", "other", ""},
	} {
		t.Run(tc.file, func(t *testing.T) {
			w := newWorld(t)
			w.withTools()
			start := filepath.Join(w.home, "code", "bakery")
			put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
			put(t, filepath.Join(start, tc.file), "{}\n")
			w.probe.Cwd = start
			p := &person{}
			if err := w.runAt(p, flow.Given{Declined: []flow.Offer{flow.OfferXcode}}); err != nil {
				t.Fatal(err)
			}
			q, ok := p.question("making")
			if !ok || q.Default == nil || q.Default.Choice != tc.want {
				t.Fatalf("making's default = %+v, want %s", q.Default, tc.want)
			}
			var adds []string
			for _, c := range w.mac.Changes() {
				if strings.HasPrefix(c, "agents add") {
					adds = append(adds, c)
				}
			}
			if tc.modules == "" && len(adds) != 0 || tc.modules != "" && !slices.Equal(adds, []string{tc.modules}) {
				t.Errorf("ran %q, want %q", adds, tc.modules)
			}
		})
	}
}

func TestATerminalRunAgainAsksNothingAndChangesNothing(t *testing.T) {
	w := newWorld(t)
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	before := len(w.mac.Changes())
	project := filepath.Join(w.home, "Projects", "pine-bakery")
	files := snapshot(t, project)
	again := &person{answer: func(q flow.Question) (screen.Answer, error) {
		return screen.Answer{}, errors.New("asked " + string(q.Request.Screen))
	}}
	if err := w.runAt(again, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	if changes := w.mac.Changes()[before:]; len(changes) != 0 {
		t.Errorf("a second run changed the Mac: %q", changes)
	}
	if got := snapshot(t, project); !mapsEqual(got, files) {
		t.Errorf("a second run changed the project")
	}
	if len(again.asked) != 0 {
		t.Errorf("a second run asked %v", again.askedScreens())
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestATerminalRunStopsAtTroubleAndARunAgainPicksUp(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := os.Remove(filepath.Join(w.mac.Root, "opt/homebrew/bin/sleepy")); err != nil {
		t.Fatal(err)
	}
	w.mac.On("install bensyverson/tap/sleepy", flow.Result{Exit: 1, Output: []byte("Error: sleepy: no bottle available")}, 1)
	p := &person{}
	err := w.runAt(p, flow.Given{Name: "pine-bakery"})
	handed, ok := errors.AsType[*flow.HandedOff](err)
	if !ok {
		t.Fatalf("err = %v, want a handoff", err)
	}
	if h := handed.Handoff; h.Tool != "sleepy" || h.Reason != session.ReasonExit || !slices.Contains(h.Ways, session.WaySkip) || len(h.Evidence.Tail) == 0 {
		t.Errorf("handoff = %+v", h)
	}
	if slices.Contains(p.seen, "chat") {
		t.Errorf("a terminal run showed the chat screen: %v", p.seen)
	}
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.mac.Root, "opt/homebrew/bin/sleepy")); err != nil {
		t.Error("the run again didn't retry sleepy")
	}
	if state := w.state(); state.Finished[flow.StepDone] != session.StepDone || state.Open != nil {
		t.Errorf("finished = %v, open = %+v", state.Finished, state.Open)
	}
}

// After trouble with a tool, running again with --skip goes on without it: a flag given now wins over an earlier answer.
func TestGivenAnswersWinOverEarlierOnes(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := os.Remove(filepath.Join(w.mac.Root, "opt/homebrew/bin/sleepy")); err != nil {
		t.Fatal(err)
	}
	w.mac.On("install bensyverson/tap/sleepy", flow.Result{Exit: 1, Output: []byte("Error: no bottle")}, 0)
	if _, ok := errors.AsType[*flow.HandedOff](w.runAt(&person{}, flow.Given{Name: "pine-bakery"})); !ok {
		t.Fatal("no handoff")
	}
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Skip: []string{"sleepy"}, Making: "app", Declined: []flow.Offer{flow.OfferXcode}}); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	if slices.Contains(state.Chosen, "sleepy") || !slices.Contains(state.Chosen, "peep") {
		t.Errorf("chosen = %v", state.Chosen)
	}
	if !slices.Contains(w.mac.Changes(), "agents add swift swiftui stage-build") {
		t.Errorf("--making app didn't reach the rules: %q", w.mac.Changes())
	}
	if slices.Contains(w.mac.Changes(), "open "+"macappstore://apps.apple.com/app/xcode/id497799835") {
		t.Error("--no-xcode opened the App Store")
	}
}

func TestDeclinedOffersAreSkipped(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	p := &person{}
	given := flow.Given{Name: "pine-bakery", Declined: []flow.Offer{flow.OfferGit, flow.OfferGitHub, flow.OfferSettings}}
	if err := w.runAt(p, given); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	for _, step := range []session.StepID{flow.StepGit, flow.StepGitHub, flow.StepIdentity, flow.StepSettings} {
		if state.Finished[step] != session.StepSkipped {
			t.Errorf("%s = %q, want skipped", step, state.Finished[step])
		}
	}
	for _, id := range []screen.ID{"git-ask", "github", "settings"} {
		if slices.Contains(p.askedScreens(), id) {
			t.Errorf("asked %s, which a flag answered", id)
		}
	}
}

func TestAQuestionWithNoDefaultStopsTheRun(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Declined: []flow.Offer{flow.OfferGitHub}})
	if !errors.Is(err, errNoDefault) {
		t.Fatalf("err = %v", err)
	}
	q := flowQuestion(t, w)
	if q != "identity-name" {
		t.Errorf("stopped on %s", q)
	}
	// With the name and email given, it goes on.
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", GitName: "Ada", GitEmail: "ada@example.com"}); err != nil {
		t.Fatal(err)
	}
	if g := w.mac.Globals(); g["user.name"] != "Ada" || g["user.email"] != "ada@example.com" {
		t.Errorf("globals = %v", g)
	}
}

// flowQuestion is the screen the log last showed.
func flowQuestion(t *testing.T, w *world) screen.ID {
	t.Helper()
	state := w.state()
	if state.Current == nil {
		return ""
	}
	return state.Current.Request.Screen
}

func TestARefusedAnswerIsAskedAgain(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	var tries []error
	p := &person{answer: func(q flow.Question) (screen.Answer, error) {
		switch q.Request.Screen {
		case "identity-name":
			return screen.Answer{Choice: "next", Name: "Ada"}, nil
		case "identity-email":
			tries = append(tries, q.Refused)
			if q.Refused == nil {
				return screen.Answer{Choice: "next", Name: "not an email"}, nil
			}
			return screen.Answer{Choice: "next", Name: "ada@example.com"}, nil
		}
		return *q.Default, nil
	}}
	if err := w.runAt(p, flow.Given{Name: "pine-bakery", Declined: []flow.Offer{flow.OfferGitHub}}); err != nil {
		t.Fatal(err)
	}
	if len(tries) != 2 || tries[1] == nil {
		t.Errorf("tries = %v", tries)
	}
}

func TestGivenAnswersAreChecked(t *testing.T) {
	for _, tc := range []struct {
		name  string
		given flow.Given
		field flow.GivenField
		valid []string
	}{
		{"an unknown making", flow.Given{Making: "game"}, flow.GivenMaking, []string{"website", "app", "code", "other"}},
		{"an unknown tool", flow.Given{Skip: []string{"nope"}}, flow.GivenSkip, []string{"sleepy", "peep", "woodcase"}},
		{"a required tool", flow.Given{Skip: []string{"jobs"}}, flow.GivenSkip, []string{"sleepy", "peep", "woodcase"}},
		{"a bad name", flow.Given{Name: "../up"}, flow.GivenName, nil},
		{"a bad email", flow.Given{GitEmail: "nope"}, flow.GivenGitEmail, nil},
		{"a bad git name", flow.Given{GitName: "-x"}, flow.GivenGitName, nil},
		{"a folder with a space", flow.Given{Name: "pine-bakery", In: "~/My Projects"}, flow.GivenIn, nil},
		{"a relative folder", flow.Given{Name: "pine-bakery", In: "code"}, flow.GivenIn, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			cfg := w.config()
			cfg.Person, cfg.Given = &person{}, tc.given
			_, err := flow.New(cfg)
			bad, ok := errors.AsType[*flow.GivenError](err)
			if !ok {
				t.Fatalf("err = %v", err)
			}
			if bad.Field != tc.field || !slices.Equal(bad.Valid, tc.valid) || bad.Message == "" {
				t.Errorf("err = %+v", bad)
			}
		})
	}
}

func TestWhere(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "code", "bakery")
	put(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	made := filepath.Join(home, "bakery")
	generic := filepath.Join(home, "Claude")
	for _, dir := range []string{made, generic} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, cwd, given, in, want string
		inPlace                    bool
		err                        error
	}{
		{"a project is set up in place", repo, "", "", repo, true, nil},
		{"a name makes a folder in Projects", home, "Pine Bakery", "", filepath.Join(home, "Projects", "pine-bakery"), false, nil},
		{"a folder under home", home, "pine-bakery", "~/git", filepath.Join(home, "git", "pine-bakery"), false, nil},
		{"a folder anywhere", home, "pine-bakery", "/Volumes/work", "/Volumes/work/pine-bakery", false, nil},
		{"a folder is ignored in place", repo, "", "~/git", repo, true, nil},
		{"no name outside a project", home, "", "", "", false, flow.ErrNeedsName},
		{"a name inside a project", repo, "other", "", "", false, flow.ErrInProject},
		// Ben, 2026-11-06 (FaXHnx): "bootstrap this" in a folder they made.
		{"no name in a folder they made", made, "", "", made, true, nil},
		{"a name in a folder they made", made, "pine-bakery", "", filepath.Join(home, "Projects", "pine-bakery"), false, nil},
		{"no name in a generic folder", generic, "", "", "", false, flow.ErrNeedsName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, inPlace, err := flow.Where(detect.Probe{Home: home, Cwd: tc.cwd}, tc.given, tc.in)
			if !errors.Is(err, tc.err) || got != tc.want || inPlace != tc.inPlace {
				t.Errorf("Where = %q, %v, %v; want %q, %v, %v", got, inPlace, err, tc.want, tc.inPlace, tc.err)
			}
		})
	}
}
