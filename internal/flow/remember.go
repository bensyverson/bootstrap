package flow

import (
	"slices"

	"github.com/bensyverson/bootstrap/internal/prefs"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Later setups remember the person's answers (Ben, 2026-11-06, XbdzWw;
// project/2026-11-05-bootstrap-new-plan.md). Every answer the person gives, on
// the page, at a terminal or as a flag of bootstrap new, is kept in
// preferences.json, and the next setup offers it as the default. With --yes the
// default is taken, so a no on the page stays a no. A run's own log still wins
// for its project: the answers kept there are this project's.

// rememberedAs names the questions whose answers are remembered, each under one
// name. GitHub's sign-up and the typed name and email fold into GitHub's and the
// identity's answers in remember.
var rememberedAs = map[screen.ID]prefs.Name{
	askRepo.Screen:   prefs.Git,
	askGitHub.Screen: prefs.GitHub,
	"identity":       prefs.Identity,
	making.Screen:    prefs.Making,
	xcode.Screen:     prefs.Xcode,
	settings.Screen:  prefs.Settings,
	phone.Screen:     prefs.Phone,
}

// remember keeps the person's answer to a question for the next setup.
func (f *Flow) remember(req screen.Request, a screen.Answer) error {
	return f.cfg.Preferences.Update(func(p *prefs.Preferences) {
		switch id := req.Screen; {
		case id == list.Screen:
			kept := screen.KeepRequired(a.Tools)
			p.SkipTools = slices.DeleteFunc(optionalTools(), func(t string) bool { return slices.Contains(kept, t) })
		case id == signup.Screen:
			// A made account is one the person has from now on.
			choice := "yes"
			if a.Choice == notNow {
				choice = notNow
			}
			p.Remember(prefs.GitHub, choice)
		case id == typedName.Screen || id == typedEmail.Screen:
			choice := useThese
			if a.Choice == skip {
				choice = skip
			}
			p.Remember(prefs.Identity, choice)
		default:
			if name, ok := rememberedAs[id]; ok {
				p.Remember(name, a.Choice)
			}
		}
	})
}

// rememberIdentity keeps the identity offer taken up front, which has no answer
// of its own, so an earlier skip doesn't come back next time.
func (f *Flow) rememberIdentity() error {
	if !f.accepted(OfferIdentity) {
		return nil
	}
	return f.cfg.Preferences.Update(func(p *prefs.Preferences) { p.Remember(prefs.Identity, useThese) })
}

// rememberedDefault is the person's remembered answer to a question, as an
// answer the question takes, or nil when there's none. A remembered choice the
// question doesn't offer, from an older or hand-edited file, is passed over.
func (f *Flow) rememberedDefault(req screen.Request, desc screen.Description) (*screen.Answer, error) {
	p, err := f.cfg.Preferences.Load()
	if err != nil {
		return nil, err
	}
	var a screen.Answer
	switch id := req.Screen; {
	case id == list.Screen:
		if len(p.SkipTools) == 0 {
			return nil, nil
		}
		kept := slices.DeleteFunc(screen.AllToolIDs(), func(t string) bool { return slices.Contains(p.SkipTools, t) })
		a = screen.Answer{Choice: "lets-go", Tools: screen.KeepRequired(kept)}
	case id == typedName.Screen || id == typedEmail.Screen:
		// Only a skip answers a question that wants something typed.
		if choice, _ := p.Answer(prefs.Identity); choice != skip {
			return nil, nil
		}
		a = screen.Answer{Choice: skip}
	default:
		name, ok := rememberedAs[id]
		if !ok {
			return nil, nil
		}
		choice, ok := p.Answer(name)
		if !ok {
			return nil, nil
		}
		a = screen.Answer{Choice: choice}
	}
	if screen.ValidateAnswer(desc, a) != nil {
		return nil, nil
	}
	return &a, nil
}

// pageDefault is a question for the page, with the person's remembered choice as its main button.
func (r *run) pageDefault(req screen.Request) (screen.Request, error) {
	state, err := r.state()
	if err != nil {
		return req, err
	}
	desc, err := screen.Describe(req, screen.Context{Agent: state.Agent, Chosen: state.Chosen, Carried: state.Progress})
	if err != nil {
		return req, err
	}
	a, err := r.f.rememberedDefault(req, desc)
	if err != nil || a == nil || a.Choice == "" || len(a.Tools) > 0 {
		return req, err
	}
	req.Default = a.Choice
	return req, nil
}

// Attendance says whether anyone will act outside the terminal: click through
// Apple's window or Homebrew's installer, or type GitHub's code.
type Attendance int

const (
	// Attended is the page, or a person at a terminal.
	Attended Attendance = iota
	// Unattended is bootstrap new --yes: the person said to take every default
	// without asking, so nothing waits on them (Ben, 2026-11-06, XbdzWw).
	Unattended
)

// NeedsPerson is a step that waits on the person acting outside the setup, on a
// run where no one will. A required step stops the run with it; GitHub's sign-in
// is put off, and the run goes on.
type NeedsPerson struct {
	Step session.StepID
	// What is what the step waits on, as a line to pass on.
	What string
}

func (n *NeedsPerson) Error() string {
	return "the setup stopped before " + string(n.Step) + ": " + n.What
}

// waitsOnPerson are the required steps that can't finish without the person,
// and what each waits on.
var waitsOnPerson = map[session.StepID]string{
	StepApple:    "Apple's tools need a click on Install in Apple's window",
	StepHomebrew: "Homebrew's installer needs clicking through, and the Mac's password",
}

// gitHubWaits is what GitHub's sign-in waits on.
const gitHubWaits = "GitHub's sign-in waits for a code typed in at github.com"

// PutOff lists the steps this run skipped because they wait on a person and no one is there.
func (f *Flow) PutOff() []NeedsPerson { return slices.Clone(f.putOff) }

// unattended reports whether no one will act outside the terminal.
func (r *run) unattended() bool {
	return r.f.cfg.Person != nil && r.f.cfg.Attendance == Unattended
}

// putItOff skips a step that waits on a person, says so in the log, and keeps it for PutOff.
func (r *run) putItOff(what string) (outcome, error) {
	r.f.putOff = append(r.f.putOff, NeedsPerson{Step: r.def.ID, What: what})
	if err := r.note("skipped %s: %s, and no one is there to do it", r.def.ID, what); err != nil {
		return outcome{}, err
	}
	return skipped()
}
