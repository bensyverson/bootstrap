// Package flow runs the setup as data: a fixed list of steps, each of which checks
// whether it's already done before it acts, so the order can't be skipped or
// reordered and a second run changes nothing. It runs inside the server. Every
// outside command goes through a Runner, every step, event and handoff goes into
// the session log, and the agent reads them there with no server connection.
//
// When a result is anything the flow doesn't expect, it hands off to the agent
// with what happened, the evidence and the ways forward. It never guesses.
package flow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The steps, in order: the order in project/2026-11-05-server-driven-flow.md.
// There's no commit: the setup never commits for the person (Ben, 2026-11-05),
// and Git's identity comes after GitHub, which can supply it.
const (
	StepPlan     session.StepID = "plan"
	StepApple    session.StepID = "apple"
	StepHomebrew session.StepID = "homebrew"
	StepTools    session.StepID = "tools"
	StepFolder   session.StepID = "folder"
	StepGit      session.StepID = "git"
	StepGitHub   session.StepID = "github"
	StepIdentity session.StepID = "identity"
	StepStage    session.StepID = "stage"
	StepRules    session.StepID = "rules"
	StepSettings session.StepID = "settings"
	StepPhone    session.StepID = "phone"
	StepDone     session.StepID = "done"
)

// Step is one step as data: the screens it shows, the screens whose answers it
// needs, and whether the person may go on without it. Any step can also show the
// chat screen, when it hands off.
type Step struct {
	ID        session.StepID
	Screens   []screen.ID
	Asks      []screen.ID
	Skippable bool
}

// Steps lists the setup in order.
func Steps() []Step {
	var out []Step
	for _, def := range steps {
		out = append(out, def.Step)
	}
	return out
}

// Limits are how long things may take before the flow hands off.
type Limits struct {
	// Command is any one command, unless it says otherwise.
	Command time.Duration
	// Apple is Apple's tools, from the person's click to Git arriving.
	Apple time.Duration
	// Homebrew is Homebrew's installer, from opening it to brew arriving.
	Homebrew time.Duration
	// Login is gh's web sign-in, which waits on the person in a browser.
	Login time.Duration
}

// DefaultLimits are the limits outside tests. Each is far past what the step
// takes on a working Mac, since going past one hands off.
func DefaultLimits() Limits {
	return Limits{Command: 10 * time.Minute, Apple: time.Hour, Homebrew: 30 * time.Minute, Login: 15 * time.Minute}
}

// DefaultHooks are the tools whose own `hook install` the settings step runs (GGbVZ5).
func DefaultHooks() []string { return []string{"contextfill", "jobs"} }

// defaultPoll is how often the flow reads the log for writes from other
// processes, such as the agent's resume. The server's own writes, the person's
// answers among them, ring the store's bell and wake the flow at once.
const defaultPoll = time.Second

// Config is what a flow needs. Probe.Cwd is the folder the agent started in, and
// Probe.Home is where ~/Projects goes.
type Config struct {
	Store  session.Store
	Runner Runner
	Probe  detect.Probe
	// HomebrewPkg is the URL of Homebrew's .pkg.
	HomebrewPkg string
	// Hooks are catalog tool ids whose own hook install the settings step runs.
	Hooks  []string
	Limits Limits
	// Poll is how often the flow reads the log while it waits on the page or the
	// agent, for writes that don't ring Store.Bell.
	Poll time.Duration
}

// Flow is one run of the setup.
type Flow struct {
	cfg Config
	// inPlace is a starting folder that's already a project, which is set up where it is.
	inPlace bool
}

// New checks a config and fills in its defaults.
func New(cfg Config) (*Flow, error) {
	switch {
	case cfg.Store.Dir == "":
		return nil, errors.New("a flow needs the session's folder")
	case cfg.Runner == nil:
		return nil, errors.New("a flow needs a runner")
	case cfg.HomebrewPkg == "":
		return nil, errors.New("a flow needs Homebrew's package URL")
	case cfg.Probe.Cwd == "" || cfg.Probe.Home == "":
		return nil, errors.New("a flow needs the starting folder and the home folder")
	}
	for _, id := range cfg.Hooks {
		if tool, ok := catalog.Find(id); !ok || tool.OwnStep {
			return nil, fmt.Errorf("%q has no hook to install", id)
		}
	}
	defaults := DefaultLimits()
	for _, pair := range []struct{ got, def *time.Duration }{
		{&cfg.Limits.Command, &defaults.Command}, {&cfg.Limits.Apple, &defaults.Apple},
		{&cfg.Limits.Homebrew, &defaults.Homebrew}, {&cfg.Limits.Login, &defaults.Login},
	} {
		if *pair.got == 0 {
			*pair.got = *pair.def
		}
	}
	if cfg.Poll == 0 {
		cfg.Poll = defaultPoll
	}
	return &Flow{cfg: cfg}, nil
}

// Run walks every step to the end. It returns early only when the log can't be
// written, the session ends, or ctx is done: anything else is a handoff.
func (f *Flow) Run(ctx context.Context) error {
	start := detect.Run(f.cfg.Probe).Folder
	// A home folder that's a Git repo (dotfiles) is still the home folder.
	f.inPlace = start.Git == detect.Yes && start.Kind == detect.KindProject
	if err := os.MkdirAll(f.logs(), 0o700); err != nil {
		return err
	}
	if _, err := f.cfg.Store.Drive(time.Now()); err != nil {
		return err
	}
	for i, def := range steps {
		r := &run{f: f, ctx: ctx, def: def, index: i + 1}
		if err := r.walk(); err != nil {
			return err
		}
	}
	return nil
}

func (f *Flow) logs() string { return filepath.Join(f.cfg.Store.Dir, "steps") }

// outcome is what a step's act came to.
type outcome struct {
	kind    outcomeKind
	ask     screen.Request
	handoff session.Handoff
}

type outcomeKind int

const (
	outcomeDone outcomeKind = iota
	outcomeSkipped
	// outcomeNeedsPerson is a question on the page. The act runs again once it's answered.
	outcomeNeedsPerson
	// outcomeHandOff is trouble, which waits for the agent.
	outcomeHandOff
)

func done() (outcome, error)    { return outcome{kind: outcomeDone}, nil }
func skipped() (outcome, error) { return outcome{kind: outcomeSkipped}, nil }
func needs(req screen.Request) (outcome, error) {
	return outcome{kind: outcomeNeedsPerson, ask: req}, nil
}
func handOff(h session.Handoff) (outcome, error) {
	return outcome{kind: outcomeHandOff, handoff: h}, nil
}

// stepDef is a step with its behavior. check says whether it's already finished,
// as done or skipped, or "" when it must act. act must be safe to call again:
// after a question is answered, or a retry, it runs from the top, and every part
// of it checks before it changes anything.
type stepDef struct {
	Step
	check func(*run) (session.StepStatus, *session.Handoff, error)
	act   func(*run) (outcome, error)
}

// run is one step being walked.
type run struct {
	f     *Flow
	ctx   context.Context
	def   stepDef
	index int
}

func (r *run) record(status session.StepStatus) error {
	_, err := r.f.cfg.Store.RecordStep(session.Step{ID: r.def.ID, Status: status, Index: r.index, Of: len(steps)}, time.Now())
	return err
}

func (r *run) walk() error {
	if err := r.record(session.StepStarted); err != nil {
		return err
	}
	for {
		status, trouble, err := r.def.check(r)
		if err != nil {
			return err
		}
		if trouble != nil {
			finished, err := r.handOff(*trouble)
			if err != nil || finished {
				return err
			}
			continue
		}
		if status != "" {
			return r.record(status)
		}
		finished, err := r.act()
		if err != nil || finished {
			return err
		}
	}
}

// act runs the step's act until it finishes, or until a retry sends it back to its check.
func (r *run) act() (finished bool, err error) {
	for {
		out, err := r.def.act(r)
		if err != nil {
			return false, err
		}
		switch out.kind {
		case outcomeNeedsPerson:
			if err := r.ask(out.ask); err != nil {
				return false, err
			}
		case outcomeDone:
			return true, r.record(session.StepDone)
		case outcomeSkipped:
			return true, r.record(session.StepSkipped)
		case outcomeHandOff:
			return r.handOff(out.handoff)
		}
	}
}

// handOff waits for the agent to resume. A skip of a whole step finishes it; a
// skip of one tool, or a retry, goes back to the check.
func (r *run) handOff(h session.Handoff) (finished bool, err error) {
	if _, err := r.f.cfg.Store.HandOff(h, time.Now()); err != nil {
		return false, err
	}
	if err := r.record(session.StepHandedOff); err != nil {
		return false, err
	}
	if err := r.show(screen.Request{Screen: "chat"}); err != nil {
		return false, err
	}
	var way session.Way
	err = r.poll(func(state session.State) bool {
		if state.Open != nil {
			return false
		}
		way = state.Resumed.Way
		return true
	})
	if err != nil {
		return false, err
	}
	if way == session.WaySkip && h.Tool == "" {
		return true, r.record(session.StepSkipped)
	}
	return false, nil
}

// tell hands the agent something to do that the flow doesn't wait for.
func (r *run) tell(h session.Handoff) error {
	_, err := r.f.cfg.Store.HandOff(h, time.Now())
	return err
}

func (r *run) note(format string, args ...any) error {
	_, err := r.f.cfg.Store.Note(fmt.Sprintf(format, args...), time.Now())
	return err
}

func (r *run) state() (session.State, error) { return r.f.cfg.Store.Load() }

// poll reads the log until done says so, the session ends, or ctx is done. It
// reads again at each ring of the store's bell, and at each tick for the rest.
func (r *run) poll(done func(session.State) bool) error {
	tick := time.NewTicker(r.f.cfg.Poll)
	defer tick.Stop()
	for {
		rung := r.f.cfg.Store.Bell.Rung()
		state, err := r.state()
		if err != nil {
			return err
		}
		if state.Ended {
			return session.ErrEnded
		}
		if done(state) {
			return nil
		}
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-rung:
		case <-tick.C:
		}
	}
}

// show puts a screen on the page, unless it's there already.
func (r *run) show(req screen.Request) error {
	state, err := r.state()
	if err != nil {
		return err
	}
	if state.Current != nil && state.Current.Request == req {
		return nil
	}
	_, err = r.f.cfg.Store.Show(req, 0, time.Now())
	return err
}

// answer is the person's latest answer to a question, from anywhere in this
// session's log, so a flow that runs again doesn't ask again.
func (r *run) answer(req screen.Request) (screen.Answer, bool, error) {
	state, err := r.state()
	if err != nil {
		return screen.Answer{}, false, err
	}
	for _, a := range slices.Backward(state.Answers) {
		if a.Request == req {
			return a.Answer, true, nil
		}
	}
	return screen.Answer{}, false, nil
}

// ask shows a question and waits for the person's answer. If another screen
// replaces it, as an agent's stray show would, the question goes back up.
func (r *run) ask(req screen.Request) error {
	for {
		state, err := r.f.cfg.Store.Show(req, 0, time.Now())
		if err != nil {
			return err
		}
		seq := state.Current.Seq
		var got screen.Answer
		moved := false
		err = r.poll(func(state session.State) bool {
			if state.Current == nil || state.Current.Seq != seq {
				moved = true
				return true
			}
			a, ok := state.CurrentAnswer()
			got = a.Answer
			return ok
		})
		if err != nil {
			return err
		}
		if !moved {
			return r.note("%s", told(req, got))
		}
	}
}

// told is a person's answer as a line the agent can pass on.
func told(req screen.Request, a screen.Answer) string {
	switch {
	case len(a.Tools) > 0 || req.Screen == "list":
		return fmt.Sprintf("person chose %s, keeping %s", a.Choice, joinList(screen.KeepRequired(a.Tools)))
	case req.Screen == "naming":
		return "person named the project " + a.Name
	case a.Name != "" && a.Choice != "skip":
		return fmt.Sprintf("person typed %q on %s", a.Name, req.Screen)
	case req.Status != "":
		return fmt.Sprintf("person chose %s on %s %s", a.Choice, req.Screen, req.Status)
	}
	return fmt.Sprintf("person chose %s on %s", a.Choice, req.Screen)
}
