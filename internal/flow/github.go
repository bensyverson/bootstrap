package flow

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// signupURL is GitHub's page for a new account, opened for a person who has none.
const signupURL = "https://github.com/signup"

var (
	askGitHub  = screen.Request{Screen: "github"}
	signup     = screen.Request{Screen: "github-signup"}
	typedName  = screen.Request{Screen: "identity-name"}
	typedEmail = screen.Request{Screen: "identity-email"}
)

// Answers the GitHub and identity steps read.
const (
	makeOne  = "make-one"
	useThese = "use-these"
	skip     = "skip"
)

// loginCode finds the one-time code in gh's sign-in, which with no terminal
// prints `! One-time code (D7BA-CFE2) copied to clipboard` (project/2026-11-05-vm-run-findings.md).
var loginCode = regexp.MustCompile(`One-time code \(([A-Z0-9]{4}-[A-Z0-9]{4})\)`)

// signedIn reports whether gh is signed in to GitHub.
func (r *run) signedIn() (bool, *session.Handoff, error) {
	if _, ok := r.locate(ghPath); !ok {
		return false, nil, nil
	}
	res, h, err := r.exec(Command{Kind: Inspect, Path: ghPath, Args: []string{"auth", "status"}}, "", 0, 1)
	if err != nil || h != nil {
		return false, h, err
	}
	return res.Exit == 0, nil, nil
}

func checkGitHub(r *run) (session.StepStatus, *session.Handoff, error) {
	in, h, err := r.signedIn()
	if err != nil || h != nil {
		return "", h, err
	}
	if in {
		return session.StepDone, nil, nil
	}
	// The sign-up's "not now" counts only while the last answer was to make an account.
	a, ok, err := r.answer(askGitHub)
	if err != nil || !ok {
		return "", nil, err
	}
	no := a.Choice == notNow
	if a.Choice == makeOne {
		if no, err = r.declined(signup); err != nil {
			return "", nil, err
		}
	}
	if no {
		return session.StepSkipped, nil, nil
	}
	return "", nil, nil
}

func actGitHub(r *run) (outcome, error) {
	a, ask, err := r.asked(askGitHub)
	if err != nil || ask != nil {
		return deref(ask, err)
	}
	if a.Choice == notNow {
		return skipped()
	}
	if a.Choice == makeOne {
		made, ok, err := r.answer(signup)
		if err != nil {
			return outcome{}, err
		}
		if !ok {
			if r.unattended() {
				return r.putItOff(gitHubWaits)
			}
			// Only making the account needs the person in a browser; the rest is gh's.
			if _, h, err := r.exec(Command{Kind: Change, Path: openPath, Args: []string{signupURL}}, ""); err != nil || h != nil {
				return orHandOff(h, err)
			}
			return needs(signup)
		}
		if made.Choice == notNow {
			return skipped()
		}
	}
	// The sign-in waits on a code the person types, so nothing of it starts with no one there.
	if r.unattended() {
		return r.putItOff(gitHubWaits)
	}
	if _, ok := r.locate(ghPath); !ok {
		install := Command{Kind: Change, Path: brewPath, Args: []string{"install", "gh"}}
		if _, h, err := r.exec(install, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if _, ok := r.locate(ghPath); !ok {
			return handOff(r.trouble(session.ReasonMissing, "", fmt.Sprintf("%s exited 0, but %s isn't there", install, ghPath)))
		}
	}
	if h, err := r.login(); err != nil || h != nil {
		return orHandOff(h, err)
	}
	// gh's own exit isn't proof: the design confirms the sign-in took.
	if _, h, err := r.exec(Command{Kind: Inspect, Path: ghPath, Args: []string{"auth", "status"}}, ""); err != nil || h != nil {
		return orHandOff(h, err)
	}
	// So Git pushes with the person's GitHub sign-in, with no password of its own.
	if _, h, err := r.exec(Command{Kind: Change, Path: ghPath, Args: []string{"auth", "setup-git"}}, ""); err != nil || h != nil {
		return orHandOff(h, err)
	}
	if err := r.note("signed in to GitHub"); err != nil {
		return outcome{}, err
	}
	return done()
}

// login runs gh's web sign-in and puts its one-time code on the page as soon as
// gh prints it, since gh only exits once the person has typed it in.
func (r *run) login() (*session.Handoff, error) {
	cmd := Command{Kind: Change, Path: ghPath, Args: []string{"auth", "login", "--hostname", "github.com", "--web", "--git-protocol", "https", "--skip-ssh-key"}, Timeout: r.f.cfg.Limits.Login}
	var shown error
	seen := false
	_, h, err := r.stream(cmd, "", func(line string) {
		m := loginCode.FindStringSubmatch(line)
		if m == nil || seen {
			return
		}
		seen = true
		if shown = r.show(screen.Request{Screen: "github-code", Code: m[1]}); shown == nil {
			shown = r.note("signing in to GitHub: the person is entering gh's code at github.com/login/device")
		}
	})
	if err == nil {
		err = shown
	}
	return h, err
}

// ghUser is the part of `gh api user` the identity step reads.
type ghUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Name  string `json:"name"`
}

// identityKeys are Git's settings that sign a person's work, in the order they're set.
var identityKeys = []string{"user.name", "user.email"}

// unsetIdentity is Git and whichever of its identity settings are unset.
func (r *run) unsetIdentity() (string, []string, *session.Handoff, error) {
	git, h := r.git()
	if h != nil {
		return "", nil, h, nil
	}
	var unset []string
	for _, key := range identityKeys {
		value, h, err := r.globalGit(git, key)
		if err != nil || h != nil {
			return "", nil, h, err
		}
		if value == "" {
			unset = append(unset, key)
		}
	}
	return git, unset, nil, nil
}

// skippedIdentity reports whether the person's last word on who signs their
// work was a skip. Taking the offer now, or answering any identity question
// since, sets an earlier skip aside.
func (r *run) skippedIdentity() (bool, error) {
	if r.f.accepted(OfferIdentity) {
		return false, nil
	}
	state, err := r.state()
	if err != nil {
		return false, err
	}
	for _, a := range slices.Backward(state.Answers) {
		if isIdentity(a.Request.Screen) {
			return a.Answer.Choice == skip, nil
		}
	}
	return false, nil
}

func checkIdentity(r *run) (session.StepStatus, *session.Handoff, error) {
	// Git's name and email are the Mac's, so just the tools still sets them.
	tools, err := r.justTools()
	if err != nil {
		return "", nil, err
	}
	if !tools {
		dir, err := r.mustProject()
		if err != nil {
			return "", nil, err
		}
		// With no repo, there's nothing for a name to sign yet.
		if !exists(filepath.Join(dir, ".git")) {
			return session.StepSkipped, nil, nil
		}
	}
	_, unset, h, err := r.unsetIdentity()
	if err != nil || h != nil {
		return "", h, err
	}
	if len(unset) == 0 {
		return session.StepDone, nil, nil
	}
	if no, err := r.skippedIdentity(); no || err != nil {
		return session.StepSkipped, nil, err
	}
	return "", nil, nil
}

func actIdentity(r *run) (outcome, error) {
	git, unset, h, err := r.unsetIdentity()
	if err != nil || h != nil {
		return orHandOff(h, err)
	}
	in, h, err := r.signedIn()
	if err != nil || h != nil {
		return orHandOff(h, err)
	}
	give := map[string]string{}
	if in {
		offer, h, err := r.gitHubIdentity()
		if err != nil || h != nil {
			return orHandOff(h, err)
		}
		// Taking the offer up front (--identity) is the yes to GitHub's name, so there's nothing to ask.
		if !r.f.accepted(OfferIdentity) {
			a, ask, err := r.identityAsked(offer)
			if err != nil || ask != nil {
				return deref(ask, err)
			}
			if a.Choice == skip {
				return skipped()
			}
		}
		give["user.name"], give["user.email"] = offer.Name, offer.Email
	} else {
		for _, q := range []struct {
			key string
			req screen.Request
		}{{"user.name", typedName}, {"user.email", typedEmail}} {
			if !slices.Contains(unset, q.key) {
				continue
			}
			a, ask, err := r.identityAsked(q.req)
			if err != nil || ask != nil {
				return deref(ask, err)
			}
			if a.Choice == skip {
				return skipped()
			}
			give[q.key] = strings.TrimSpace(a.Name)
		}
	}
	// Only what's unset: a name or email the person already gave Git stays.
	for _, key := range unset {
		if h, err := r.setGlobalGit(git, key, give[key]); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if err := r.note("set Git's %s to %s", key, give[key]); err != nil {
			return outcome{}, err
		}
	}
	return done()
}

// identityAsked is asked for an identity question, where a skip from before
// this run doesn't count once the person has taken the offer again.
func (r *run) identityAsked(req screen.Request) (screen.Answer, *outcome, error) {
	a, ask, err := r.asked(req)
	if err != nil || ask != nil || a.Choice != skip || !r.f.accepted(OfferIdentity) {
		return a, ask, err
	}
	state, err := r.state()
	if err != nil {
		return a, nil, err
	}
	for _, kept := range slices.Backward(state.Answers) {
		if sameQuestion(kept.Request, req) {
			if kept.Seq > r.f.startSeq {
				return a, nil, nil
			}
			break
		}
	}
	out, _ := needs(req)
	return screen.Answer{}, &out, nil
}

// gitHubIdentity is the offer of the person's GitHub name, or login when it's
// blank, with GitHub's private noreply address.
func (r *run) gitHubIdentity() (screen.Request, *session.Handoff, error) {
	cmd := Command{Kind: Inspect, Path: ghPath, Args: []string{"api", "user"}}
	res, h, err := r.exec(cmd, "")
	if err != nil || h != nil {
		return screen.Request{}, h, err
	}
	var user ghUser
	if err := json.Unmarshal(res.Output, &user); err != nil || user.Login == "" || user.ID == 0 {
		trouble := r.trouble(session.ReasonMissing, "", fmt.Sprintf("%s exited 0, but its answer has no login and id", cmd))
		trouble.Evidence = session.Evidence{Command: cmd.String(), Log: filepath.Join(r.f.logs(), string(r.def.ID)+".log"), Tail: tail(res.Output)}
		return screen.Request{}, &trouble, nil
	}
	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = user.Login
	}
	offer := screen.Request{Screen: "identity", Name: name, Email: fmt.Sprintf("%d+%s@users.noreply.github.com", user.ID, user.Login)}
	if err := screen.Validate(offer); err != nil {
		trouble := r.trouble(session.ReasonMissing, "", fmt.Sprintf("GitHub's name %q or address %q can't go into Git as it is: %v", offer.Name, offer.Email, err))
		trouble.Evidence = session.Evidence{Command: cmd.String(), Tail: tail(res.Output)}
		return screen.Request{}, &trouble, nil
	}
	return offer, nil, nil
}
