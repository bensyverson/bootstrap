package flow_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// unanswered is what a run with no one to ask would still have to ask.
func (w *world) unanswered(given flow.Given) []screen.ID {
	w.t.Helper()
	cfg := w.config()
	cfg.Person, cfg.Given = &person{}, given
	f, err := flow.New(cfg)
	if err != nil {
		w.t.Fatal(err)
	}
	open, err := f.Unanswered()
	if err != nil {
		w.t.Fatal(err)
	}
	return open
}

func TestUnansweredListsEveryOpenQuestion(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	everything := []screen.ID{"git-ask", "github", "identity", "making", "xcode", "settings"}
	if got := w.unanswered(flow.Given{Name: "pine-bakery"}); !slices.Equal(got, everything) {
		t.Errorf("unanswered = %v, want %v", got, everything)
	}
	given := flow.Given{Name: "pine-bakery", Making: "code", Accepted: []flow.Offer{flow.OfferGit}, Declined: []flow.Offer{flow.OfferGitHub}}
	if got, want := w.unanswered(given), []screen.ID{"identity", "settings"}; !slices.Equal(got, want) {
		t.Errorf("with flags, unanswered = %v, want %v", got, want)
	}
	if got, want := w.unanswered(flow.Given{Name: "pine-bakery", GitName: "Ada", GitEmail: "ada@example.com", Making: "app"}), []screen.ID{"git-ask", "github", "xcode", "settings"}; !slices.Equal(got, want) {
		t.Errorf("with a name and email, unanswered = %v, want %v", got, want)
	}
}

func TestUnansweredListsTheToolsWhenSomeAreMissing(t *testing.T) {
	w := newWorld(t)
	got := w.unanswered(flow.Given{Name: "pine-bakery"})
	if len(got) == 0 || got[0] != "list" {
		t.Errorf("unanswered = %v, want the tool list first", got)
	}
	if got := w.unanswered(flow.Given{Name: "pine-bakery", Skip: []string{"sleepy"}}); slices.Contains(got, "list") {
		t.Errorf("--skip answers the tool list, but unanswered = %v", got)
	}
}

func TestUnansweredCountsAnswersKeptForTheProject(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery"}); err != nil {
		t.Fatal(err)
	}
	if got := w.unanswered(flow.Given{Name: "pine-bakery"}); len(got) != 0 {
		t.Errorf("after a run, unanswered = %v", got)
	}
}

func TestAYesReversesAKeptNo(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	offers := []flow.Offer{flow.OfferGit, flow.OfferGitHub, flow.OfferSettings}
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Declined: offers}); err != nil {
		t.Fatal(err)
	}
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Accepted: offers}); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	for _, step := range []session.StepID{flow.StepGit, flow.StepGitHub, flow.StepIdentity, flow.StepSettings} {
		if state.Finished[step] != session.StepDone {
			t.Errorf("%s = %q, want done", step, state.Finished[step])
		}
	}
	if _, err := os.Stat(filepath.Join(w.home, "Projects", "pine-bakery", ".git")); err != nil {
		t.Error("--git didn't turn on Git")
	}
}

func TestAYesToIdentityReversesAKeptNo(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Declined: []flow.Offer{flow.OfferIdentity}}); err != nil {
		t.Fatal(err)
	}
	if state := w.state(); state.Finished[flow.StepIdentity] != session.StepSkipped {
		t.Fatalf("identity = %q, want skipped", state.Finished[flow.StepIdentity])
	}
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", Accepted: []flow.Offer{flow.OfferIdentity}}); err != nil {
		t.Fatal(err)
	}
	if state := w.state(); state.Finished[flow.StepIdentity] != session.StepDone {
		t.Errorf("identity = %q, want done", state.Finished[flow.StepIdentity])
	}
	if g := w.mac.Globals(); g["user.name"] != "Ada Lovelace" {
		t.Errorf("globals = %v", g)
	}
}

func TestAYesAndANoToOneOfferIsRefused(t *testing.T) {
	w := newWorld(t)
	cfg := w.config()
	cfg.Person, cfg.Given = &person{}, flow.Given{Accepted: []flow.Offer{flow.OfferGit}, Declined: []flow.Offer{flow.OfferGit}}
	_, err := flow.New(cfg)
	bad, ok := errors.AsType[*flow.GivenError](err)
	if !ok || bad.Field != flow.GivenOffer || bad.Offer != flow.OfferGit {
		t.Errorf("err = %#v", err)
	}
}
