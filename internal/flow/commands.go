package flow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/session"
)

// tailLines is how much of a command's output a handoff carries.
const tailLines = 20

// exec runs a command, keeps its output in the step's log, and returns a handoff
// for any result outside expect (exit 0 when none is given). tool is the catalog
// tool the command is for, if any, which decides whether skipping is offered.
func (r *run) exec(cmd Command, tool string, expect ...int) (Result, *session.Handoff, error) {
	if cmd.Timeout == 0 {
		cmd.Timeout = r.f.cfg.Limits.Command
	}
	if len(expect) == 0 {
		expect = []int{0}
	}
	began := time.Now()
	res := r.f.cfg.Runner.Run(r.ctx, cmd)
	if err := r.ctx.Err(); err != nil {
		return res, nil, err
	}
	log, err := r.keep(cmd, tool, res)
	if err != nil {
		return res, nil, err
	}
	evidence := session.Evidence{Command: cmd.String(), Exit: res.Exit, Elapsed: time.Since(began).Round(time.Millisecond), Log: log, Tail: tail(res.Output)}
	h := func(reason session.HandoffReason, what string) *session.Handoff {
		trouble := r.trouble(reason, tool, what)
		trouble.Evidence = evidence
		return &trouble
	}
	switch {
	case res.Err != nil:
		return res, h(session.ReasonNoStart, fmt.Sprintf("%s couldn't start: %v", cmd, res.Err)), nil
	case res.TimedOut:
		evidence.Limit = cmd.Timeout
		return res, h(session.ReasonTimeout, fmt.Sprintf("%s ran past its %v limit", cmd, cmd.Timeout)), nil
	case !slices.Contains(expect, res.Exit):
		return res, h(session.ReasonExit, fmt.Sprintf("%s exited %d", cmd, res.Exit)), nil
	}
	return res, nil, nil
}

// keep appends a command and its output to the step's log, named for the tool or the step.
func (r *run) keep(cmd Command, tool string, res Result) (string, error) {
	name := tool
	if name == "" {
		name = string(r.def.ID)
	}
	path := filepath.Join(r.f.logs(), name+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fmt.Fprintf(f, "$ %s\n%s", cmd, res.Output)
	if len(res.Output) > 0 && !bytes.HasSuffix(res.Output, []byte("\n")) {
		fmt.Fprintln(f)
	}
	if res.Err != nil {
		fmt.Fprintf(f, "(couldn't start: %v)\n", res.Err)
	}
	fmt.Fprintf(f, "(exit %d)\n", res.Exit)
	return path, nil
}

func tail(output []byte) []string {
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines[max(0, len(lines)-tailLines):]
}

// trouble is a handoff for this step. Skipping is offered only for a tool the
// person may go without, or, with no tool, a step they may.
func (r *run) trouble(reason session.HandoffReason, tool, what string) session.Handoff {
	skippable := r.def.Skippable
	if t, ok := catalog.Find(tool); ok {
		skippable = !t.Required
	}
	ways := []session.Way{session.WayRetry}
	if skippable {
		ways = append(ways, session.WaySkip)
	}
	ways = append(ways, session.WayTalk)
	return session.Handoff{Kind: session.HandoffTrouble, Step: r.def.ID, Tool: tool, Reason: reason, What: what, Ways: ways}
}

// limitPassed is a handoff for a wait that ran out of time.
func (r *run) limitPassed(tool, what string, limit time.Duration) session.Handoff {
	h := r.trouble(session.ReasonTimeout, tool, what)
	h.Evidence = session.Evidence{Limit: limit}
	return h
}

func joinList(ids []string) string { return strings.Join(ids, ", ") }

// tilde is a path the way a person reads it.
func (r *run) tilde(path string) string {
	if rest, ok := strings.CutPrefix(path, r.f.cfg.Probe.Home+"/"); ok {
		return "~/" + rest
	}
	return path
}
