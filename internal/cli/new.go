package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// bootstrap new is the setup from the command line: the same flow the page runs,
// in the foreground, with the terminal asking (project/2026-11-05-bootstrap-new-plan.md).
//
// Its log is its own, in ~/.bootstrap/new/<project>-<hash>/, one per project
// folder. The page's session.jsonl and server files are never touched, so status,
// wait and run can't mistake a run with no server for a page. One log per
// project is what lets a run again pick up: the answers and finished steps of the
// last run are there, so nothing is asked twice, and a different project starts clean.

const newHelp = "bootstrap new --help"

// declineFlags are the switches that turn an offer down, by the offer.
var declineFlags = []struct {
	name  string
	offer flow.Offer
}{
	{"no-git", flow.OfferGit}, {"no-github", flow.OfferGitHub}, {"no-identity", flow.OfferIdentity},
	{"no-xcode", flow.OfferXcode}, {"no-settings", flow.OfferSettings},
}

// stepFlags are the switches that go on without a step, for a handoff that offers skipping one.
var stepFlags = map[session.StepID]string{
	flow.StepGit: "--no-git", flow.StepGitHub: "--no-github", flow.StepIdentity: "--no-identity", flow.StepSettings: "--no-settings",
}

// givenFlags are the flags that give each answer up front.
var givenFlags = map[flow.GivenField]string{
	flow.GivenName: "<name>", flow.GivenSkip: "--skip", flow.GivenMaking: "--making",
	flow.GivenGitName: "--git-name", flow.GivenGitEmail: "--git-email",
}

var newCommand = command{
	name: "new", class: "Act", usage: "new [<name>] [--yes] [--skip <tool>] [--making <website|app|code|other>] [--live] [--no-git] [--no-github] [--no-identity] [--git-name <name> --git-email <email>] [--no-xcode] [--no-settings] [--agent <claude-code|opencode>] [--json]",
	about: `Runs the whole setup here, in the foreground, with no page, server or browser. In a folder that's already a project (a Git repo), it sets that folder up and takes no name; anywhere else, <name> makes ~/Projects/<name>.
At a terminal, it asks each open question with its default ready, so Enter takes it. With --yes, --json or no terminal (an agent), it takes every default without asking, and a question with no default fails with exit 2, naming the flag that answers it.
Every offer is on unless a flag says no:
  --skip <tool>        go without a tool (repeat, or use commas); every tool is kept by default
  --making <kind>      website, app, code or other; by default package.json means website,
                       Package.swift or an .xcodeproj means app, and anything else is other
  --live               people already use it (the default is not yet)
  --no-git             don't turn on Git
  --no-github          don't sign in to GitHub
  --no-identity        don't set the name and email Git signs your work with; by default
                       they come from GitHub, or from --git-name and --git-email
  --no-xcode           for an app, don't open Xcode in the App Store
  --no-settings        don't turn on the agent's helpers
  --agent <app>        the app the rules and helpers are for: from the environment, else claude-code
Steps that need a click (Apple's window, Homebrew's installer, GitHub's code) say what to do and wait. Asking about the phone is the page's; new prints a hint instead.
Trouble prints what happened and the evidence, as wait does, and exits 4: running new again picks up where it stopped, since every step checks before it acts. Answers are kept for the project, so a run again asks nothing already answered; a flag given now wins over an earlier answer.
The log is ~/.bootstrap/new/<project>-<hash>/session.jsonl, apart from the page's.`,
	example:  "bootstrap new pine-bakery --yes --skip woodcase",
	flags:    []string{"agent", "making", "git-name", "git-email"},
	lists:    []string{"skip"},
	switches: []string{"yes", "live", "no-git", "no-github", "no-identity", "no-xcode", "no-settings"},
	run:      runNew,
}

type newValue struct {
	Project string           `json:"project"`
	Log     string           `json:"log"`
	Updates []session.Update `json:"updates"`
	Next    string           `json:"next"`
}

func runNew(c *call) (output, error) {
	if len(c.args) > 1 {
		return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unexpected argument %q", c.args[1]), next: newHelp}
	}
	name := ""
	if len(c.args) == 1 {
		name = c.args[0]
	}
	agent, known, err := c.agent()
	if err != nil {
		return output{}, err
	}
	if !known {
		agent = screen.ClaudeCode
	}
	asJSON := c.switched("json")
	interactive := c.env.Terminal != nil && c.env.Terminal() && !c.switched("yes") && !asJSON
	given := flow.Given{Skip: c.lists["skip"], Making: c.flag("making"), Live: c.switched("live"), GitName: c.flag("git-name"), GitEmail: c.flag("git-email")}
	for _, d := range declineFlags {
		if c.switched(d.name) {
			given.Declined = append(given.Declined, d.offer)
		}
	}
	base, err := c.store()
	if err != nil {
		return output{}, err
	}
	cfg, err := c.flowConfig(base)
	if err != nil {
		return output{}, err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t := newTerminal(c, interactive, asJSON)
	dir, inPlace, err := where(ctx, t, cfg.Probe, agent, name)
	if err != nil {
		return output{}, err
	}
	again := "bootstrap new"
	if !inPlace {
		given.Name = filepath.Base(dir)
		again += " " + given.Name
	}
	store := session.Store{Dir: filepath.Join(base.Dir, "new", projectKey(dir)), Bell: session.NewBell()}
	cfg.Store, cfg.Given, cfg.Person = store, given, t
	f, err := flow.New(cfg)
	if err != nil {
		return output{}, givenFailure(err)
	}
	release, err := store.LockServer()
	if errors.Is(err, session.ErrServerRunning) {
		return output{}, &failure{code: exitTimeout, message: "another bootstrap new is setting up this project", next: "wait for it to finish, then " + again}
	}
	if err != nil {
		return output{}, err
	}
	defer release()
	if _, err := store.Begin(agent, os.Getpid(), time.Now()); err != nil {
		return output{}, err
	}
	t.attach(store)
	shown := tilde(dir, c.env.Getenv("HOME"))
	t.print(fmt.Sprintf("project %s %s", kv("path", shown), kv("log", store.LogPath())))

	watching, quit := context.WithCancel(ctx)
	go t.watch(watching)
	runErr := f.Run(ctx)
	quit()
	t.mu.Lock()
	t.flush()
	updates := nonNil(t.updates)
	t.mu.Unlock()
	if runErr != nil {
		fail := newFailure(runErr, again)
		fail.updates = updates
		return output{}, fail
	}
	next := fmt.Sprintf("open a new Terminal window, cd %s, and start %s there", shown, agent.App)
	return output{next: next, value: newValue{Project: dir, Log: store.LogPath(), Updates: updates, Next: next}}, nil
}

// where is the project folder, asking for a name at a terminal when the setup needs one.
func where(ctx context.Context, t *terminal, probe detect.Probe, agent screen.Agent, name string) (string, bool, error) {
	dir, inPlace, err := flow.Where(probe, name)
	if !errors.Is(err, flow.ErrNeedsName) || !t.interactive {
		return dir, inPlace, whereFailure(err)
	}
	req := screen.Request{Screen: "naming"}
	desc, err := screen.Describe(req, screen.Context{Agent: agent})
	if err != nil {
		return "", false, err
	}
	q := flow.Question{Request: req, Screen: desc}
	for {
		a, err := t.Ask(ctx, q)
		if err != nil {
			return "", false, newFailure(err, "bootstrap new")
		}
		dir, inPlace, err = flow.Where(probe, a.Name)
		if _, bad := errors.AsType[*flow.GivenError](err); bad {
			q.Refused = err
			continue
		}
		return dir, inPlace, whereFailure(err)
	}
}

func whereFailure(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, flow.ErrNeedsName):
		return &failure{code: exitUsage, message: "this folder isn't a project, so new needs a name for one", next: "bootstrap new <name>"}
	case errors.Is(err, flow.ErrInProject):
		return &failure{code: exitUsage, message: err.Error(), next: "bootstrap new with no name sets up this folder; to make a new project, cd out of it first"}
	}
	return givenFailure(err)
}

// givenFailure names the flag behind an answer given up front that won't do.
func givenFailure(err error) error {
	bad, ok := errors.AsType[*flow.GivenError](err)
	if !ok {
		return err
	}
	message := bad.Message
	if flag := givenFlags[bad.Field]; flag != "" && bad.Field != flow.GivenName {
		message = flag + ": " + message
	}
	return &failure{code: exitUsage, message: message, valid: bad.Valid, next: newHelp}
}

// newFailure is why a run stopped, with how to go on.
func newFailure(err error, again string) *failure {
	if handed, ok := errors.AsType[*flow.HandedOff](err); ok {
		return &failure{code: exitTarget, message: handed.Error(), next: waysOn(handed.Handoff, again)}
	}
	if u, ok := isUnanswered(err); ok {
		next := again + " at a terminal, to answer it"
		switch u.q.Request.Screen {
		case "naming":
			next = "bootstrap new <name>"
		case "identity-name", "identity-email":
			next = again + " --git-name <name> --git-email <email>, or add --no-identity"
		}
		return &failure{code: exitUsage, message: u.Error(), next: next}
	}
	if errors.Is(err, context.Canceled) {
		return &failure{code: exitTimeout, message: "stopped before the end", next: again + " picks up where this left off"}
	}
	return asFailure(err)
}

// waysOn is a handoff's ways forward, as runs of new.
func waysOn(h session.Handoff, again string) string {
	var out []string
	for _, w := range h.Ways {
		switch w {
		case session.WayRetry:
			look := "look into what happened"
			if h.Evidence.Log != "" {
				look = "read the log"
			}
			out = append(out, look+", fix what you can, then "+again)
		case session.WaySkip:
			if h.Tool != "" {
				out = append(out, fmt.Sprintf("or, to go on without %s: %s --skip %s", h.Tool, again, h.Tool))
			} else if flag, ok := stepFlags[h.Step]; ok {
				out = append(out, fmt.Sprintf("or, to go on without this step: %s %s", again, flag))
			}
		}
	}
	return strings.Join(out, "; ")
}

// projectKey names a project's log folder: readable, and apart from any other folder of the same name.
func projectKey(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return filepath.Base(dir) + "-" + hex.EncodeToString(sum[:4])
}

func tilde(path, home string) string {
	if rest, ok := strings.CutPrefix(path, home+"/"); ok && home != "" {
		return "~/" + rest
	}
	return path
}
