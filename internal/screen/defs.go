package screen

import (
	"embed"
	"slices"

	"github.com/bensyverson/bootstrap/catalog"
)

// Samples holds one JSON description per screen and agent, for the page's preview route and tests.
//
//go:embed samples/*.json
var Samples embed.FS

// Lost is shown by the server on its own, never by an agent.
const Lost ID = "lost"

// Words the lost screen and off-network trouble use. They live together so the
// agent trial can change them in one place.
const (
	LostTitle        = "Whoops! We lost your {app} session."
	LostLine         = "Open it back up and type “continue”."
	OtherFailureLine = "Something went wrong. {ai} can take a look."
)

const brewCommand = `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`

// Next steps the ack on a screen teaches.
const (
	nextAnswer = "bootstrap wait answer"
	nextCLT    = "bootstrap wait clt"
	nextBrew   = "bootstrap wait brew"
	nextWork   = "do the work, then bootstrap show <screen>"
	nextStop   = "bootstrap stop"
)

// option is one value in a closed list, with the words it puts on the screen.
type option struct {
	id   string
	text string
}

type choiceDef struct {
	id, label string
}

// toolArg says which tools a screen takes.
type toolArg int

const (
	noTool toolArg = iota
	// installTool is a tool with its own "Adding" screen. Apple's tools,
	// Homebrew and Git have their own steps instead.
	installTool
	anyTool
)

// Definition is a screen's row in the closed list: what an agent may ask of it.
type Definition struct {
	Screen   ID       `json:"screen"`
	Mode     Mode     `json:"mode"`
	Tools    []string `json:"tools,omitempty"`
	Statuses []string `json:"statuses,omitempty"`
	Reasons  []string `json:"reasons,omitempty"`
	Choices  []string `json:"choices,omitempty"`
	Next     string   `json:"next,omitempty"`
	Internal bool     `json:"internal,omitempty"`
}

// spec is one screen's words and rules. In text, {link:id} is a tool's linked
// name and {tool} its plain name.
type spec struct {
	id       ID
	mode     Mode
	layout   Layout
	title    string
	note     string
	noteTool string
	status   string
	statuses []option
	reasons  []option
	tool     toolArg
	choices  []choiceDef
	rows     bool
	code     string
	field    *Field
	card     string
	cardTool bool
	next     string
	internal bool
}

var specs = []spec{
	{id: "welcome", mode: Waiting, title: "Let’s give {app} the good stuff.",
		choices: []choiceDef{{"see-plan", "See the plan"}, {"skip-tour", "Skip the tour"}}, next: nextAnswer},
	{id: "list", mode: Waiting, layout: LayoutOver, title: "Here’s what you get.", rows: true,
		choices: []choiceDef{{"lets-go", "Let’s go"}}, next: nextAnswer},
	{id: "fast", mode: Working, title: "Got it. We’ll only stop if we need you.", next: nextWork},
	{id: "apple-ask", mode: Waiting, title: "Your turn: click Install.",
		status: "A window from Apple will pop up. It adds the {link:clt}, which include Git. This can take a while.", next: nextCLT},
	{id: "apple-wait", mode: Working, title: "Adding Apple’s tools", noteTool: "clt",
		status: "Apple’s window shows how far along it is. This can take a while.", card: "git", next: nextCLT},
	{id: "terminal", mode: Waiting, layout: LayoutLow, title: "Your turn: paste this in Terminal.",
		status: "This installs {link:homebrew}, which installs other software for you. It will ask for your Mac password, and you won’t see it as you type. That’s normal.",
		code:   brewCommand, next: nextBrew},
	{id: "brew-wait", mode: Working, title: "Adding {link:homebrew}", noteTool: "homebrew",
		status: "Follow along in Terminal.", card: "homebrew", next: nextBrew},
	{id: "tool", mode: Working, title: "Adding {tool-link}", tool: installTool, cardTool: true,
		statuses: []option{{"downloading", "Downloading {tool}"}, {"installing", "Installing {tool}"}}, next: nextWork},
	{id: "making", mode: Waiting, title: "What are you working on?",
		choices: []choiceDef{{"website", "A website"}, {"app", "A Mac or iPhone app"}, {"code", "Some code"}, {"other", "Something else"}}, next: nextAnswer},
	{id: "xcode", mode: Waiting, title: "This one needs Xcode.", status: "It’s free, and it’s big. The App Store will take a while.",
		choices: []choiceDef{{"app-store", "Open the App Store"}, {"later", "Do it later"}}, next: nextAnswer},
	{id: "live-yet", mode: Waiting, title: "Do people use it yet?",
		choices: []choiceDef{{"not-yet", "Not yet"}, {"yes", "Yes"}}, next: nextAnswer},
	{id: "naming", mode: Waiting, layout: LayoutLow, title: "What should we call this project?", status: "We’ll make a folder with this name.",
		field: &Field{Label: "Project name", Value: "my-project"}, choices: []choiceDef{{"next", "Next"}}, next: nextAnswer},
	{id: "building", mode: Working, title: "Setting up your project",
		statuses: []option{{"folder", "Making your folder"}, {"git", "Turning on Git"}, {"rules", "Teaching {ai} how you work"}}, next: nextWork},
	{id: "github", mode: Waiting, title: "Want a place to keep your work online?", status: "GitHub is free. You can always do this later.",
		choices: []choiceDef{{"yes", "Yes, help me"}, {"not-now", "Not now"}}, next: nextAnswer},
	{id: "phone", mode: Waiting, title: "Want to use {ai} from your phone?",
		choices: []choiceDef{{"yes", "Show me how"}, {"not-now", "Not now"}}, next: nextAnswer},
	{id: "done", mode: Settled, title: "All set! You can close this and go back to {app}.", next: nextStop},
	{id: "trouble", mode: Waiting, title: "{tool} didn’t install.", tool: anyTool,
		reasons: []option{{"network", "Your Mac couldn’t reach the download. Check your Wi-Fi."}, {"other", OtherFailureLine}},
		choices: []choiceDef{{"retry", "Try again"}, {"skip", "Skip {tool}"}, {"ask", "Ask {ai}"}}, next: nextAnswer},
	{id: "fixing", mode: Working, title: "{ai} is on it.", status: "Looking at what went wrong", next: nextWork},
	{id: Lost, mode: Waiting, title: LostTitle, status: LostLine, internal: true},
}

func find(id ID) (spec, bool) {
	for _, s := range specs {
		if s.id == id {
			return s, true
		}
	}
	return spec{}, false
}

func optionIDs(options []option) []string {
	var ids []string
	for _, o := range options {
		ids = append(ids, o.id)
	}
	return ids
}

func (s spec) toolIDs() []string {
	switch s.tool {
	case installTool:
		var ids []string
		for _, t := range catalog.Tools() {
			if !t.OwnStep {
				ids = append(ids, t.ID)
			}
		}
		return ids
	case anyTool:
		return catalog.IDs()
	}
	return nil
}

// Definitions lists every screen, in the order the path meets them.
func Definitions() []Definition {
	var out []Definition
	for _, s := range specs {
		def := Definition{Screen: s.id, Mode: s.mode, Tools: s.toolIDs(), Statuses: optionIDs(s.statuses), Reasons: optionIDs(s.reasons), Next: s.next, Internal: s.internal}
		for _, c := range s.choices {
			def.Choices = append(def.Choices, c.id)
		}
		out = append(out, def)
	}
	return out
}

// ScreenIDs lists the screens an agent may show.
func ScreenIDs() []string {
	var ids []string
	for _, s := range specs {
		if !s.internal {
			ids = append(ids, string(s.id))
		}
	}
	return ids
}

// Copy lists every word the screens can show, before names are swapped in.
func Copy() []string {
	var out []string
	add := func(strs ...string) {
		for _, s := range strs {
			if s != "" {
				out = append(out, s)
			}
		}
	}
	for _, s := range specs {
		add(s.title, s.note, s.status, s.code)
		if s.field != nil {
			add(s.field.Label, s.field.Value)
		}
		for _, o := range slices.Concat(s.statuses, s.reasons) {
			add(o.text)
		}
		for _, c := range s.choices {
			add(c.label)
		}
	}
	return out
}

// NextFor is the command an ack teaches after showing a screen.
func NextFor(id ID) string {
	s, _ := find(id)
	return s.next
}

// AwaitsClick reports whether a screen has something on the page for a person to
// click. Only then can a person be reading while the agent is gone. "Your turn"
// screens wait on Apple's window or Terminal, so they can't hold the lost screen
// back: a click would never come, and the agent should be running wait instead.
func AwaitsClick(id ID) bool {
	s, ok := find(id)
	return ok && len(s.choices) > 0
}

// IsSettled reports whether a screen has nothing left to happen.
func IsSettled(id ID) bool {
	s, ok := find(id)
	return ok && s.mode == Settled
}
