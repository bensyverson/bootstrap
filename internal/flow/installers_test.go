package flow_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/flow/flowtest"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The code has to reach the page while gh is still waiting for it, since gh only
// exits once the person has typed it in.
func TestTheCodeIsOnThePageWhileGhWaits(t *testing.T) {
	w := newWorld(t)
	var onPage *screen.Request
	w.mac.LoginWait = func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			state, err := w.store.Load()
			if err == nil && state.Current != nil && state.Current.Request.Screen == "github-code" {
				req := state.Current.Request
				onPage = &req
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if onPage == nil {
		t.Fatal("the code never reached the page while gh waited")
	}
	if onPage.Code != flowtest.LoginCode {
		t.Errorf("the page shows code %q, want %q", onPage.Code, flowtest.LoginCode)
	}
}

func TestNoAccountOpensGitHubsSignUp(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "make-one"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	changes := strings.Join(w.mac.Changes(), "\n")
	signup := strings.Index(changes, "open https://github.com/signup")
	login := strings.Index(changes, "gh auth login")
	if signup < 0 || login < signup {
		t.Errorf("want the sign-up page opened before gh signs in:\n%s", changes)
	}
	if !slices.Contains(w.asked(), asked{"github-signup", ""}) {
		t.Error("never asked whether the account was made")
	}
}

func TestNotNowOnSignUpSkipsGitHub(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "make-one"}
	w.answers[asked{"github-signup", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(w.mac.Changes(), "\n"), "gh auth login") {
		t.Error("signed in after the person said not now")
	}
	if got := w.state().Finished[flow.StepGitHub]; got != session.StepSkipped {
		t.Errorf("github finished %q", got)
	}
}

func TestAHaveAccountAnswerOpensNoSignUp(t *testing.T) {
	w := newWorld(t)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(w.mac.Changes(), "\n"), "github.com/signup") {
		t.Error("opened the sign-up page for someone with an account")
	}
}

func TestABlankGitHubNameUsesTheLogin(t *testing.T) {
	w := newWorld(t)
	w.mac.GitHubUser = flowtest.GitHubUser{Login: "ada", ID: 1234}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.mac.Globals()["user.name"]; got != "ada" {
		t.Errorf("user.name = %q", got)
	}
}

func TestWithoutGitHubThePersonTypesTheirName(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "not-now"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	globals := w.mac.Globals()
	if globals["user.name"] != "ada" || globals["user.email"] != "ada@example.com" {
		t.Errorf("git's settings = %v", globals)
	}
	if slices.Contains(w.asked(), asked{"identity", ""}) {
		t.Error("offered GitHub's details without a sign-in")
	}
}

// A typed name keeps its capitals and spaces: only the folder field makes a folder name.
func TestATypedNameIsKeptAsTyped(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"github", ""}] = screen.Answer{Choice: "not-now"}
	w.answers[asked{"identity-name", ""}] = screen.Answer{Choice: "next", Name: "Ada Lovelace"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.mac.Globals()["user.name"]; got != "Ada Lovelace" {
		t.Errorf("user.name = %q: a typed name was made into a folder name", got)
	}
}

func TestAnIdentityGitHasIsLeftAlone(t *testing.T) {
	w := newWorld(t)
	w.mac.SetGlobal("user.name", "Grace Hopper")
	w.mac.SetGlobal("user.email", "grace@example.com")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(w.mac.Changes(), "\n"), "git config --global user.") {
		t.Errorf("changed Git's identity:\n%s", strings.Join(w.mac.Changes(), "\n"))
	}
	for _, a := range w.asked() {
		if strings.HasPrefix(string(a.Screen), "identity") {
			t.Errorf("asked %v though Git has a name and email", a)
		}
	}
}

func TestOnlyTheMissingHalfIsSet(t *testing.T) {
	w := newWorld(t)
	w.mac.SetGlobal("user.name", "Grace Hopper")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	globals := w.mac.Globals()
	if globals["user.name"] != "Grace Hopper" || globals["user.email"] != "1234+ada@users.noreply.github.com" {
		t.Errorf("git's settings = %v", globals)
	}
}

func TestSkippingTheIdentitySetsNothing(t *testing.T) {
	w := newWorld(t)
	w.answers[asked{"identity", ""}] = screen.Answer{Choice: "skip"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	globals := w.mac.Globals()
	if _, ok := globals["user.name"]; ok {
		t.Errorf("git's settings = %v", globals)
	}
	if got := w.state().Finished[flow.StepIdentity]; got != session.StepSkipped {
		t.Errorf("identity finished %q", got)
	}
	// A second walk doesn't ask again.
	asks := len(w.asked())
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if again := w.asked()[asks:]; len(again) > 0 {
		t.Errorf("asked again: %v", again)
	}
}

func TestABranchNameThePersonChoseIsKept(t *testing.T) {
	w := newWorld(t)
	w.mac.SetGlobal("init.defaultBranch", "trunk")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if got := w.mac.Globals()["init.defaultBranch"]; got != "trunk" {
		t.Errorf("init.defaultBranch = %q", got)
	}
}

func TestGitHubsDetailsThatCantBeReadHandOff(t *testing.T) {
	w := newWorld(t)
	w.mac.On("gh api user", flow.Result{Output: []byte("not json")}, 0)
	w.resume = w.skipOrRetry
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	h := w.only(t)
	checkTeaches(t, h)
	if h.Step != flow.StepIdentity || !slices.Contains(h.Ways, session.WaySkip) {
		t.Errorf("handoff = %+v", h)
	}
}

// Criterion GUc: the Homebrew step opens only a package Homebrew signed and Apple notarized.
func TestHomebrewRefusesAPackageItCantTrust(t *testing.T) {
	cases := []struct {
		name  string
		setup func(m *flowtest.Mac)
	}{
		{"the wrong signer", func(m *flowtest.Mac) { m.Signer = "Developer ID Installer: Someone Else (ABCDE12345)" }},
		{"Homebrew's team further down the chain", func(m *flowtest.Mac) {
			m.Signer = "Developer ID Installer: Someone Else (ABCDE12345)"
			m.Chain = []string{"Developer ID Installer: Homebrew Inc (927JGANW46)"}
		}},
		{"a signer that only mentions the team", func(m *flowtest.Mac) { m.Signer = "Developer ID Application: Not Homebrew (927JGANW46)" }},
		{"not notarized", func(m *flowtest.Mac) { m.Notarized = false }},
		{"no signature", func(m *flowtest.Mac) { m.Unsigned = true }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			c.setup(w.mac)
			var openedFirst []string
			w.resume = func(h session.Handoff) session.Way {
				for _, ch := range w.mac.Changes() {
					if strings.HasPrefix(ch, "open ") && strings.HasSuffix(ch, "Homebrew.pkg") {
						openedFirst = append(openedFirst, ch)
					}
				}
				w.mac.Signer, w.mac.Chain, w.mac.Notarized, w.mac.Unsigned = flowtest.HomebrewSigner, nil, true, false
				return w.skipOrRetry(h)
			}
			if err := w.walk(); err != nil {
				t.Fatal(err)
			}
			h := w.only(t)
			checkTeaches(t, h)
			if h.Step != flow.StepHomebrew || h.Reason != session.ReasonUnsigned || slices.Contains(h.Ways, session.WaySkip) {
				t.Errorf("handoff = %+v", h)
			}
			if len(openedFirst) > 0 {
				t.Errorf("opened a package it refused: %v", openedFirst)
			}
		})
	}
}
