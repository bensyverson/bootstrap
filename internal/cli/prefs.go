package cli

import (
	"errors"
	"fmt"

	"github.com/bensyverson/bootstrap/internal/prefs"
)

// bootstrap prefs shows what later setups take without asking, and forgets one
// on request (Ben, 2026-11-06, XbdzWw). The remembering itself is the flow's.

const prefsHelp = "bootstrap prefs --help"

var prefsCommand = command{
	name: "prefs", class: "Manage", usage: "prefs [reset <name>] [--json]",
	about: `Lists what bootstrap remembers between setups, one per row: the person's last answer to each question, the tools they went without, and where projects go. A setup, on the page or with new, takes each as its default, and new --yes takes it without asking.
reset <name> forgets one, so the next setup asks it again. The names are skip_tools, projects_in, git, github, identity, making, xcode, settings and phone.
The file is ~/.bootstrap/preferences.json. Nothing here needs a session.`,
	example: "bootstrap prefs reset github",
	run:     runPrefs,
}

type prefsValue struct {
	File        string        `json:"file"`
	Preferences []prefs.Entry `json:"preferences"`
	Next        string        `json:"next"`
}

type resetValue struct {
	File   string     `json:"file"`
	Name   prefs.Name `json:"name"`
	Forgot bool       `json:"forgot"`
	Next   string     `json:"next"`
}

func runPrefs(c *call) (output, error) {
	base, err := c.store()
	if err != nil {
		return output{}, err
	}
	file := prefs.File{Dir: base.Dir}
	switch {
	case len(c.args) == 0:
		return listPrefs(file)
	case c.args[0] != "reset":
		return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unknown prefs verb %q", c.args[0]), valid: []string{"reset"}, next: prefsHelp}
	case len(c.args) == 1:
		return output{}, &failure{code: exitUsage, message: "reset needs the name of what to forget", valid: names(), next: "bootstrap prefs"}
	case len(c.args) > 2:
		return output{}, &failure{code: exitUsage, message: fmt.Sprintf("unexpected argument %q", c.args[2]), next: prefsHelp}
	}
	name, err := prefs.ParseName(c.args[1])
	if unknown, ok := errors.AsType[*prefs.UnknownNameError](err); ok {
		return output{}, &failure{code: exitUsage, message: unknown.Error(), valid: unknown.Valid, next: "bootstrap prefs"}
	}
	if _, err := loadPrefs(file); err != nil {
		return output{}, err
	}
	forgot := false
	if err := file.Update(func(p *prefs.Preferences) { forgot = p.Forget(name) }); err != nil {
		return output{}, &failure{code: exitEnviron, message: err.Error(), next: "fix or delete " + file.Path() + ", then bootstrap prefs"}
	}
	row := "forgot " + string(name)
	if !forgot {
		row = string(name) + " wasn't remembered, so there was nothing to forget"
	}
	next := "bootstrap prefs"
	return output{rows: []string{row}, next: next, value: resetValue{File: file.Path(), Name: name, Forgot: forgot, Next: next}}, nil
}

func listPrefs(file prefs.File) (output, error) {
	p, err := loadPrefs(file)
	if err != nil {
		return output{}, err
	}
	entries := p.Entries()
	var rows []string
	for _, e := range entries {
		rows = append(rows, string(e.Name)+" "+e.Value)
	}
	next := "bootstrap prefs reset <name> forgets one, so the next setup asks again"
	if len(entries) == 0 {
		rows = []string{"nothing remembered yet"}
		next = "bootstrap new <name> remembers your answers for the next setup"
	}
	if entries == nil {
		entries = []prefs.Entry{}
	}
	return output{rows: rows, next: next, value: prefsValue{File: file.Path(), Preferences: entries, Next: next}}, nil
}

func loadPrefs(file prefs.File) (prefs.Preferences, error) {
	p, err := file.Load()
	if err != nil {
		return p, &failure{code: exitEnviron, message: err.Error(), next: "fix or delete " + file.Path() + ", then bootstrap prefs"}
	}
	return p, nil
}

func names() []string {
	var out []string
	for _, n := range prefs.Names() {
		out = append(out, string(n))
	}
	return out
}
