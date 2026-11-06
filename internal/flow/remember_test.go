package flow_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/prefs"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Later setups remember the person's answers (Ben, 2026-11-06, XbdzWw): a
// remembered answer is the next setup's default, a flag given up front wins and
// becomes the remembered one, and --yes never starts a step that waits on a person.

// remembering gives the world a preferences file, with what's in it to begin with.
func (w *world) remembering(change func(*prefs.Preferences)) {
	w.t.Helper()
	w.prefs = prefs.File{Dir: w.t.TempDir()}
	if err := w.prefs.Update(change); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) preferences() prefs.Preferences {
	w.t.Helper()
	p, err := w.prefs.Load()
	if err != nil {
		w.t.Fatal(err)
	}
	return p
}

// runFlow runs a flow with a person at a terminal, and returns it for what it reports afterward.
func (w *world) runFlow(cfg flow.Config) (*flow.Flow, error) {
	w.t.Helper()
	f, err := flow.New(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return f, f.Run(ctx)
}

func TestThePagesAnswersAreRemembered(t *testing.T) {
	w := newWorld(t)
	w.remembering(func(*prefs.Preferences) {})
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "not-now"}
	w.answers[asked{"settings", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	got := w.preferences()
	for name, want := range map[prefs.Name]string{
		prefs.Git: "yes", prefs.GitHub: "not-now", prefs.Identity: "use-these",
		prefs.Making: "website", prefs.Settings: "not-now", prefs.Phone: "yes",
	} {
		if choice, _ := got.Answer(name); choice != want {
			t.Errorf("remembered %s = %q, want %q", name, choice, want)
		}
	}
	if !slices.Equal(got.SkipTools, []string{"peep", "woodcase"}) {
		t.Errorf("remembered skip_tools = %v, want the two the person left off", got.SkipTools)
	}
}

func TestAMadeAccountIsRememberedAsHavingOne(t *testing.T) {
	w := newWorld(t)
	w.remembering(func(*prefs.Preferences) {})
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "make-one"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if choice, _ := w.preferences().Answer(prefs.GitHub); choice != "yes" {
		t.Errorf("remembered github = %q, want yes: the account is made, so next time there's no making one", choice)
	}
}

func TestTheRememberedChoiceIsThePagesDefault(t *testing.T) {
	w := newWorld(t)
	w.remembering(func(p *prefs.Preferences) { p.Remember(prefs.GitHub, "not-now") })
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(w.state().Answers, func(a session.Answered) bool { return a.Request.Screen == "github" })
	if i < 0 {
		t.Fatal("github was never answered")
	}
	if got := w.state().Answers[i].Request.Default; got != "not-now" {
		t.Errorf("github was shown with default %q, want not-now", got)
	}
	if choice, _ := w.preferences().Answer(prefs.GitHub); choice != "yes" {
		t.Errorf("remembered github = %q, want the person's new yes", choice)
	}
}

func TestARememberedDeclineIsTheDefaultAtATerminal(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	// With a tool missing, the list has something to ask.
	if err := os.Remove(filepath.Join(w.mac.Root, "opt/homebrew/bin/woodcase")); err != nil {
		t.Fatal(err)
	}
	w.remembering(func(p *prefs.Preferences) {
		p.Remember(prefs.GitHub, "not-now")
		p.Remember(prefs.Identity, "skip")
		p.Remember(prefs.Settings, "not-now")
		p.SkipTools = []string{"woodcase"}
	})
	p := &person{}
	if err := w.runAt(p, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[screen.ID]string{"github": "not-now", "identity-name": "skip", "settings": "not-now"} {
		q, ok := p.question(id)
		if !ok || q.Default == nil || q.Default.Choice != want {
			t.Errorf("%s's default = %+v, want %s", id, q.Default, want)
		}
	}
	if q, ok := p.question("list"); !ok || q.Default == nil || slices.Contains(q.Default.Tools, "woodcase") || !slices.Contains(q.Default.Tools, "peep") {
		t.Errorf("the list's default = %+v, want every tool but woodcase", q.Default)
	}
	for _, c := range w.mac.Changes() {
		if strings.HasPrefix(c, "gh auth login") || c == "brew install gh" || strings.Contains(c, "hook install") || strings.Contains(c, "install bensyverson/tap/woodcase") {
			t.Errorf("ran %q, which the remembered answers declined", c)
		}
	}
	if got := w.state().Finished[flow.StepGitHub]; got != session.StepSkipped {
		t.Errorf("github = %q, want skipped", got)
	}
}

func TestAFlagOverridesARememberedAnswerAndBecomesIt(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.mac.SetGlobal("user.name", "Ada")
	w.mac.SetGlobal("user.email", "ada@example.com")
	w.remembering(func(p *prefs.Preferences) {
		p.Remember(prefs.Settings, "not-now")
		p.Remember(prefs.GitHub, "yes")
	})
	given := flow.Given{Name: "pine-bakery", Accepted: []flow.Offer{flow.OfferSettings}, Declined: []flow.Offer{flow.OfferGitHub}}
	if err := w.runAt(&person{}, given); err != nil {
		t.Fatal(err)
	}
	if hooks := w.mac.Hooks(); len(hooks) == 0 {
		t.Error("--settings took back the remembered no, but no hook was installed")
	}
	got := w.preferences()
	for name, want := range map[prefs.Name]string{prefs.Settings: "yes", prefs.GitHub: "not-now"} {
		if choice, _ := got.Answer(name); choice != want {
			t.Errorf("remembered %s = %q, want the flag's %q", name, choice, want)
		}
	}
}

func TestTakingTheIdentityOfferForgetsAnEarlierSkip(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.remembering(func(p *prefs.Preferences) { p.Remember(prefs.Identity, "skip") })
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Accepted: []flow.Offer{flow.OfferIdentity}, Declined: []flow.Offer{flow.OfferGitHub}, GitName: "Ada", GitEmail: "ada@example.com"}); err != nil {
		t.Fatal(err)
	}
	if g := w.mac.Globals(); g["user.name"] != "Ada" {
		t.Errorf("--identity took back the remembered skip, but Git's name = %q", g["user.name"])
	}
	if choice, _ := w.preferences().Answer(prefs.Identity); choice == "skip" {
		t.Error("--identity was given, but the skip is still remembered")
	}
}

func TestARememberedMakingIsTheDefaultOnlyWhenTheFilesSayNothing(t *testing.T) {
	for _, tc := range []struct {
		name, file, want string
	}{
		{"no files", "", "app"},
		{"a website's files", "package.json", "website"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			w.withTools()
			w.remembering(func(p *prefs.Preferences) { p.Remember(prefs.Making, "app") })
			start := filepath.Join(w.home, "code", "bakery")
			put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
			if tc.file != "" {
				put(t, filepath.Join(start, tc.file), "{}\n")
			}
			w.probe.Cwd = start
			p := &person{}
			if err := w.runAt(p, flow.Given{Declined: []flow.Offer{flow.OfferXcode}}); err != nil {
				t.Fatal(err)
			}
			if q, ok := p.question("making"); !ok || q.Default == nil || q.Default.Choice != tc.want {
				t.Errorf("making's default = %+v, want %s", q.Default, tc.want)
			}
		})
	}
}

func TestAStaleRememberedAnswerIsIgnored(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.remembering(func(p *prefs.Preferences) { p.Remember(prefs.Settings, "maybe") })
	p := &person{}
	if err := w.runAt(p, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	if q, ok := p.question("settings"); !ok || q.Default == nil || q.Default.Choice != "yes" {
		t.Errorf("settings' default = %+v, want the setup's own yes", q.Default)
	}
}

func TestUnattendedSkipsGitHubsSignInAndSaysSo(t *testing.T) {
	for _, github := range []string{"yes", "make-one"} {
		t.Run(github, func(t *testing.T) {
			w := newWorld(t)
			w.withTools()
			w.remembering(func(p *prefs.Preferences) { p.Remember(prefs.GitHub, github) })
			cfg := w.config()
			cfg.Person, cfg.Attendance = &person{}, flow.Unattended
			cfg.Given = flow.Given{Name: "pine-bakery", GitName: "Ada", GitEmail: "ada@example.com"}
			f, err := w.runFlow(cfg)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range w.mac.Changes() {
				if strings.HasPrefix(c, "gh auth login") || strings.Contains(c, "github.com/signup") {
					t.Errorf("ran %q, which waits on a person, with no one there", c)
				}
			}
			if got := w.state().Finished[flow.StepGitHub]; got != session.StepSkipped {
				t.Errorf("github = %q, want skipped", got)
			}
			put := f.PutOff()
			if len(put) != 1 || put[0].Step != flow.StepGitHub || put[0].What == "" {
				t.Errorf("put off %+v, want GitHub's sign-in, with why", put)
			}
			if !strings.Contains(notes(w.state()), "GitHub") {
				t.Errorf("the log doesn't say GitHub was put off:\n%s", notes(w.state()))
			}
			if choice, _ := w.preferences().Answer(prefs.GitHub); choice != github {
				t.Errorf("remembered github = %q; putting it off isn't a no", choice)
			}
		})
	}
}

func TestUnattendedStopsBeforeAnInstallerThatWaitsOnAPerson(t *testing.T) {
	for _, tc := range []struct {
		name  string
		place []string
		step  session.StepID
	}{
		{"Apple's tools", nil, flow.StepApple},
		{"Homebrew", []string{"/Library/Developer/CommandLineTools/usr/bin/git"}, flow.StepHomebrew},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			for _, path := range tc.place {
				if err := w.mac.Place(path); err != nil {
					t.Fatal(err)
				}
			}
			cfg := w.config()
			cfg.Person, cfg.Attendance, cfg.Given = &person{}, flow.Unattended, flow.Given{Name: "pine-bakery"}
			_, err := w.runFlow(cfg)
			needs, ok := errors.AsType[*flow.NeedsPerson](err)
			if !ok {
				t.Fatalf("err = %v, want NeedsPerson", err)
			}
			if needs.Step != tc.step || needs.What == "" {
				t.Errorf("needs = %+v, want %s with why", needs, tc.step)
			}
			if changes := w.mac.Changes(); len(changes) != 0 {
				t.Errorf("ran %q before stopping", changes)
			}
		})
	}
}

// The trial's bug end to end (project/2026-11-06-clean-mac-trial-findings.md,
// run 2, finding 4): not now on the page, then bootstrap new --yes on the same Mac.
func TestADeclineOnThePageStaysDeclinedInALaterRunWithYes(t *testing.T) {
	w := newWorld(t)
	w.remembering(func(*prefs.Preferences) {})
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "not-now"}
	w.answers[asked{"identity-name", ""}] = screen.Answer{Choice: "skip"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	before := len(w.mac.Changes())
	// bootstrap new keeps its own log, apart from the page's.
	w.store = session.Store{Dir: filepath.Join(t.TempDir(), "new")}
	if _, err := w.store.Begin(screen.ClaudeCode, 2, time.Now()); err != nil {
		t.Fatal(err)
	}
	cfg := w.config()
	cfg.Person, cfg.Attendance, cfg.Given = &person{}, flow.Unattended, flow.Given{Name: "second-project"}
	f, err := w.runFlow(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range w.mac.Changes()[before:] {
		if strings.HasPrefix(c, "gh ") || c == "brew install gh" {
			t.Errorf("ran %q, though the person said not now to GitHub on the page", c)
		}
	}
	if got := w.state().Finished[flow.StepGitHub]; got != session.StepSkipped {
		t.Errorf("github = %q, want skipped", got)
	}
	if len(f.PutOff()) != 0 {
		t.Errorf("put off %+v: a no isn't a step put off", f.PutOff())
	}
}
