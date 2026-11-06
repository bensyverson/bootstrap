package flow

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// With no one to ask and no word to take the defaults, bootstrap new refuses
// before it touches anything, naming each question nothing answers yet (Ben,
// 2026-11-05, OWv6U2). This is the flow's own reckoning of those questions, so
// the CLI holds no copy of which steps ask what.

// Unanswered lists, in the order a run meets them, the questions a run would
// ask that nothing answers: not Given, and not an answer kept for the project.
// It reads and changes nothing but the log. A question that comes only on some
// Macs, such as GitHub's when gh isn't signed in, is listed, since nothing has
// run to tell; one the flow answers itself, or that the project already
// settles, isn't.
func (f *Flow) Unanswered() ([]screen.ID, error) {
	state, err := f.cfg.Store.Load()
	if errors.Is(err, session.ErrNoSession) {
		state, err = session.State{}, nil
	}
	if err != nil {
		return nil, err
	}
	given, err := f.cfg.Given.answers()
	if err != nil {
		return nil, err
	}
	// last is the answer that stands for a screen: the one given now, else the one kept.
	last := func(id screen.ID) (screen.Answer, bool) {
		for _, g := range slices.Backward(given) {
			if g.req.Screen == id {
				return g.answer, true
			}
		}
		for _, a := range slices.Backward(state.Answers) {
			if a.Request.Screen == id {
				return a.Answer, true
			}
		}
		return screen.Answer{}, false
	}
	// A step that finished done for this project asks nothing again, even with no answer kept:
	// GitHub's sign-in, when gh was already signed in.
	finished := func(id screen.ID) bool {
		return slices.ContainsFunc(steps, func(def stepDef) bool {
			return slices.Contains(def.Asks, id) && state.Finished[def.ID] == session.StepDone
		})
	}
	answered := func(ids ...screen.ID) bool {
		return slices.ContainsFunc(ids, func(id screen.ID) bool { _, ok := last(id); return ok || finished(id) })
	}
	probe := f.cfg.Probe
	dir := probe.Cwd
	if !startsInPlace(probe) {
		dir, _, _ = Where(probe, f.cfg.Given.Name, f.cfg.Given.In)
	}
	hasRepo := dir != "" && exists(filepath.Join(dir, ".git"))
	named, _ := last(naming.Screen)
	justTools := !startsInPlace(probe) && named.Choice == screen.JustTools

	var open []screen.ID
	if !answered(list.Screen) && !toolsSettled(probe) {
		open = append(open, list.Screen)
	}
	if justTools {
		// With no project, only the machine-wide questions are left.
		if !answered(askGitHub.Screen) {
			open = append(open, askGitHub.Screen)
		}
		if !slices.Contains(f.cfg.Given.Accepted, OfferIdentity) && !answered("identity", typedName.Screen, typedEmail.Screen) {
			open = append(open, "identity")
		}
		if !answered(settings.Screen) {
			open = append(open, settings.Screen)
		}
		return open, nil
	}
	gitOff := false
	if !hasRepo {
		a, ok := last(askRepo.Screen)
		gitOff = ok && a.Choice == notNow
		if !ok {
			open = append(open, askRepo.Screen)
		}
	}
	if !answered(askGitHub.Screen) {
		open = append(open, askGitHub.Screen)
	}
	identityTaken := slices.Contains(f.cfg.Given.Accepted, OfferIdentity)
	if !gitOff && !identityTaken && !answered("identity", typedName.Screen, typedEmail.Screen) {
		open = append(open, "identity")
	}
	what, ok := last(making.Screen)
	if !ok {
		open = append(open, making.Screen)
	}
	if (!ok || what.Choice == anApp) && !answered(xcode.Screen) {
		open = append(open, xcode.Screen)
	}
	if !answered(settings.Screen) {
		open = append(open, settings.Screen)
	}
	return open, nil
}

// toolsSettled reports whether every tool on the list is already on this Mac,
// so the list asks nothing and the flow answers it itself.
func toolsSettled(probe detect.Probe) bool {
	return !slices.ContainsFunc(screen.AllToolIDs(), func(id string) bool {
		tool, _ := catalog.Find(id)
		_, there := detect.Locate(probe, tool.Detect)
		return !tool.OwnStep && !there
	})
}

// accepted reports whether the person took an offer up front.
func (f *Flow) accepted(offer Offer) bool { return slices.Contains(f.cfg.Given.Accepted, offer) }

// sameQuestion reports whether two requests ask the same thing. The naming
// screen's folder is only where the pick starts, so it asks the same question
// wherever it starts.
func sameQuestion(a, b screen.Request) bool {
	a.In, b.In = "", ""
	// Nor does the default it offers, which is only where the answer starts.
	a.Default, b.Default = "", ""
	return a == b
}

// isIdentity reports whether a screen asks who signs the person's work.
func isIdentity(id screen.ID) bool { return strings.HasPrefix(string(id), "identity") }
