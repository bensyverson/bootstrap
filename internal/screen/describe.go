package screen

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bensyverson/bootstrap/catalog"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var linkToken = regexp.MustCompile(`\{link:[a-z]+\}|\{tool-link\}`)

// Validate checks a request against the closed lists. It doesn't look at the session.
func Validate(req Request) error {
	s, ok := find(req.Screen)
	if !ok || s.internal {
		if s.internal {
			return invalid(ScreenIDs(), "the %s screen appears on its own, so it can't be shown", req.Screen)
		}
		return invalid(ScreenIDs(), "unknown screen %q", req.Screen)
	}
	tools := s.toolIDs()
	switch {
	case s.tool == noTool && req.Tool != "":
		return invalid(nil, "the %s screen takes no tool", s.id)
	case s.tool != noTool && req.Tool == "":
		return &InvalidError{Message: fmt.Sprintf("the %s screen needs a tool", s.id), Valid: tools, Next: "bootstrap tools"}
	case s.tool != noTool && !slices.Contains(tools, req.Tool):
		return &InvalidError{Message: fmt.Sprintf("the %s screen can't show tool %q", s.id, req.Tool), Valid: tools, Next: "bootstrap tools"}
	}
	if req.Status != "" {
		if len(s.statuses) == 0 {
			return invalid(nil, "the %s screen takes no status", s.id)
		}
		if !slices.Contains(optionIDs(s.statuses), req.Status) {
			return invalid(optionIDs(s.statuses), "unknown status %q for the %s screen", req.Status, s.id)
		}
	}
	switch {
	case len(s.reasons) == 0 && req.Reason != "":
		return invalid(nil, "the %s screen takes no reason", s.id)
	case len(s.reasons) > 0 && req.Reason == "":
		return invalid(optionIDs(s.reasons), "the %s screen needs a reason", s.id)
	case len(s.reasons) > 0 && !slices.Contains(optionIDs(s.reasons), req.Reason):
		return invalid(optionIDs(s.reasons), "unknown reason %q for the %s screen", req.Reason, s.id)
	}
	return s.validateDetail(req)
}

// The details the flow fills in. Each has gh's or GitHub's exact shape, so a
// request can't carry words of its own onto the page.
var (
	oneTimeCode   = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)
	noreplyEmail  = regexp.MustCompile(`^[0-9]+\+[A-Za-z0-9-]+@users\.noreply\.github\.com$`)
	typedEmail    = regexp.MustCompile(`^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$`)
	maxNameLength = 255
)

func (s spec) validateDetail(req Request) error {
	switch {
	case s.detail != codeDetail && req.Code != "":
		return invalid(nil, "the %s screen takes no code", s.id)
	case s.detail != identityDetail && (req.Name != "" || req.Email != ""):
		return invalid(nil, "the %s screen takes no name or email", s.id)
	case s.detail == codeDetail && !oneTimeCode.MatchString(req.Code):
		return invalid(nil, "the %s screen needs gh's one-time code, like ABCD-1234", s.id)
	case s.detail == identityDetail && !GitName(req.Name):
		return invalid(nil, "the %s screen needs a name Git can use", s.id)
	case s.detail == identityDetail && !noreplyEmail.MatchString(req.Email):
		return invalid(nil, "the %s screen needs GitHub's private email, like 1234+name@users.noreply.github.com", s.id)
	}
	return nil
}

// GitName reports whether a name can go into git config as it is: one line, not
// mistakable for an option, and not empty.
func GitName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && !strings.HasPrefix(name, "-") &&
		utf8.RuneCountInString(name) <= maxNameLength && !strings.ContainsFunc(name, unicode.IsControl)
}

// GitEmail reports whether a typed email can go into git config as it is.
func GitEmail(email string) bool {
	return typedEmail.MatchString(email) && !strings.HasPrefix(email, "-") && len(email) <= maxNameLength
}

// CheckChosen refuses a tool the person skipped on the list screen, since showing
// it would mean installing something they said no to.
func CheckChosen(req Request, chosen []string) error {
	if chosen == nil || req.Screen != "tool" || slices.Contains(chosen, req.Tool) {
		return nil
	}
	return &InvalidError{Message: fmt.Sprintf("the person skipped %s", req.Tool), Valid: installOrder(chosen), Next: "bootstrap status"}
}

// Describe builds what the page draws for a valid request.
func Describe(req Request, ctx Context) (Description, error) {
	if err := Validate(req); err != nil {
		return Description{}, err
	}
	s, _ := find(req.Screen)
	return s.describe(req, ctx), nil
}

// DescribeLost builds the lost screen, holding the progress so far.
func DescribeLost(ctx Context) Description {
	s, _ := find(Lost)
	return s.describe(Request{Screen: Lost}, ctx)
}

// DescribeEnd builds the end screen, holding the progress so far.
func DescribeEnd(ctx Context) Description {
	s, _ := find(End)
	return s.describe(Request{Screen: End}, ctx)
}

func (s spec) describe(req Request, ctx Context) Description {
	tool, _ := catalog.Find(req.Tool)
	d := Description{
		Screen:   s.id,
		Mode:     s.mode,
		Layout:   s.layout,
		Progress: Progress(req, ctx.Chosen, ctx.Carried),
		Title:    spans(s.title, tool, ctx.Agent),
		Code:     s.code,
		Field:    s.field,
		Card:     s.card,
	}
	if s.cardTool {
		d.Card = tool.ID
	}
	if s.noteTool != "" {
		note, _ := catalog.Find(s.noteTool)
		d.Note = ctx.Agent.Fill(note.Say)
	} else if s.tool == installTool {
		d.Note = ctx.Agent.Fill(tool.Say)
	}
	d.Status = spans(s.statusText(req), tool, ctx.Agent)
	if s.detail == codeDetail {
		d.Code = req.Code
	}
	if s.detail == identityDetail {
		// After the app's names are in, so a name is shown exactly as GitHub has it.
		d.Title, d.Status = fillIdentity(d.Title, req), fillIdentity(d.Status, req)
	}
	for i, c := range s.choices {
		// Nothing offers to skip what can't be skipped.
		if c.id == "skip" && tool.Required {
			continue
		}
		d.Choices = append(d.Choices, Choice{ID: c.id, Label: fillTool(c.label, tool, ctx.Agent), Primary: i == 0})
	}
	for i, l := range s.links {
		d.Choices = append(d.Choices, Choice{ID: l.id, Label: fillTool(l.label, tool, ctx.Agent), URL: l.url, Primary: i == 0 && len(s.choices) == 0})
	}
	if s.rows {
		kept := chosenOrAll(ctx.Chosen)
		for _, t := range catalog.Tools() {
			d.Rows = append(d.Rows, Row{ID: t.ID, Name: t.Name, URL: t.URL, Say: ctx.Agent.Fill(t.Say), Required: t.Required, Checked: t.Required || slices.Contains(kept, t.ID)})
		}
	}
	return d
}

func (s spec) statusText(req Request) string {
	if len(s.statuses) > 0 {
		for _, o := range s.statuses {
			if o.id == req.Status {
				return o.text
			}
		}
		return s.statuses[0].text
	}
	for _, o := range s.reasons {
		if o.id == req.Reason {
			return o.text
		}
	}
	return s.status
}

func fillIdentity(in []Span, req Request) []Span {
	r := strings.NewReplacer("{name}", req.Name, "{email}", req.Email)
	out := make([]Span, len(in))
	for i, s := range in {
		out[i] = Span{Text: r.Replace(s.Text), URL: s.URL}
	}
	return out
}

func fillTool(text string, tool catalog.Tool, agent Agent) string {
	return agent.Fill(strings.ReplaceAll(text, "{tool}", tool.Name))
}

// spans cuts text at its link tokens.
func spans(text string, tool catalog.Tool, agent Agent) []Span {
	if text == "" {
		return nil
	}
	var out []Span
	add := func(words string) {
		if words != "" {
			out = append(out, Span{Text: fillTool(words, tool, agent)})
		}
	}
	last := 0
	for _, loc := range linkToken.FindAllStringIndex(text, -1) {
		add(text[last:loc[0]])
		token := text[loc[0]:loc[1]]
		linked := tool
		if id, ok := strings.CutPrefix(token, "{link:"); ok {
			linked, _ = catalog.Find(strings.TrimSuffix(id, "}"))
		}
		out = append(out, Span{Text: linked.Name, URL: linked.URL})
		last = loc[1]
	}
	add(text[last:])
	return out
}

func plain(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// installOrder is the tools that get an "Adding" screen, among those chosen.
func installOrder(chosen []string) []string {
	kept := chosenOrAll(chosen)
	var ids []string
	for _, t := range catalog.Tools() {
		if !t.OwnStep && (t.Required || slices.Contains(kept, t.ID)) {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// Progress works out how far along a screen is, from 0 to 1, using the prototype's
// steps: Apple's tools, Homebrew, every tool, and the project. Callers never pass
// a percentage, so the object's color can't drift from the truth.
func Progress(req Request, chosen []string, carried float64) float64 {
	order := installOrder(chosen)
	steps := float64(len(order) + 3)
	var p float64
	switch req.Screen {
	case "welcome", "list", "fast", "apple-ask":
		p = 0
	case "apple-wait":
		p = 0.5 / steps
	case "brew-ask":
		p = 1 / steps
	case "tool":
		i := slices.Index(order, req.Tool)
		if i < 0 {
			return carried
		}
		p = float64(i+2) / steps
	case "making", "xcode", "live-yet", "naming":
		p = (steps - 1) / steps
	case "building":
		p = 0.92
	case "github", "github-signup", "github-code", "identity", "identity-name", "identity-email", "phone", "done":
		p = 1
	default:
		return carried
	}
	return math.Round(p*1000) / 1000
}

// PathOrder is the main path's screens in order, for a set of chosen tools.
func PathOrder(chosen []string) []Request {
	requests := []Request{{Screen: "welcome"}, {Screen: "list"}, {Screen: "fast"}, {Screen: "apple-ask"}, {Screen: "apple-wait"}, {Screen: "brew-ask"}}
	for _, id := range installOrder(chosen) {
		requests = append(requests, Request{Screen: "tool", Tool: id, Status: "downloading"}, Request{Screen: "tool", Tool: id, Status: "installing"})
	}
	requests = append(requests,
		Request{Screen: "making"}, Request{Screen: "xcode"}, Request{Screen: "live-yet"}, Request{Screen: "naming"},
		Request{Screen: "building", Status: "folder"}, Request{Screen: "building", Status: "git"}, Request{Screen: "building", Status: "rules"},
		Request{Screen: "github"}, Request{Screen: "phone"}, Request{Screen: "done"})
	return requests
}

// ValidateAnswer checks an answer against the screen it answers.
func ValidateAnswer(desc Description, ans Answer) error {
	var choices []string
	for _, c := range desc.Choices {
		if c.URL == "" {
			choices = append(choices, c.ID)
		}
	}
	if !slices.Contains(choices, ans.Choice) {
		return invalid(choices, "unknown choice %q for the %s screen", ans.Choice, desc.Screen)
	}
	if len(desc.Rows) == 0 && len(ans.Tools) > 0 {
		return invalid(nil, "the %s screen has no tools to choose", desc.Screen)
	}
	for _, id := range ans.Tools {
		if !slices.ContainsFunc(desc.Rows, func(r Row) bool { return r.ID == id }) {
			return invalid(catalog.IDs(), "unknown tool %q", id)
		}
	}
	if desc.Field == nil {
		if ans.Name != "" {
			return invalid(nil, "the %s screen has no name to type", desc.Screen)
		}
		return nil
	}
	switch {
	// What's typed before a skip is thrown away, so it needn't be whole.
	case ans.Choice == "skip":
	case desc.Field.Kind == FieldName && !GitName(strings.TrimSpace(ans.Name)):
		return invalid(nil, "type your name on one line")
	case desc.Field.Kind == FieldEmail && !GitEmail(strings.TrimSpace(ans.Name)):
		return invalid(nil, "%q doesn't look like an email address", ans.Name)
	case desc.Field.Folder() && !SafeFolderName(ans.Name):
		return invalid(nil, "%q can't be a folder name. Use letters, numbers and dashes", ans.Name)
	}
	return nil
}

// KeepRequired returns the tools kept on the list screen, in catalog order. Required
// tools are always kept, whatever a request says.
func KeepRequired(tools []string) []string {
	var kept []string
	for _, t := range catalog.Tools() {
		if t.Required || slices.Contains(tools, t.ID) {
			kept = append(kept, t.ID)
		}
	}
	return kept
}

// AllToolIDs is every tool.
func AllToolIDs() []string { return catalog.IDs() }

// FolderName turns what a person typed into a git-friendly folder name: lowercase,
// with dashes for spaces. It refuses anything that still isn't safe, rather than
// quietly dropping characters and naming the folder something they didn't type.
func FolderName(typed string) (string, bool) {
	name := strings.ToLower(strings.Join(strings.Fields(typed), "-"))
	if !SafeFolderName(name) {
		return "", false
	}
	return name, true
}

// SafeFolderName reports whether a typed name is safe for an agent to use in a
// command: letters, numbers, dots, dashes and underscores, starting with a letter
// or number. Spaces are refused because agents don't reliably quote.
func SafeFolderName(name string) bool {
	return safeName.MatchString(name)
}
