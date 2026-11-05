package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/page"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/server"
	"github.com/bensyverson/bootstrap/internal/session"
)

const (
	hiddenVerb = server.HiddenVerb
	// stopWait is how long stop waits for the server to notice and exit.
	stopWait     = 5 * time.Second
	stillWaiting = "still waiting, run wait again"
)

type serveValue struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Browser string `json:"browser"`
	Reused  bool   `json:"reused"`
	Next    string `json:"next"`
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

var serveCommand = command{
	name: "serve", class: "Act", usage: "serve [--agent <claude-code|opencode>] [--json]",
	about:   "Starts the page in the background, opens the browser and returns. A second serve reuses a running page. --agent picks the app's name people read, and defaults from the environment.",
	example: "bootstrap serve --agent claude-code",
	flags:   []string{"agent"},
	run: func(c *call) (output, error) {
		if err := c.noArgs("serve"); err != nil {
			return output{}, err
		}
		agent, known, err := c.agent()
		if err != nil {
			return output{}, err
		}
		if !known {
			return output{}, &failure{code: exitUsage, message: "nothing says which app this is", valid: screen.AgentIDs(), next: "bootstrap serve --agent <app>"}
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		opts, err := c.startOptions(store, agent, "serve")
		if err != nil {
			return output{}, err
		}
		started, err := server.Start(opts)
		if err != nil {
			return output{}, serveFailure(err)
		}
		row := fmt.Sprintf("serving url=%s pid=%d browser=%s reused=%s", started.URL, started.PID, started.Browser, yesNo(started.Reused))
		if started.Browser == server.BrowserFailed {
			return output{}, &failure{code: exitEnviron, message: "the browser didn't open", state: row, url: started.URL, pid: started.PID, browser: string(started.Browser),
				next: "ask the person to open the url. If they can't, " + fallbackNext}
		}
		state, err := store.Load()
		if err != nil {
			return output{}, err
		}
		next := nextAfter(state, true)
		return output{rows: []string{row}, next: next, value: serveValue{URL: started.URL, PID: started.PID, Browser: string(started.Browser), Reused: started.Reused, Next: next}}, nil
	},
}

func serveFailure(err error) error {
	if start, ok := errors.AsType[*server.StartError](err); ok {
		return &failure{code: exitEnviron, message: "can't start the page: " + start.Reason, next: fallbackNext}
	}
	return err
}

type showValue struct {
	Seq         int                `json:"seq"`
	Screen      screen.ID          `json:"screen"`
	Tool        string             `json:"tool,omitempty"`
	Status      string             `json:"status,omitempty"`
	Reason      string             `json:"reason,omitempty"`
	Mode        screen.Mode        `json:"mode"`
	Progress    float64            `json:"progress"`
	Description screen.Description `json:"description"`
	Next        string             `json:"next"`
}

var showCommand = command{
	name: "show", class: "Act", usage: "show <screen> [<tool>] [--status <s>] [--reason <r>] [--expect 20m] [--json]",
	about:   "Moves the page to a screen and renews the time before the page says the agent is lost: 3 minutes, or --expect. Screens, tools, statuses and reasons are closed lists: see bootstrap screens.",
	example: "bootstrap show tool jobs --status installing --expect 20m",
	flags:   []string{"status", "reason", "expect"},
	run: func(c *call) (output, error) {
		if len(c.args) == 0 {
			return output{}, &failure{code: exitUsage, message: "show needs a screen", valid: screen.ScreenIDs(), next: "bootstrap screens"}
		}
		if len(c.args) > 2 {
			return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unexpected argument %q", c.args[2]), next: "bootstrap show --help"}
		}
		req := screen.Request{Screen: screen.ID(c.args[0]), Status: c.flag("status"), Reason: c.flag("reason")}
		if len(c.args) == 2 {
			req.Tool = c.args[1]
		}
		var expect time.Duration
		if raw := c.flag("expect"); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil || parsed <= 0 {
				return output{}, &failure{code: exitUsage, message: fmt.Sprintf("--expect %q isn't a length of time, like 20m", raw), next: "bootstrap show --help"}
			}
			expect = parsed
		}
		if err := screen.Validate(req); err != nil {
			return output{}, err
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		state, err := store.Show(req, expect, time.Now())
		if err != nil {
			return output{}, err
		}
		if !store.ServerRunning() {
			return output{}, &failure{code: exitEnviron, message: "saved, but the page is closed", state: fmt.Sprintf("seq=%d screen=%s", state.Current.Seq, req.Screen), next: "bootstrap serve"}
		}
		desc, err := screen.Describe(req, screen.Context{Agent: state.Agent, Chosen: state.Chosen, Carried: state.Progress})
		if err != nil {
			return output{}, err
		}
		next := screen.NextFor(req.Screen)
		row := screenRow(fmt.Sprintf("shown seq=%d screen=%s", state.Current.Seq, req.Screen), req, desc)
		return output{rows: []string{row}, next: next, value: showValue{Seq: state.Current.Seq, Screen: req.Screen, Tool: req.Tool, Status: req.Status, Reason: req.Reason, Mode: desc.Mode, Progress: desc.Progress, Description: desc, Next: next}}, nil
	},
}

// arrivalTargets maps what wait accepts to the catalog tool that proves it.
var arrivalTargets = map[string]string{"clt": "clt", "brew": "homebrew"}

// arrivalNext is the step after a tool lands. The agent confirms Homebrew by running it.
var arrivalNext = map[string]string{"clt": "bootstrap show <screen>", "brew": "/opt/homebrew/bin/brew --version"}

// startedNext is the step after someone gives an installer the go-ahead.
var startedNext = map[string]string{"clt": "bootstrap show apple-wait, then bootstrap wait clt"}

var waitTargets = []string{"answer", "clt", "brew"}

type arrivalValue struct {
	Target string        `json:"target"`
	Event  session.Event `json:"event"`
	Path   string        `json:"path,omitempty"`
	Next   string        `json:"next"`
}

type answerValue struct {
	statusAnswer
	Next string `json:"next"`
}

var waitCommand = command{
	name: "wait", class: "Act", usage: "wait [<answer|clt|brew>] [--timeout 90s] [--json]",
	about: "With no target, after bootstrap run: blocks until the setup needs you, then prints what happened since your last wait, to pass on to the person, and what to do next. " +
		"Running out of time is exit 3, with the same rows: run wait again. " +
		"With a target, for an agent driving the page itself: returns what the person chose on the current screen, or what happened to Apple's tools or Homebrew: started (clt only, when the person clicks Agree), arrived, or canceled (exit 4). Answers to earlier screens are never returned.",
	example: "bootstrap wait",
	flags:   []string{"timeout"},
	run: func(c *call) (output, error) {
		timeout := session.DefaultWaitTimeout
		if raw := c.flag("timeout"); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil || parsed <= 0 {
				return output{}, &failure{code: exitUsage, message: fmt.Sprintf("--timeout %q isn't a length of time, like 90s", raw), next: "bootstrap wait --help"}
			}
			timeout = parsed
		}
		if len(c.args) == 0 {
			return waitFlow(c, timeout)
		}
		target := c.args[0]
		if len(c.args) > 1 {
			return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unexpected argument %q", c.args[1]), next: "bootstrap wait --help"}
		}
		_, isArrival := arrivalTargets[target]
		if target != "answer" && !isArrival {
			return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unknown wait target %q", target), valid: waitTargets, next: "bootstrap wait --help"}
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		if isArrival {
			return waitArrival(c, store, target, timeout)
		}
		return waitAnswer(store, timeout)
	},
}

func waitArrival(c *call, store session.Store, target string, timeout time.Duration) (output, error) {
	probe := detect.Probe{Root: c.env.Getenv(envRoot), Home: c.env.Getenv("HOME"), Running: detect.Running}
	toolID := arrivalTargets[target]
	got, err := store.WaitInstall(context.Background(), probe, toolID, timeout)
	if errors.Is(err, session.ErrWaitTimeout) {
		tool, _ := catalog.Find(toolID)
		return output{}, &failure{code: exitTimeout, message: stillWaiting, state: fmt.Sprintf("target=%s path=%s", target, tool.Detect[0]), next: "bootstrap wait " + target}
	}
	if err != nil {
		return output{}, err
	}
	switch got.Event {
	case session.Canceled:
		return output{}, &failure{code: exitTarget, message: "the installer closed before it finished", state: "target=" + target, next: fmt.Sprintf("bootstrap show trouble %s --reason other", toolID)}
	case session.Started:
		next := startedNext[target]
		return output{rows: []string{"started target=" + target}, next: next, value: arrivalValue{Target: target, Event: got.Event, Next: next}}, nil
	}
	next := arrivalNext[target]
	row := fmt.Sprintf("arrived target=%s %s", target, kv("path", got.Path))
	return output{rows: []string{row}, next: next, value: arrivalValue{Target: target, Event: got.Event, Path: got.Path, Next: next}}, nil
}

func waitAnswer(store session.Store, timeout time.Duration) (output, error) {
	answered, err := store.WaitAnswer(context.Background(), timeout)
	if err != nil {
		return output{}, waitAnswerFailure(store, err)
	}
	const next = "bootstrap show <screen>"
	answer := withChosen(answerOf(answered))
	return output{rows: []string{answerRow(answer)}, next: next, value: answerValue{answer, next}}, nil
}

func waitAnswerFailure(store session.Store, err error) error {
	state, _ := store.Load()
	switch {
	case errors.Is(err, session.ErrWaitTimeout):
		shown := ""
		if state.Current != nil {
			shown = fmt.Sprintf("screen=%s seq=%d", state.Current.Request.Screen, state.Current.Seq)
		}
		return &failure{code: exitTimeout, message: stillWaiting, state: shown, next: "bootstrap wait answer"}
	case errors.Is(err, session.ErrNoScreen):
		return &failure{code: exitUsage, message: "no screen is showing, so there's nothing to answer", next: "bootstrap show welcome"}
	case errors.Is(err, session.ErrNoChoices):
		id := state.Current.Request.Screen
		return &failure{code: exitUsage, message: fmt.Sprintf("the %s screen has nothing to answer", id), next: screen.NextFor(id)}
	}
	return err
}

type stopValue struct {
	Session string `json:"session"`
	PID     int    `json:"pid,omitempty"`
	Server  string `json:"server"`
}

var stopCommand = command{
	name: "stop", class: "Act", usage: "stop [--json]",
	about:   "Ends the session and waits for the server to exit. Stopping twice is harmless.",
	example: "bootstrap stop",
	run: func(c *call) (output, error) {
		if err := c.noArgs("stop"); err != nil {
			return output{}, err
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		info, infoErr := store.ReadServerInfo()
		state, err := store.Stop(time.Now())
		switch {
		case errors.Is(err, session.ErrNoSession):
			return output{}, &failure{code: exitEnviron, message: "no session", next: "bootstrap status"}
		case errors.Is(err, session.ErrEnded):
			return output{rows: []string{"already stopped"}, value: stopValue{Session: state.ID, Server: "exited"}}, nil
		case err != nil:
			return output{}, err
		}
		value := stopValue{Session: state.ID, Server: "none"}
		if infoErr == nil {
			value.PID, value.Server = info.PID, "exited"
			if !waitForExit(store, info.PID) {
				return output{}, &failure{code: exitTarget, message: fmt.Sprintf("the session ended, but the server (pid %d) is still running", info.PID), next: fmt.Sprintf("kill %d", info.PID)}
			}
		}
		row := fmt.Sprintf("stopped session=%s server=%s", state.ID, value.Server)
		if value.PID != 0 {
			row = fmt.Sprintf("stopped session=%s pid=%d server=%s", state.ID, value.PID, value.Server)
		}
		return output{rows: []string{row}, value: value}, nil
	},
}

func waitForExit(store session.Store, pid int) bool {
	for deadline := time.Now().Add(stopWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if !store.ServerRunning() && syscall.Kill(pid, 0) != nil {
			return true
		}
	}
	return false
}

// runChild is the background server, launched by serve. Its one report goes to
// stdout, which is a pipe to the serve that started it.
func runChild(args []string, env Env) int {
	c, err := parse(command{name: hiddenVerb, flags: []string{"agent", "port"}, switches: []string{strings.TrimPrefix(server.FlowArg, "--")}}, args, env)
	if err != nil {
		return exitUsage
	}
	store, err := c.store()
	if err != nil {
		return exitEnviron
	}
	agent, err := screen.FindAgent(c.flag("agent"))
	if err != nil {
		return exitUsage
	}
	port, err := strconv.Atoi(c.flag("port"))
	if err != nil {
		return exitUsage
	}
	opts := server.ChildOptions{Store: store, Agent: agent, Port: port, Page: page.Files()}
	if c.switched(strings.TrimPrefix(server.FlowArg, "--")) {
		f, err := c.newFlow(store)
		if err != nil {
			return exitEnviron
		}
		opts.Flow = f.Run
	}
	server.RunChild(opts, os.Stdout)
	return exitOK
}
