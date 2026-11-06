package flow_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Someone may want only the tools (Ben, 2026-11-06, VGCn9C): the naming screen's
// "just the tools" skips the project steps and keeps the machine-wide ones.

var projectSteps = []session.StepID{flow.StepFolder, flow.StepGit, flow.StepStage, flow.StepRules}

func (w *world) wantNoProject(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(w.home, "Projects")); err == nil {
		t.Error("just the tools made ~/Projects")
	}
	state := w.state()
	for _, id := range projectSteps {
		if got := state.Finished[id]; got != session.StepSkipped {
			t.Errorf("%s = %q, want skipped", id, got)
		}
	}
	for _, c := range w.mac.Changes() {
		if c == "git init" || strings.HasPrefix(c, "agents ") || strings.HasPrefix(c, "job init") {
			t.Errorf("ran %q, a project step", c)
		}
	}
}

func TestJustTheToolsOnThePageSkipsTheProject(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"naming", ""}] = screen.Answer{Choice: screen.JustTools}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	w.wantNoProject(t)
	var screens []screen.ID
	for _, a := range w.asked() {
		screens = append(screens, a.Screen)
	}
	if want := []screen.ID{"welcome", "list", "naming", "github", "identity", "settings", "phone"}; !slices.Equal(screens, want) {
		t.Errorf("asked %v, want %v", screens, want)
	}
	if g := w.mac.Globals(); g["user.name"] != "Ada Lovelace" {
		t.Errorf("with no project, Git's name is still set for the Mac: %v", g)
	}
	if hooks := w.mac.Hooks(); len(hooks) == 0 {
		t.Error("the helpers are machine-wide, but none was turned on")
	}
	state := w.state()
	// The helpers are on, so done says to just ask (S2lZZq); with them off it
	// says how to run bootstrap new (selfmove_test.go, teach_test.go).
	if state.Current == nil || state.Current.Request != (screen.Request{Screen: "done", Status: screen.DoneJustToolsAsk}) {
		t.Errorf("ended on %+v, want done for just the tools, saying to just ask", state.Current)
	}
	i := slices.IndexFunc(state.Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffDone })
	if i < 0 {
		t.Fatal("no done handoff")
	}
	if what := state.Handoffs[i].Handoff.What; !strings.Contains(what, "just ask you to bootstrap it") {
		t.Errorf("the done handoff says %q, want that they can just ask", what)
	}
}

func TestJustTheToolsWithNoGitHubAsksForAName(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"naming", ""}] = screen.Answer{Choice: screen.JustTools}
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if g := w.mac.Globals(); g["user.name"] != "ada" || g["user.email"] != "ada@example.com" {
		t.Errorf("globals = %v, want the typed name and email", g)
	}
}

func TestJustTheToolsAtATerminal(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	p := &person{}
	if err := w.runAt(p, flow.Given{JustTools: true}); err != nil {
		t.Fatal(err)
	}
	w.wantNoProject(t)
	if got, want := p.askedScreens(), []screen.ID{"github", "identity", "settings"}; !slices.Equal(got, want) {
		t.Errorf("asked %v, want %v", got, want)
	}
}

func TestUnansweredWithJustTheToolsLeavesOutTheProjectsQuestions(t *testing.T) {
	w := newWorld(t)
	cfg := w.config()
	cfg.Person, cfg.Given = &person{}, flow.Given{JustTools: true}
	f, err := flow.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Unanswered()
	if err != nil {
		t.Fatal(err)
	}
	if want := []screen.ID{"list", "github", "identity", "settings"}; !slices.Equal(got, want) {
		t.Errorf("unanswered = %v, want %v", got, want)
	}
}

func TestJustTheToolsAndANameSayOppositeThings(t *testing.T) {
	w := newWorld(t)
	cfg := w.config()
	cfg.Person, cfg.Given = &person{}, flow.Given{JustTools: true, Name: "pine-bakery"}
	_, err := flow.New(cfg)
	if bad, ok := errors.AsType[*flow.GivenError](err); !ok || bad.Field != flow.GivenJustTools {
		t.Errorf("err = %v, want a GivenError for just-tools", err)
	}
}
