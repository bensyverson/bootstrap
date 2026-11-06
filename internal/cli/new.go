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

// offerFlags are the switches that take an offer (--<name>) or turn it down
// (--no-<name>). Either wins over an answer kept from an earlier run.
var offerFlags = []struct {
	name  string
	offer flow.Offer
}{
	{"git", flow.OfferGit}, {"github", flow.OfferGitHub}, {"identity", flow.OfferIdentity},
	{"xcode", flow.OfferXcode}, {"live", flow.OfferLive}, {"settings", flow.OfferSettings},
}

// questionFlags are the flags that answer each question new can ask, for the
// refusal that lists what's still open.
var questionFlags = map[screen.ID]string{
	"list":     "--skip <tool>",
	"git-ask":  "--git|--no-git",
	"github":   "--github|--no-github",
	"identity": "--identity|--no-identity|--git-name <name> --git-email <email>",
	"making":   "--making <website|app|code|other>",
	"xcode":    "--xcode|--no-xcode",
	"live-yet": "--live|--no-live",
	"settings": "--settings|--no-settings",
}

// openQuestion is a question no flag or earlier answer settles, and the flags that would.
type openQuestion struct {
	Screen screen.ID `json:"screen"`
	Title  string    `json:"title,omitempty"`
	Answer string    `json:"answer"`
}

// stepFlags are the switches that go on without a step, for a handoff that offers skipping one.
var stepFlags = map[session.StepID]string{
	flow.StepGit: "--no-git", flow.StepGitHub: "--no-github", flow.StepIdentity: "--no-identity", flow.StepSettings: "--no-settings",
}

// givenFlags are the flags that give each answer up front.
var givenFlags = map[flow.GivenField]string{
	flow.GivenName: "<name>", flow.GivenSkip: "--skip", flow.GivenMaking: "--making",
	flow.GivenGitName: "--git-name", flow.GivenGitEmail: "--git-email", flow.GivenIn: "--in",
}

var newCommand = command{
	name: "new", class: "Act", usage: "new [<name>] [--in <folder>] [--yes] [--skip <tool>] [--making <website|app|code|other>] [--[no-]git] [--[no-]github] [--[no-]identity] [--git-name <name> --git-email <email>] [--[no-]xcode] [--[no-]live] [--[no-]settings] [--agent <claude-code|opencode>] [--json]",
	about: `Runs the whole setup here, in the foreground, with no page, server or browser. In a folder that's already a project (a Git repo), it sets that folder up and takes no name or --in; anywhere else, <name> makes <folder>/<name>, in ~/Projects unless --in or an earlier pick says otherwise.
At a terminal, it asks each open question with its default ready, so Enter takes it. --yes takes every default without asking. With no terminal (an agent) or --json, and no --yes, it refuses with exit 2 before touching anything, listing each question no flag or earlier answer settles and the flag that answers it. A question with no default (Git's name and email, when GitHub can't supply them) fails with exit 2, naming the flag.
Every offer is on unless a flag says no:
  --in <folder>        where the new project goes, such as ~/git; remembered for the next
                       setup, on the page or here (~/.bootstrap/preferences.json)
  --skip <tool>        go without a tool (repeat, or use commas); every tool is kept by default
  --making <kind>      website, app, code or other; by default package.json means website,
                       Package.swift or an .xcodeproj means app, and anything else is other
  --git, --no-git      turn on Git, or don't
  --github, --no-github
                       sign in to GitHub, or don't
  --identity, --no-identity
                       set the name and email Git signs your work with, or don't; they come
                       from GitHub, or from --git-name and --git-email
  --xcode, --no-xcode  for an app, open Xcode in the App Store, or don't
  --live, --no-live    people already use it, or not yet (the default)
  --settings, --no-settings
                       turn on the agent's helpers, or don't
  --agent <app>        the app the rules and helpers are for: from the environment, else claude-code
Steps that need a click (Apple's window, Homebrew's installer, GitHub's code) say what to do and wait. Asking about the phone is the page's; new prints a hint instead.
Trouble prints what happened and the evidence, as wait does, and exits 4: running new again picks up where it stopped, since every step checks before it acts. Answers are kept for the project, so a run again asks nothing already answered; a flag given now wins over an earlier answer, so --git takes back an earlier --no-git. A flag and its --no- together are a usage error.
While a page setup (bootstrap run or serve) is running on this Mac, new refuses with exit 3, so two setups never install at once.
The log is ~/.bootstrap/new/<project>-<hash>/session.jsonl, apart from the page's.`,
	example:  "bootstrap new pine-bakery --yes --in ~/git --skip woodcase",
	flags:    []string{"agent", "making", "git-name", "git-email", "in"},
	lists:    []string{"skip"},
	switches: []string{"yes", "git", "no-git", "github", "no-github", "identity", "no-identity", "xcode", "no-xcode", "live", "no-live", "settings", "no-settings"},
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
	asJSON, yes := c.switched("json"), c.switched("yes")
	// A terminal decides only between asking and refusing, never between asking and installing.
	interactive := c.env.Terminal != nil && c.env.Terminal() && !yes && !asJSON
	answers := refuseQuestions
	switch {
	case interactive:
		answers = askPerson
	case yes:
		answers = takeDefaults
	}
	given := flow.Given{Skip: c.lists["skip"], Making: c.flag("making"), GitName: c.flag("git-name"), GitEmail: c.flag("git-email")}
	for _, o := range offerFlags {
		take, decline := c.switched(o.name), c.switched("no-"+o.name)
		switch {
		case take && decline:
			return output{}, &failure{code: exitUsage, message: fmt.Sprintf("--%s and --no-%s say opposite things", o.name, o.name), next: newHelp}
		case take:
			given.Accepted = append(given.Accepted, o.offer)
		case decline:
			given.Declined = append(given.Declined, o.offer)
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
	_, flagged := c.options["in"]
	in, err := c.projectsIn(cfg)
	if err != nil {
		return output{}, err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	t := newTerminal(c, answers, asJSON)
	dir, inPlace, err := where(ctx, t, cfg.Probe, agent, name, in)
	if err != nil {
		return output{}, err
	}
	if inPlace && flagged {
		return output{}, whereFailure(flow.ErrInProject)
	}
	again := "bootstrap new"
	if !inPlace {
		given.Name, given.In = filepath.Base(dir), in
		again += " " + given.Name
		if flagged {
			again += " --in " + in
		}
	}
	store := session.Store{Dir: filepath.Join(base.Dir, "new", projectKey(dir)), Bell: session.NewBell()}
	cfg.Store, cfg.Given, cfg.Person = store, given, t
	f, err := flow.New(cfg)
	if err != nil {
		return output{}, givenFailure(err)
	}
	if base.ServerRunning() {
		return output{}, &failure{code: exitTimeout, message: "a setup is running on the page on this Mac, and two setups can't install at once",
			next: "bootstrap wait until it's done, or bootstrap stop to end it; then " + again}
	}
	if !yes && !interactive {
		if err := refuseOpen(f, agent, again); err != nil {
			return output{}, err
		}
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

// refuseOpen refuses a run with no one to ask and no --yes, when any question is still open.
func refuseOpen(f *flow.Flow, agent screen.Agent, again string) error {
	ids, err := f.Unanswered()
	if err != nil || len(ids) == 0 {
		return err
	}
	var open []openQuestion
	var rows []string
	for _, id := range ids {
		q := openQuestion{Screen: id, Answer: questionFlags[id]}
		row := "question " + string(id)
		// A screen whose words need what the run learns, such as GitHub's name, has no title yet.
		if desc, err := screen.Describe(screen.Request{Screen: id}, screen.Context{Agent: agent}); err == nil {
			q.Title = spans(desc.Title)
			row += " “" + q.Title + "”"
		}
		open = append(open, q)
		rows = append(rows, row+" "+kv("answer", q.Answer))
	}
	noun := "questions"
	if len(open) == 1 {
		noun = "question"
	}
	return &failure{code: exitUsage, questions: open, state: strings.Join(rows, "\n"),
		message: fmt.Sprintf("no one is at a terminal to answer %d %s, and there's no --yes to take the defaults", len(open), noun),
		next:    again + " --yes takes the defaults; or answer each with its flag, or run new at a terminal"}
}

// projectsIn is where a new project goes: --in, else the folder picked last
// time, else "" for ~/Projects. A folder under home is written ~/….
func (c *call) projectsIn(cfg flow.Config) (string, error) {
	raw, flagged := c.options["in"]
	if !flagged {
		p, err := cfg.Preferences.Load()
		if err != nil {
			return "", &failure{code: exitEnviron, message: err.Error(), next: "fix or delete " + cfg.Preferences.Path() + ", then bootstrap new"}
		}
		return p.ProjectsIn, nil
	}
	home := cfg.Probe.Home
	path := raw
	switch {
	case path == "~" || strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	case !filepath.IsAbs(path):
		path = filepath.Join(cfg.Probe.Cwd, path)
	}
	in := flow.Place(home, filepath.Clean(path))
	if !screen.SafePlace(in) {
		// Name it as typed, so the person recognizes it.
		return "", &failure{code: exitUsage, message: fmt.Sprintf("--in: %q can't hold projects. Use ~/<folder> or a full path, with letters, numbers and dashes", raw), next: newHelp}
	}
	return in, nil
}

// where is the project folder, asking for a name at a terminal when the setup needs one.
func where(ctx context.Context, t *terminal, probe detect.Probe, agent screen.Agent, name, in string) (string, bool, error) {
	dir, inPlace, err := flow.Where(probe, name, in)
	if !errors.Is(err, flow.ErrNeedsName) || t.answering != askPerson {
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
		dir, inPlace, err = flow.Where(probe, a.Name, in)
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
