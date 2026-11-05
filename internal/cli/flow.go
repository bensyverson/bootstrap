package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/server"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The agent's side of the server-driven flow: run starts it, wait reports it, and
// resume answers a handoff. See project/2026-11-05-server-driven-flow.md.

const (
	stillWorking = "still working, run wait again"
	nextWait     = "bootstrap wait"
	runWithApp   = "bootstrap run --agent <app>"
	noFlow       = "the page is open, but the setup isn't running in it"
	setupOver    = "the setup is over, so there is nothing more to run"
)

// newFlow is the setup as the background server runs it, on this Mac. The
// runner shares the detect seam's root, so a test's pretend Mac holds the programs too.
func (c *call) newFlow(store session.Store) (*flow.Flow, error) {
	cwd, err := c.env.Cwd()
	if err != nil {
		return nil, err
	}
	root := c.env.Getenv(envRoot)
	return flow.New(flow.Config{
		Store:       store,
		Runner:      flow.ExecRunner{Root: root},
		Probe:       detect.Probe{Root: root, Home: c.env.Getenv("HOME"), Cwd: cwd, Chip: detect.Chip, Free: detect.FreeBytes, Running: detect.Running},
		HomebrewPkg: flow.DefaultHomebrewPkg,
		Hooks:       flow.DefaultHooks(),
	})
}

// startOptions is how serve and run start the background server.
func (c *call) startOptions(store session.Store, agent screen.Agent, verb string) (server.StartOptions, error) {
	exe, err := c.env.Exe()
	if err != nil {
		return server.StartOptions{}, &failure{code: exitEnviron, message: "can't start the page: " + err.Error(), next: fallbackNext}
	}
	port := 0
	if raw := c.env.Getenv(envPort); raw != "" {
		if port, err = strconv.Atoi(raw); err != nil {
			return server.StartOptions{}, &failure{code: exitUsage, message: fmt.Sprintf("%s %q isn't a port", envPort, raw), next: "bootstrap " + verb + " --help"}
		}
	}
	open := c.env.Getenv(envOpen)
	if open == "" {
		open = defaultOpen
	}
	return server.StartOptions{Store: store, Agent: agent, Exe: exe, Env: c.env.Environ(), Port: port, OpenPath: open}, nil
}

// runsWithoutFlow is a page that serve started, which has no setup running in it.
func runsWithoutFlow(store session.Store) bool {
	if !store.ServerRunning() {
		return false
	}
	info, err := store.ReadServerInfo()
	return err == nil && !info.Flow
}

type runValue struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Browser string `json:"browser"`
	Joined  bool   `json:"joined"`
	Next    string `json:"next"`
}

var runCommand = command{
	name: "run", class: "Act", usage: "run [--agent <claude-code|opencode>] [--json]",
	about: "Starts the setup in the background: the page, and the server that runs every step and asks the person on the page. Returns at once. " +
		"A second run joins the setup already running, and a run after a stop picks up where the log left off. Then loop on bootstrap wait. --agent defaults from the environment.",
	example: "bootstrap run --agent claude-code",
	flags:   []string{"agent"},
	run: func(c *call) (output, error) {
		if err := c.noArgs("run"); err != nil {
			return output{}, err
		}
		agent, known, err := c.agent()
		if err != nil {
			return output{}, err
		}
		if !known {
			return output{}, &failure{code: exitUsage, message: "nothing says which app this is", valid: screen.AgentIDs(), next: runWithApp}
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		if runsWithoutFlow(store) {
			return output{}, &failure{code: exitEnviron, message: noFlow, next: "bootstrap stop, then bootstrap run --agent " + agent.ID}
		}
		opts, err := c.startOptions(store, agent, "run")
		if err != nil {
			return output{}, err
		}
		opts.Flow = true
		started, err := server.Start(opts)
		if err != nil {
			return output{}, serveFailure(err)
		}
		row := fmt.Sprintf("running url=%s pid=%d browser=%s joined=%s", started.URL, started.PID, started.Browser, yesNo(started.Reused))
		if started.Browser == server.BrowserFailed {
			return output{}, &failure{code: exitEnviron, message: "the browser didn't open, but the setup is running", state: row, url: started.URL, pid: started.PID, browser: string(started.Browser),
				next: "ask the person to open the url, then bootstrap wait. If they can't, bootstrap stop, then " + fallbackNext}
		}
		return output{rows: []string{row}, next: nextWait, value: runValue{URL: started.URL, PID: started.PID, Browser: string(started.Browser), Joined: started.Reused, Next: nextWait}}, nil
	},
}

type waitFlowValue struct {
	Updates  []session.Update `json:"updates"`
	Open     *session.Open    `json:"open,omitempty"`
	Step     *session.Step    `json:"step,omitempty"`
	Screen   screen.ID        `json:"screen,omitempty"`
	Finished bool             `json:"finished"`
	Next     []string         `json:"next"`
}

// waitFlow is wait with no target: the agent's loop while the server runs the setup.
func waitFlow(c *call, timeout time.Duration) (output, error) {
	store, err := c.store()
	if err != nil {
		return output{}, err
	}
	state, err := store.Load()
	if errors.Is(err, session.ErrNoSession) {
		return output{}, &failure{code: exitEnviron, message: "no session", next: runWithApp}
	}
	if err != nil {
		return output{}, err
	}
	again := "bootstrap run --agent " + state.Agent.ID
	if runsWithoutFlow(store) {
		return output{}, &failure{code: exitEnviron, message: noFlow, next: "bootstrap stop, then " + again}
	}
	told, err := store.WaitFlow(context.Background(), timeout)
	switch {
	case errors.Is(err, session.ErrEnded):
		return output{}, &failure{code: exitEnviron, message: "session ended", next: again}
	case errors.Is(err, session.ErrFlowStopped):
		return output{}, &failure{code: exitEnviron, message: err.Error(), next: again}
	case errors.Is(err, session.ErrPageGone):
		return output{}, &failure{code: exitEnviron, message: "the page is closed, so the setup isn't running", next: again}
	case errors.Is(err, session.ErrWaitTimeout):
		rows, _ := toldRows(told)
		return output{}, &failure{code: exitTimeout, message: stillWorking, state: strings.Join(rows, "\n"), updates: nonNil(told.Updates), next: nextWait}
	case err != nil:
		return output{}, err
	}
	rows, next := toldRows(told)
	for _, n := range next {
		rows = append(rows, "next: "+n)
	}
	value := waitFlowValue{Updates: nonNil(told.Updates), Open: told.Open, Step: told.State.Step, Finished: finished(told), Next: next}
	if told.State.Current != nil {
		value.Screen = told.State.Current.Request.Screen
	}
	return output{rows: rows, value: value}, nil
}

func nonNil(u []session.Update) []session.Update {
	if u == nil {
		return []session.Update{}
	}
	return u
}

func finished(told session.Told) bool {
	if told.Finished {
		return true
	}
	for _, u := range told.Updates {
		if u.Handoff != nil && u.Handoff.Kind == session.HandoffDone {
			return true
		}
	}
	return false
}

// toldRows is what a wait says, as rows to pass on, and the ways forward.
func toldRows(told session.Told) (rows, next []string) {
	agent := told.State.Agent
	var handoffs []session.Handoff
	openShown := false
	for i, u := range told.Updates {
		switch {
		case u.Step != nil:
			// A handoff says more than its step's status, and a step that moved on says only where it got to.
			if u.Step.Status == session.StepHandedOff {
				continue
			}
			if i+1 < len(told.Updates) && told.Updates[i+1].Step != nil && told.Updates[i+1].Step.ID == u.Step.ID {
				continue
			}
			rows = append(rows, fmt.Sprintf("step %s %s %d of %d", u.Step.ID, u.Step.Status, u.Step.Index, u.Step.Of))
		case u.Note != "":
			rows = append(rows, "note "+u.Note)
		case u.Handoff != nil:
			rows = append(rows, handoffRows(*u.Handoff)...)
			handoffs = append(handoffs, *u.Handoff)
			if told.Open != nil && told.Open.Seq == u.Seq {
				openShown = true
			}
		}
	}
	if told.Open != nil && !openShown {
		// Still waiting on the agent from an earlier wait, so it's said again.
		rows = append(rows, handoffRows(told.Open.Handoff)...)
		handoffs = append(handoffs, told.Open.Handoff)
	}
	rows = append(rows, stateRow(told))
	for _, h := range handoffs {
		next = append(next, waysForward(h, agent)...)
	}
	if len(next) == 0 && finished(told) {
		next = []string{setupOver}
	}
	return rows, next
}

func stateRow(told session.Told) string {
	if finished(told) {
		return "state finished"
	}
	cells := []string{"state working"}
	if told.Open != nil {
		cells[0] = "state handed-off"
	}
	if step := told.State.Step; step != nil {
		cells = append(cells, fmt.Sprintf("step=%s %d of %d", step.ID, step.Index, step.Of))
	}
	if told.Open == nil && told.State.Current != nil {
		cells = append(cells, "screen="+string(told.State.Current.Request.Screen))
	}
	return strings.Join(cells, " ")
}

// handoffRows says what happened and shows the evidence, so the agent needn't run anything to see it.
func handoffRows(h session.Handoff) []string {
	head := []string{"handoff", string(h.Kind), "step=" + string(h.Step)}
	if h.Tool != "" {
		head = append(head, "tool="+h.Tool)
	}
	if h.Reason != "" {
		head = append(head, "reason="+string(h.Reason))
	}
	rows := []string{strings.Join(head, " "), "what: " + h.What}
	ev := h.Evidence
	if ev.Command != "" {
		row := "command: " + ev.Command
		if ev.Exit != 0 {
			row += fmt.Sprintf(" (exit %d)", ev.Exit)
		}
		rows = append(rows, row)
	}
	if ev.Limit > 0 {
		row := fmt.Sprintf("limit: %v", ev.Limit)
		if ev.Elapsed > 0 {
			row += fmt.Sprintf(", ran %v", ev.Elapsed)
		}
		rows = append(rows, row)
	}
	if ev.Log != "" {
		rows = append(rows, "log: "+ev.Log)
	}
	if len(ev.Tail) > 0 {
		rows = append(rows, "output, last lines:")
		for _, line := range ev.Tail {
			rows = append(rows, "  | "+line)
		}
	}
	return rows
}

// waysForward is one next line per way a handoff offers, each a whole instruction.
func waysForward(h session.Handoff, agent screen.Agent) []string {
	switch h.Kind {
	case session.HandoffPhone:
		return []string{"do what the phone handoff says in the chat, then " + nextWait}
	case session.HandoffDone:
		return []string{"do what the done handoff says in the chat; " + setupOver}
	}
	var out []string
	canSkip := false
	for _, w := range h.Ways {
		if w == session.WaySkip {
			canSkip = true
		}
	}
	for _, w := range h.Ways {
		switch w {
		case session.WayRetry:
			look := "look into what happened"
			if h.Evidence.Log != "" {
				look = "read the log"
			}
			out = append(out, look+", fix what you can, then bootstrap resume --retry")
		case session.WaySkip:
			without := "this step"
			if h.Tool != "" {
				without = h.Tool
			}
			out = append(out, fmt.Sprintf("or, to go on without %s: bootstrap resume --skip", without))
		case session.WayTalk:
			resume := "bootstrap resume --retry"
			if canSkip {
				resume += " or --skip"
			}
			out = append(out, fmt.Sprintf("or, to talk it over: ask the person in the chat (the page says %q), then %s", chatTitle(agent), resume))
		}
	}
	return out
}

// chatTitle is what the page shows while a handoff waits, in the screen's own words.
func chatTitle(agent screen.Agent) string {
	desc, err := screen.Describe(screen.Request{Screen: "chat"}, screen.Context{Agent: agent})
	if err != nil {
		return ""
	}
	var words []string
	for _, span := range desc.Title {
		words = append(words, span.Text)
	}
	return strings.Join(words, "")
}

// answered is the handoff the last resume answered.
func answered(state session.State) session.Handoff {
	for _, l := range state.Handoffs {
		if state.Resumed != nil && l.Seq == state.Resumed.For {
			return l.Handoff
		}
	}
	return session.Handoff{}
}

type resumeValue struct {
	Way  session.Way    `json:"way"`
	Step session.StepID `json:"step"`
	Tool string         `json:"tool,omitempty"`
	Next string         `json:"next"`
}

var resumeWays = []string{"--retry", "--skip"}

var resumeCommand = command{
	name: "resume", class: "Act", usage: "resume --retry|--skip [--json]",
	about: "Answers the handoff that bootstrap wait gave you: --retry runs the step again from its check, and --skip goes on without the tool or step, when the handoff offers it. " +
		"Talking it over isn't a resume: do that in the chat, then resume.",
	example:  "bootstrap resume --retry",
	switches: []string{"retry", "skip"},
	run: func(c *call) (output, error) {
		if err := c.noArgs("resume"); err != nil {
			return output{}, err
		}
		retry, skip := c.switched("retry"), c.switched("skip")
		switch {
		case retry && skip:
			return output{}, &failure{code: exitUsage, message: "resume takes one of --retry or --skip", valid: resumeWays, next: nextWait}
		case !retry && !skip:
			return output{}, &failure{code: exitUsage, message: "resume needs --retry or --skip", valid: resumeWays, next: nextWait}
		}
		way := session.WayRetry
		if skip {
			way = session.WaySkip
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		before, err := store.Load()
		if errors.Is(err, session.ErrNoSession) {
			return output{}, &failure{code: exitEnviron, message: "no session", next: runWithApp}
		}
		if err != nil {
			return output{}, err
		}
		after, err := store.Resume(way, time.Now())
		if invalid, ok := errors.AsType[*screen.InvalidError](err); ok {
			return output{}, &failure{code: exitUsage, message: invalid.Message, valid: invalid.Valid, next: "bootstrap resume --" + invalid.Valid[0]}
		}
		switch {
		case errors.Is(err, session.ErrNoHandoff):
			return output{}, &failure{code: exitTimeout, message: err.Error(), next: nextWait}
		case errors.Is(err, session.ErrEnded):
			return output{}, &failure{code: exitEnviron, message: "session ended", next: "bootstrap run --agent " + before.Agent.ID}
		case err != nil:
			return output{}, err
		}
		h := answered(after)
		row := fmt.Sprintf("resumed way=%s step=%s", way, h.Step)
		if h.Tool != "" {
			row += " tool=" + h.Tool
		}
		return output{rows: []string{row}, next: nextWait, value: resumeValue{Way: way, Step: h.Step, Tool: h.Tool, Next: nextWait}}, nil
	},
}
