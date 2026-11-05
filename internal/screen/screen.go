// Package screen holds every word the page shows and the closed lists of what
// an agent may ask for. The agent picks a screen by name and this package fills
// in the words, so no agent can invent wording, even by accident.
package screen

import (
	"fmt"
	"strings"

	"github.com/bensyverson/bootstrap/catalog"
)

// ID names a screen.
type ID string

// Mode is what the object does, and whose turn it is.
type Mode string

const (
	Working Mode = "working"
	Waiting Mode = "waiting"
	Settled Mode = "settled"
)

// Layout is the prototype's placement of the words around the object.
type Layout string

const (
	LayoutCenter Layout = ""
	LayoutOver   Layout = "over"
	LayoutLow    Layout = "low"
)

// Span is a run of text, a link when URL is set. Words reach the page as spans,
// never as HTML, so the page has nothing to inject.
type Span struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// Choice is one button. ID is what wait answer returns; Label is what a person reads.
// A button with a URL opens that page in a new tab and answers nothing.
type Choice struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Primary bool   `json:"primary,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Row is one line of the list screen.
type Row struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Say      string `json:"say"`
	Required bool   `json:"required"`
	Checked  bool   `json:"checked"`
}

// FieldKind is what a text box holds, which decides how its answer is checked.
type FieldKind string

const (
	// FieldFolder is a project's folder name, made git-friendly when it's answered.
	FieldFolder FieldKind = ""
	// FieldName is a person's name, kept as they typed it.
	FieldName FieldKind = "name"
	// FieldEmail is an email address.
	FieldEmail FieldKind = "email"
)

// Field is the one text box. Label is also what the box shows while it's empty.
type Field struct {
	Label string    `json:"label"`
	Value string    `json:"value"`
	Kind  FieldKind `json:"kind,omitempty"`
}

// Folder reports whether the box names a folder, so its answer is made git-friendly.
func (f Field) Folder() bool { return f.Kind == FieldFolder }

// Description is everything the page draws. The page knows nothing about any
// particular screen, so anything it needs has to be in here. samples/ holds the
// contract for the JavaScript side, which can't import this struct.
type Description struct {
	Screen   ID       `json:"screen"`
	Mode     Mode     `json:"mode"`
	Layout   Layout   `json:"layout,omitempty"`
	Progress float64  `json:"progress"`
	Title    []Span   `json:"title"`
	Note     string   `json:"note,omitempty"`
	Status   []Span   `json:"status,omitempty"`
	Rows     []Row    `json:"rows,omitempty"`
	Code     string   `json:"code,omitempty"`
	Field    *Field   `json:"field,omitempty"`
	Choices  []Choice `json:"choices,omitempty"`
	Card     string   `json:"card,omitempty"`
}

// Request is what an agent asks to show.
type Request struct {
	Screen ID     `json:"screen"`
	Tool   string `json:"tool,omitempty"`
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Code is gh's one-time code, for the screen that shows it.
	Code string `json:"code,omitempty"`
	// Name and Email are what Git would sign the person's work with, from GitHub.
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// Context is what a description depends on besides the request.
type Context struct {
	Agent Agent
	// Chosen holds the tool ids the person kept. Nil means every tool, as before they've chosen.
	Chosen []string
	// Carried is the progress so far, which off-path screens like trouble hold steady.
	Carried float64
}

// Answer is what a person decided on a screen. Name is what they typed in the
// screen's text box, whatever the box holds.
type Answer struct {
	Choice string   `json:"choice"`
	Tools  []string `json:"tools,omitempty"`
	Name   string   `json:"name,omitempty"`
}

// InvalidError is a request or answer outside the closed lists. Valid lists what
// would have worked, and Next is the command that explains the list, if not bootstrap screens.
type InvalidError struct {
	Message string
	Valid   []string
	Next    string
}

func (e *InvalidError) Error() string { return e.Message }

func invalid(valid []string, format string, args ...any) error {
	return &InvalidError{Message: fmt.Sprintf(format, args...), Valid: valid}
}

// Agent is the app a person is using, which sets the names they read.
type Agent struct {
	ID  string `json:"id"`
	App string `json:"app"`
	AI  string `json:"ai"`
}

// The agents this binary can name.
var (
	ClaudeCode = Agent{ID: "claude-code", App: "Claude Code", AI: "Claude"}
	OpenCode   = Agent{ID: "opencode", App: "OpenCode", AI: "OpenCode"}
)

// Agents returns every agent.
func Agents() []Agent { return []Agent{ClaudeCode, OpenCode} }

// AgentIDs returns every agent id.
func AgentIDs() []string {
	var ids []string
	for _, a := range Agents() {
		ids = append(ids, a.ID)
	}
	return ids
}

// FindAgent resolves an agent id.
func FindAgent(id string) (Agent, error) {
	for _, a := range Agents() {
		if a.ID == id {
			return a, nil
		}
	}
	return Agent{}, invalid(AgentIDs(), "unknown agent %q", id)
}

// Fill swaps in the app's names.
func (a Agent) Fill(s string) string {
	return strings.NewReplacer("{app}", a.App, "{ai}", a.AI).Replace(s)
}

// allTools is what Chosen means before anyone has chosen.
func chosenOrAll(chosen []string) []string {
	if chosen != nil {
		return chosen
	}
	return catalog.IDs()
}
