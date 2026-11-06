package flow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The setup with no page (bootstrap new, project/2026-11-05-bootstrap-new-plan.md).
// The steps, their checks and their order are the same; only the asking differs.
// Questions go to a Person with a default ready, trouble stops the run instead of
// waiting for an agent, and the phone question, which is the page's, becomes a hint.
// Everything still goes into the session log, so a run again picks up.

// Person is how a flow reaches the person when there's no page. A flow with no
// Person asks on the page, through the log, and waits for the agent after trouble.
type Person interface {
	// Ask returns the person's answer to a question. An error stops the run.
	Ask(ctx context.Context, q Question) (screen.Answer, error)
	// See is a screen that asks nothing: where the setup has got to, or what to
	// do outside the terminal, such as clicking through Apple's window.
	See(desc screen.Description)
}

// Question is one thing the flow needs the person to decide, in the screen's own words.
type Question struct {
	Request screen.Request
	Screen  screen.Description
	// Default is what the person gets by pressing Enter, or by not being asked. Nil when there's none.
	Default *screen.Answer
	// Refused is why the last answer wasn't taken, when asking again.
	Refused error
}

// HandedOff is trouble on a run with no agent waiting: the run stops, and
// running it again retries from the step's check.
type HandedOff struct {
	Handoff session.Handoff
}

func (h *HandedOff) Error() string {
	return "the setup stopped at " + string(h.Handoff.Step) + ": " + h.Handoff.What
}

// Offer is something the setup does unless the person says no up front.
type Offer string

const (
	OfferGit      Offer = "git"
	OfferGitHub   Offer = "github"
	OfferIdentity Offer = "identity"
	OfferXcode    Offer = "xcode"
	OfferSettings Offer = "settings"
	// OfferLive is saying people already use what's being made; its default is no.
	OfferLive Offer = "live"
)

// Given is what the person said up front, in place of being asked. An answer
// given here wins over one from an earlier run, and is kept for the next.
type Given struct {
	// Name is the new project's name, outside a project.
	Name string
	// Skip are the tools to go without.
	Skip []string
	// Making is the making question's choice: website, app, code or other.
	Making string
	// In is the folder a new project goes in, as "~/git" or an absolute path.
	// Empty is ~/Projects. A setup in place ignores it.
	In string
	// GitName and GitEmail sign the person's work when GitHub can't.
	GitName, GitEmail string
	// Accepted are the offers the person took, which wins over an earlier no.
	Accepted []Offer
	// Declined are the offers the person turned down.
	Declined []Offer
}

// GivenField names the part of Given that's wrong.
type GivenField string

const (
	GivenName     GivenField = "name"
	GivenSkip     GivenField = "skip"
	GivenMaking   GivenField = "making"
	GivenGitName  GivenField = "git-name"
	GivenGitEmail GivenField = "git-email"
	GivenIn       GivenField = "in"
	// GivenOffer is an offer both taken and turned down; GivenError.Offer names it.
	GivenOffer GivenField = "offer"
)

// GivenError is an answer given up front that its question won't take.
type GivenError struct {
	Field   GivenField
	Offer   Offer
	Message string
	Valid   []string
}

func (e *GivenError) Error() string { return e.Message }

var (
	// ErrNeedsName is a setup outside a project with no name for the new one.
	ErrNeedsName = errors.New("outside a project, the setup needs a name for the new one")
	// ErrInProject is a name given in a folder that's already a project, which is set up where it is.
	ErrInProject = errors.New("this folder is already a project, so it's set up where it is")
)

// startsInPlace reports whether the starting folder is already a project. A home
// folder that's a Git repo (dotfiles) is still the home folder.
func startsInPlace(probe detect.Probe) bool {
	start := detect.Run(probe).Folder
	return start.Git == detect.Yes && start.Kind == detect.KindProject
}

// Where is the folder a setup from probe.Cwd works in: the starting folder when
// it's already a project, and otherwise <in>/<name>, where in is "~/git" or an
// absolute path, and empty is ~/Projects. A setup in place ignores in.
func Where(probe detect.Probe, name, in string) (dir string, inPlace bool, err error) {
	if startsInPlace(probe) {
		if name != "" {
			return "", false, ErrInProject
		}
		return probe.Cwd, true, nil
	}
	if name == "" {
		return "", false, ErrNeedsName
	}
	folder, ok := screen.FolderName(name)
	if !ok {
		return "", false, badName(name)
	}
	if in != "" && !screen.SafePlace(in) {
		return "", false, badPlace(in)
	}
	return projectDir(probe.Home, in, folder), false, nil
}

// projectDir is the one rule for where a new project goes: <in>/<folder>, with
// "~/" in in meaning home, and no in meaning ~/Projects.
func projectDir(home, in, folder string) string {
	if in == "" {
		in = screen.DefaultPlace
	}
	if rest, ok := strings.CutPrefix(in, "~/"); ok {
		return filepath.Join(home, rest, folder)
	}
	return filepath.Join(in, folder)
}

// Place is a folder as the naming screen and the preferences file write it:
// "~/…" under home, and absolute anywhere else.
func Place(home, dir string) string {
	if rest, ok := strings.CutPrefix(dir, home+"/"); ok && home != "" {
		return "~/" + rest
	}
	return dir
}

func badPlace(in string) error {
	return &GivenError{Field: GivenIn, Message: fmt.Sprintf("%q can't hold projects. Use ~/<folder> or a full path, with letters, numbers and dashes", in)}
}

func badName(name string) error {
	return &GivenError{Field: GivenName, Message: fmt.Sprintf("%q can't be a folder name. Use letters, numbers and dashes", name)}
}

// given is one answer from Given and the question it answers.
type given struct {
	req    screen.Request
	answer screen.Answer
	field  GivenField
}

// declines are the answers that turn each offer down.
var declines = map[Offer]given{
	OfferGit:      {req: askRepo, answer: screen.Answer{Choice: notNow}},
	OfferGitHub:   {req: askGitHub, answer: screen.Answer{Choice: notNow}},
	OfferIdentity: {req: typedName, answer: screen.Answer{Choice: skip}},
	OfferXcode:    {req: xcode, answer: screen.Answer{Choice: "later"}},
	OfferSettings: {req: settings, answer: screen.Answer{Choice: notNow}},
	OfferLive:     {req: liveYet, answer: screen.Answer{Choice: "not-yet"}},
}

// accepts are the answers that take each offer. Identity has none: taking it
// only sets aside an earlier skip, and its own questions are asked as usual.
var accepts = map[Offer]given{
	OfferGit:      {req: askRepo, answer: screen.Answer{Choice: "yes"}},
	OfferGitHub:   {req: askGitHub, answer: screen.Answer{Choice: "yes"}},
	OfferXcode:    {req: xcode, answer: screen.Answer{Choice: appStore}},
	OfferSettings: {req: settings, answer: screen.Answer{Choice: "yes"}},
	OfferLive:     {req: liveYet, answer: screen.Answer{Choice: "yes"}},
}

// optionalTools are the tools a person may go without, in catalog order.
func optionalTools() []string {
	var out []string
	for _, t := range catalog.Tools() {
		if !t.Required {
			out = append(out, t.ID)
		}
	}
	return out
}

// answers turns Given into the answers it stands for, checking each.
func (g Given) answers() ([]given, error) {
	var out []given
	if g.Name != "" {
		name, ok := screen.FolderName(g.Name)
		if !ok {
			return nil, badName(g.Name)
		}
		if g.In != "" && !screen.SafePlace(g.In) {
			return nil, badPlace(g.In)
		}
		req := naming
		req.In = cmp.Or(g.In, screen.DefaultPlace)
		out = append(out, given{req, screen.Answer{Choice: "next", Name: name, In: req.In}, GivenName})
	}
	if len(g.Skip) > 0 {
		for _, id := range g.Skip {
			tool, ok := catalog.Find(id)
			switch {
			case !ok:
				return nil, &GivenError{Field: GivenSkip, Message: fmt.Sprintf("there's no tool %q to skip", id), Valid: optionalTools()}
			case tool.Required:
				return nil, &GivenError{Field: GivenSkip, Message: fmt.Sprintf("%s is required, so it can't be skipped", id), Valid: optionalTools()}
			}
		}
		kept := slices.DeleteFunc(screen.AllToolIDs(), func(id string) bool { return slices.Contains(g.Skip, id) })
		out = append(out, given{list, screen.Answer{Choice: "lets-go", Tools: screen.KeepRequired(kept)}, GivenSkip})
	}
	if g.Making != "" {
		out = append(out, given{making, screen.Answer{Choice: g.Making}, GivenMaking})
	}
	if g.GitName != "" {
		out = append(out, given{typedName, screen.Answer{Choice: "next", Name: g.GitName}, GivenGitName})
	}
	if g.GitEmail != "" {
		out = append(out, given{typedEmail, screen.Answer{Choice: "next", Name: g.GitEmail}, GivenGitEmail})
	}
	for _, offer := range g.Accepted {
		if slices.Contains(g.Declined, offer) {
			return nil, &GivenError{Field: GivenOffer, Offer: offer, Message: fmt.Sprintf("%s is both taken and turned down", offer)}
		}
		if _, ok := declines[offer]; !ok {
			return nil, fmt.Errorf("there's no offer %q to take", offer)
		}
		if a, ok := accepts[offer]; ok {
			out = append(out, a)
		}
	}
	for _, offer := range g.Declined {
		d, ok := declines[offer]
		if !ok {
			return nil, fmt.Errorf("there's no offer %q to decline", offer)
		}
		out = append(out, d)
	}
	for _, a := range out {
		desc, err := screen.Describe(a.req, screen.Context{Agent: screen.ClaudeCode})
		if err != nil {
			return nil, err
		}
		if err := screen.ValidateAnswer(desc, a.answer); err != nil {
			bad := &GivenError{Field: a.field, Message: err.Error()}
			if invalid, ok := errors.AsType[*screen.InvalidError](err); ok {
				bad.Valid = invalid.Valid
			}
			return nil, bad
		}
	}
	return out, nil
}

// pickUp starts a run with no page where the last one left off: trouble it
// stopped at is retried, and what the person gave now is recorded over what
// they said before.
func (f *Flow) pickUp() error {
	state, err := f.cfg.Store.Load()
	if err != nil {
		return err
	}
	if state.Open != nil {
		if state, err = f.cfg.Store.Resume(session.WayRetry, time.Now()); err != nil {
			return err
		}
	}
	answers, err := f.cfg.Given.answers()
	if err != nil {
		return err
	}
	for _, a := range answers {
		if a.req.Screen == naming.Screen && f.inPlace {
			continue
		}
		if last, ok := latest(state, a.req); ok && sameAnswer(last, a.answer) {
			continue
		}
		shown, err := f.cfg.Store.Show(a.req, 0, time.Now())
		if err != nil {
			return err
		}
		if state, err = f.cfg.Store.RecordAnswer(shown.Current.Seq, a.answer, time.Now()); err != nil {
			return err
		}
		recorded, _ := state.CurrentAnswer()
		if _, err := f.cfg.Store.Note(told(a.req, recorded.Answer), time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func latest(state session.State, req screen.Request) (screen.Answer, bool) {
	for _, a := range slices.Backward(state.Answers) {
		if sameQuestion(a.Request, req) {
			return a.Answer, true
		}
	}
	return screen.Answer{}, false
}

func sameAnswer(a, b screen.Answer) bool {
	return a.Choice == b.Choice && a.Name == b.Name && a.In == b.In && slices.Equal(a.Tools, b.Tools)
}

// defaultChoice is what each question takes when the person just presses Enter:
// the setup's offers are on unless they say no (Ben, 2026-11-05).
var defaultChoice = map[screen.ID]string{
	"git-ask":       "yes",
	"github":        "yes",
	"github-signup": "made-it",
	"identity":      useThese,
	"xcode":         appStore,
	"live-yet":      "not-yet",
	"settings":      "yes",
}

// makingFor is the making question's choice for what a folder's files say.
var makingFor = map[detect.Making]string{detect.MakingWebsite: website, detect.MakingApp: anApp, detect.MakingUnknown: "other"}

// question is a question with its words and its default. Settled is a question
// the flow answers itself with the default: the welcome, which is the page's
// tour, and the tool list when everything on it is already installed.
func (r *run) question(req screen.Request) (q Question, settled bool, err error) {
	state, err := r.state()
	if err != nil {
		return Question{}, false, err
	}
	desc, err := screen.Describe(req, screen.Context{Agent: state.Agent, Chosen: state.Chosen, Carried: state.Progress})
	if err != nil {
		return Question{}, false, err
	}
	q = Question{Request: req, Screen: desc}
	var a screen.Answer
	switch req.Screen {
	case welcome.Screen:
		a, settled = screen.Answer{Choice: "see-plan"}, true
	case list.Screen:
		a = screen.Answer{Choice: "lets-go", Tools: screen.KeepRequired(screen.AllToolIDs())}
		settled = toolsSettled(r.f.cfg.Probe)
	case making.Screen:
		dir, err := r.mustProject()
		if err != nil {
			return Question{}, false, err
		}
		a = screen.Answer{Choice: makingFor[detect.ProjectMaking(dir)]}
	default:
		choice, ok := defaultChoice[req.Screen]
		if !ok {
			return q, false, nil
		}
		a = screen.Answer{Choice: choice}
	}
	q.Default = &a
	return q, settled, nil
}

// askPerson is ask with no page: the person answers, or the default does, and
// the answer goes into the log as the page's would.
func (r *run) askPerson(req screen.Request) error {
	q, settled, err := r.question(req)
	if err != nil {
		return err
	}
	shown, err := r.f.cfg.Store.Show(req, 0, time.Now())
	if err != nil {
		return err
	}
	for {
		var a screen.Answer
		if settled {
			a = *q.Default
		} else if a, err = r.f.cfg.Person.Ask(r.ctx, q); err != nil {
			return err
		}
		state, err := r.f.cfg.Store.RecordAnswer(shown.Current.Seq, a, time.Now())
		if invalid, ok := errors.AsType[*screen.InvalidError](err); ok && !settled {
			q.Refused = invalid
			continue
		}
		if err != nil || settled {
			// Nobody chose a settled answer, so there's nothing to tell.
			return err
		}
		recorded, _ := state.CurrentAnswer()
		return r.note("%s", told(req, recorded.Answer))
	}
}

// see tells the person about a screen that asks nothing of them.
func (r *run) see(state session.State, req screen.Request) error {
	desc, err := screen.Describe(req, screen.Context{Agent: state.Agent, Chosen: state.Chosen, Carried: state.Progress})
	if err != nil {
		return err
	}
	r.f.cfg.Person.See(desc)
	return nil
}

// phoneHint is the phone question as a line to pass on, since only the page asks it.
func phoneHint(agent screen.Agent) string {
	hint := fmt.Sprintf("to use %s from your phone, ask %s how", agent.AI, agent.AI)
	if agent.ID == screen.ClaudeCode.ID {
		hint += " (in Claude Code, it's /remote-control)"
	}
	return hint
}
