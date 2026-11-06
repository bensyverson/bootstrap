package prefs

import (
	"fmt"
	"slices"
	"strings"
)

// Name names one thing bootstrap remembers, as preferences.json and bootstrap
// prefs write it.
type Name string

// What a setup remembers. The answers are named for the flags of bootstrap new
// that give them.
const (
	SkipTools  Name = "skip_tools"
	ProjectsIn Name = "projects_in"
	Git        Name = "git"
	GitHub     Name = "github"
	Identity   Name = "identity"
	Making     Name = "making"
	Xcode      Name = "xcode"
	Settings   Name = "settings"
	Phone      Name = "phone"
)

// Names lists every name, in the order a setup meets them.
func Names() []Name {
	return []Name{SkipTools, ProjectsIn, Git, GitHub, Identity, Making, Xcode, Settings, Phone}
}

// UnknownNameError is a name bootstrap doesn't remember anything by.
type UnknownNameError struct {
	Name  string
	Valid []string
}

func (e *UnknownNameError) Error() string {
	return fmt.Sprintf("bootstrap remembers nothing called %q", e.Name)
}

// ParseName resolves a name as a person types it.
func ParseName(s string) (Name, error) {
	if slices.Contains(Names(), Name(s)) {
		return Name(s), nil
	}
	var valid []string
	for _, n := range Names() {
		valid = append(valid, string(n))
	}
	return "", &UnknownNameError{Name: s, Valid: valid}
}

// Entry is one remembered thing and its value, as a person reads it.
type Entry struct {
	Name  Name   `json:"name"`
	Value string `json:"value"`
}

// Answer is the person's last choice on a question, if they made one.
func (p Preferences) Answer(n Name) (string, bool) {
	choice, ok := p.Answers[n]
	return choice, ok && choice != ""
}

// Remember keeps a choice for the next setup, over any earlier one.
func (p *Preferences) Remember(n Name, choice string) {
	if p.Answers == nil {
		p.Answers = map[Name]string{}
	}
	p.Answers[n] = choice
}

// Forget drops one remembered thing, so the next setup asks again. It reports
// whether there was anything to drop.
func (p *Preferences) Forget(n Name) bool {
	switch n {
	case ProjectsIn:
		had := p.ProjectsIn != ""
		p.ProjectsIn = ""
		return had
	case SkipTools:
		had := len(p.SkipTools) > 0
		p.SkipTools = nil
		return had
	}
	_, had := p.Answer(n)
	delete(p.Answers, n)
	if len(p.Answers) == 0 {
		p.Answers = nil
	}
	return had
}

// Entries lists what's remembered, in the order a setup meets it.
func (p Preferences) Entries() []Entry {
	var out []Entry
	for _, n := range Names() {
		value := ""
		switch n {
		case ProjectsIn:
			value = p.ProjectsIn
		case SkipTools:
			value = strings.Join(p.SkipTools, ",")
		default:
			value, _ = p.Answer(n)
		}
		if value != "" {
			out = append(out, Entry{Name: n, Value: value})
		}
	}
	return out
}
