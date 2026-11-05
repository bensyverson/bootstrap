package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

const primer = `bootstrap sets up a new Mac with a coding agent. The agent runs these commands
and does every install. A local web page shows the person what's happening and
collects their choices.

Run first: bootstrap status

Read (needs no session, except status)
  bootstrap detect                 the chip, macOS, this folder, what's installed
  bootstrap tools                  what each tool is, and whether it can be skipped
  bootstrap screens                every screen, with its statuses and reasons
  bootstrap status                 the session: the current screen and every answer

Act
  bootstrap serve [--agent <app>]  start the page in the background and open it
  bootstrap show <screen> [<tool>] [--status <s>] [--reason <r>] [--expect 20m]
                                   move the page to a screen
  bootstrap wait <answer|clt|brew> [--timeout 90s]
                                   return the answer or the arrival
  bootstrap stop                   end the session

Every command takes --json for a stable shape, and --help for an example.

Exit codes: 0 ok, 2 usage, 3 timeout (run wait again), 4 target failure,
5 environment (no session, or the page can't start).
`

var detectCommand = command{
	name: "detect", class: "Read", usage: "detect [--json]",
	about:   "Reports the chip, macOS version, this folder's kind, and what's installed. It only reports.",
	example: "bootstrap detect",
	run: func(c *call) (output, error) {
		if err := c.noArgs("detect"); err != nil {
			return output{}, err
		}
		c.renew()
		cwd, err := c.env.Cwd()
		if err != nil {
			return output{}, &failure{code: exitEnviron, message: "can't read this folder: " + err.Error(), next: "cd into a folder, then bootstrap detect"}
		}
		r := detect.Run(detect.Probe{Root: c.env.Getenv(envRoot), Home: c.env.Getenv("HOME"), Cwd: cwd, Chip: detect.Chip})
		rows := []string{
			strings.Join([]string{kv("chip", r.Chip), kv("arch", r.Arch), kv("macos", r.MacOS)}, " "),
			fmt.Sprintf("folder kind=%s files=%s git=%s %s", r.Folder.Kind, r.Folder.Files, r.Folder.Git, kv("path", r.Folder.Path)),
		}
		for _, t := range r.Tools {
			row := fmt.Sprintf("tool %s %s", t.ID, t.State)
			if t.Path != "" {
				row += " " + kv("path", t.Path)
			}
			rows = append(rows, row)
		}
		return output{rows: rows, next: "bootstrap tools", value: r}, nil
	},
}

type toolRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Say      string `json:"say"`
	Required bool   `json:"required"`
	OwnStep  bool   `json:"own_step"`
}

var toolsCommand = command{
	name: "tools", class: "Read", usage: "tools [--agent <app>] [--json]",
	about:   "Lists what each tool is, why it's there and whether it can be skipped. Own-step tools have their own screens instead of show tool.",
	example: "bootstrap tools --agent claude-code",
	flags:   []string{"agent"},
	run: func(c *call) (output, error) {
		if err := c.noArgs("tools"); err != nil {
			return output{}, err
		}
		agent, known, err := c.agent()
		if err != nil {
			return output{}, err
		}
		c.renew()
		var rows []string
		var value []toolRow
		for _, t := range catalog.Tools() {
			say := t.Say
			if known {
				say = agent.Fill(say)
			}
			need, step := "skippable", "tool"
			if t.Required {
				need = "required"
			}
			if t.OwnStep {
				step = "own"
			}
			rows = append(rows, fmt.Sprintf("%s %s step=%s %s %s %s", t.ID, need, step, kv("name", t.Name), kv("url", t.URL), kv("say", say)))
			value = append(value, toolRow{ID: t.ID, Name: t.Name, URL: t.URL, Say: say, Required: t.Required, OwnStep: t.OwnStep})
		}
		return output{rows: rows, next: "bootstrap screens", value: struct {
			Tools []toolRow `json:"tools"`
		}{value}}, nil
	},
}

var screensCommand = command{
	name: "screens", class: "Read", usage: "screens [--json]",
	about:   "Lists every screen, with the tools, statuses and reasons each one takes, what a person can choose, and what to run after showing it.",
	example: "bootstrap screens",
	run: func(c *call) (output, error) {
		if err := c.noArgs("screens"); err != nil {
			return output{}, err
		}
		c.renew()
		var rows []string
		defs := screen.Definitions()
		for _, d := range defs {
			cells := []string{string(d.Screen), "mode=" + string(d.Mode)}
			for _, list := range []struct {
				key    string
				values []string
			}{{"tools", d.Tools}, {"statuses", d.Statuses}, {"reasons", d.Reasons}, {"choices", d.Choices}} {
				if len(list.values) > 0 {
					cells = append(cells, list.key+"="+strings.Join(list.values, ","))
				}
			}
			if d.Internal {
				cells = append(cells, "appears=on-its-own")
			} else {
				cells = append(cells, kv("next", d.Next))
			}
			rows = append(rows, strings.Join(cells, " "))
		}
		return output{rows: rows, next: "bootstrap serve", value: struct {
			Screens []screen.Definition `json:"screens"`
		}{defs}}, nil
	},
}

type statusScreen struct {
	Screen   screen.ID   `json:"screen"`
	Tool     string      `json:"tool,omitempty"`
	Status   string      `json:"status,omitempty"`
	Reason   string      `json:"reason,omitempty"`
	Seq      int         `json:"seq"`
	Mode     screen.Mode `json:"mode"`
	Progress float64     `json:"progress"`
}

type statusAnswer struct {
	Seq    int       `json:"seq"`
	Screen screen.ID `json:"screen"`
	Tool   string    `json:"tool,omitempty"`
	Choice string    `json:"choice"`
	Tools  []string  `json:"tools,omitempty"`
	Name   string    `json:"name,omitempty"`
}

type statusValue struct {
	Session string         `json:"session"`
	Agent   string         `json:"agent"`
	Page    string         `json:"page"`
	Tabs    int            `json:"tabs"`
	Lost    bool           `json:"lost"`
	Screen  *statusScreen  `json:"screen"`
	Answers []statusAnswer `json:"answers"`
	Chosen  []string       `json:"chosen,omitempty"`
	Next    string         `json:"next"`
}

func answerOf(a session.Answered) statusAnswer {
	return statusAnswer{Seq: a.Seq, Screen: a.Request.Screen, Tool: a.Request.Tool, Choice: a.Answer.Choice, Tools: a.Answer.Tools, Name: a.Answer.Name}
}

// answerRow is the row status and wait answer share. Chosen tools are spelled out
// when the answer set them, so the agent never has to work out what was kept.
func answerRow(a statusAnswer) string {
	cells := []string{"answer", fmt.Sprintf("seq=%d", a.Seq), kv("screen", string(a.Screen))}
	if a.Tool != "" {
		cells = append(cells, kv("tool", a.Tool))
	}
	cells = append(cells, kv("choice", a.Choice))
	if a.Tools != nil {
		cells = append(cells, "tools="+strings.Join(a.Tools, ","))
	}
	if a.Name != "" {
		cells = append(cells, kv("name", a.Name))
	}
	return strings.Join(cells, " ")
}

// withChosen fills in the tools a list answer really kept.
func withChosen(a statusAnswer) statusAnswer {
	if a.Screen == "list" {
		a.Tools = screen.KeepRequired(a.Tools)
	}
	return a
}

func nextAfter(state session.State, running bool) string {
	switch {
	case !running:
		return "bootstrap serve"
	case state.Current == nil:
		return "bootstrap show welcome"
	}
	if _, answered := state.CurrentAnswer(); answered {
		return "bootstrap show <screen>"
	}
	return screen.NextFor(state.Current.Request.Screen)
}

var statusCommand = command{
	name: "status", class: "Read", usage: "status [--json]",
	about:   "Reports the session: the current screen and every answer so far. A new agent session starts here. Running it brings a lost page back.",
	example: "bootstrap status",
	run: func(c *call) (output, error) {
		if err := c.noArgs("status"); err != nil {
			return output{}, err
		}
		store, err := c.store()
		if err != nil {
			return output{}, err
		}
		before, err := store.Load()
		switch {
		case errors.Is(err, session.ErrNoSession):
			return output{}, &failure{code: exitEnviron, message: "no session", next: "bootstrap detect, then bootstrap serve"}
		case err != nil:
			return output{}, err
		case before.Ended:
			return output{}, session.ErrEnded
		}
		now := time.Now()
		lost := before.Lost(now)
		if err := store.Renew(now); err != nil {
			return output{}, err
		}
		state, err := store.Load()
		if err != nil {
			return output{}, err
		}
		running := store.ServerRunning()
		page, lostWord := "closed", "no"
		if running {
			page = "running"
		}
		if lost {
			lostWord = "yes"
		}
		value := statusValue{Session: state.ID, Agent: state.Agent.ID, Page: page, Tabs: state.Tabs, Lost: lost, Answers: []statusAnswer{}, Chosen: state.Chosen, Next: nextAfter(state, running)}
		rows := []string{fmt.Sprintf("session=%s agent=%s page=%s tabs=%d lost=%s", state.ID, state.Agent.ID, page, state.Tabs, lostWord)}
		if state.Current != nil {
			req := state.Current.Request
			desc, err := screen.Describe(req, screen.Context{Agent: state.Agent, Chosen: state.Chosen, Carried: state.Progress})
			if err != nil {
				return output{}, err
			}
			value.Screen = &statusScreen{Screen: req.Screen, Tool: req.Tool, Status: req.Status, Reason: req.Reason, Seq: state.Current.Seq, Mode: desc.Mode, Progress: desc.Progress}
			rows = append(rows, screenRow("screen="+string(req.Screen)+fmt.Sprintf(" seq=%d", state.Current.Seq), req, desc))
		}
		for _, a := range state.Answers {
			answer := withChosen(answerOf(a))
			value.Answers = append(value.Answers, answer)
			rows = append(rows, answerRow(answer))
		}
		if state.Chosen != nil {
			rows = append(rows, "chosen="+strings.Join(state.Chosen, ","))
		}
		return output{rows: rows, next: value.Next, value: value}, nil
	},
}

// screenRow finishes a row about a screen, with what was asked and what the binary worked out.
func screenRow(start string, req screen.Request, desc screen.Description) string {
	cells := []string{start}
	if req.Tool != "" {
		cells = append(cells, kv("tool", req.Tool))
	}
	if req.Status != "" {
		cells = append(cells, kv("status", req.Status))
	}
	if req.Reason != "" {
		cells = append(cells, kv("reason", req.Reason))
	}
	cells = append(cells, "mode="+string(desc.Mode), "progress="+formatFloat(desc.Progress))
	return strings.Join(cells, " ")
}

func (c *call) noArgs(verb string) error {
	if len(c.args) > 0 {
		return &failure{code: exitUsage, message: fmt.Sprintf("unexpected argument %q", c.args[0]), next: "bootstrap " + verb + " --help"}
	}
	return nil
}
