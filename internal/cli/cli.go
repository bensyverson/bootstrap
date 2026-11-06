// Package cli adapts the commands to the library. It parses, renders and maps
// errors to exit codes; decisions live in the packages it calls.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The house exit codes.
const (
	exitOK       = 0
	exitUsage    = 2
	exitTimeout  = 3
	exitTarget   = 4
	exitEnviron  = 5
	defaultOpen  = "/usr/bin/open"
	fallbackNext = "tell the person in one sentence that the page can't open, then set up in the chat"
)

// Version is the release's version, stamped at link time by scripts/release.
var Version = "dev"

// Environment variables. All but the agent's are seams for tests and for the fallback path.
const (
	envDir  = "BOOTSTRAP_DIR"
	envRoot = "BOOTSTRAP_ROOT"
	envOpen = "BOOTSTRAP_OPEN"
	envPort = "BOOTSTRAP_PORT"
)

// Env is everything a command takes from the outside world.
type Env struct {
	Getenv  func(string) string
	Environ func() []string
	Cwd     func() (string, error)
	Exe     func() (string, error)
	Stdin   io.Reader
	// Terminal reports whether a person is at a keyboard: stdin and stdout are both a terminal.
	Terminal func() bool
	Stdout   io.Writer
	Stderr   io.Writer
}

// SystemEnv is the real environment.
func SystemEnv() Env {
	return Env{Getenv: os.Getenv, Environ: os.Environ, Cwd: os.Getwd, Exe: os.Executable, Stdin: os.Stdin,
		Terminal: func() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }, Stdout: os.Stdout, Stderr: os.Stderr}
}

// output is what a verb reports: rows for the default form, value for --json.
type output struct {
	rows  []string
	next  string
	value any
}

// failure is an error that knows its exit code and the next command.
type failure struct {
	code    int
	message string
	valid   []string
	next    string
	state   string
	url     string
	pid     int
	browser string
	// updates are the flow's work a wait read before it ran out of time.
	updates []session.Update
	// questions are what a refused new still needs answered.
	questions []openQuestion
}

func (f *failure) Error() string { return f.message }

type errorBody struct {
	Error     string           `json:"error"`
	Exit      int              `json:"exit"`
	Valid     []string         `json:"valid,omitempty"`
	State     string           `json:"state,omitempty"`
	URL       string           `json:"url,omitempty"`
	PID       int              `json:"pid,omitempty"`
	Browser   string           `json:"browser,omitempty"`
	Updates   []session.Update `json:"updates,omitempty"`
	Questions []openQuestion   `json:"questions,omitempty"`
	Next      string           `json:"next"`
}

// command is one verb: its help, its flags and what it does.
type command struct {
	name    string
	class   string
	usage   string
	about   string
	example string
	flags   []string
	// switches are flags that take no value.
	switches []string
	// lists are flags that may be given more than once, or with commas between values.
	lists []string
	run   func(c *call) (output, error)
}

// call is one invocation of a verb.
type call struct {
	env     Env
	verb    string
	args    []string
	options map[string]string
	lists   map[string][]string
}

func (c *call) flag(name string) string { return c.options[name] }

// switched reports whether a switch was given.
func (c *call) switched(name string) bool {
	_, ok := c.options[name]
	return ok
}

// verbs lists the commands in the order the primer gives them.
func verbs() []command {
	return []command{detectCommand, toolsCommand, screensCommand, statusCommand, newCommand, runCommand, waitCommand, resumeCommand, serveCommand, showCommand, stopCommand, prefsCommand}
}

func verbNames() []string {
	var names []string
	for _, v := range verbs() {
		names = append(names, v.name)
	}
	return names
}

// Run runs one command and returns its exit code.
func Run(args []string, env Env) int {
	if len(args) > 0 && args[0] == hiddenVerb {
		return runChild(args[1:], env)
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(env.Stdout, primer)
		return exitOK
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(env.Stdout, "bootstrap %s\n", Version)
		return exitOK
	}
	asJSON := slices.Contains(args, "--json")
	i := slices.IndexFunc(verbs(), func(v command) bool { return v.name == args[0] })
	if i < 0 {
		return report(env, asJSON, &failure{code: exitUsage, message: fmt.Sprintf("unknown command %q", args[0]), valid: verbNames(), next: "bootstrap"})
	}
	verb := verbs()[i]
	if slices.Contains(args[1:], "--help") || slices.Contains(args[1:], "-h") {
		fmt.Fprintf(env.Stdout, "bootstrap %s\n%s\nClass: %s\n\nExample:\n  %s\n", verb.usage, verb.about, verb.class, verb.example)
		return exitOK
	}
	c, err := parse(verb, args[1:], env)
	if err != nil {
		return report(env, asJSON, err)
	}
	out, err := verb.run(c)
	if err != nil {
		return report(env, asJSON, err)
	}
	prog := program(env)
	if asJSON {
		encode(env.Stdout, valueByPath(out.value, prog))
		return exitOK
	}
	for _, row := range out.rows {
		if rest, ok := strings.CutPrefix(row, "next: "); ok {
			row = "next: " + byPath(rest, prog)
		}
		fmt.Fprintln(env.Stdout, row)
	}
	if out.next != "" {
		fmt.Fprintf(env.Stdout, "next: %s\n", byPath(out.next, prog))
	}
	return exitOK
}

func parse(verb command, args []string, env Env) (*call, error) {
	help := "bootstrap " + verb.name + " --help"
	c := &call{env: env, verb: verb.name, options: map[string]string{}, lists: map[string][]string{}}
	valid := []string{}
	for _, f := range append(slices.Concat(verb.flags, verb.lists, verb.switches), "json") {
		valid = append(valid, "--"+f)
	}
	valid = append(valid, "--help")
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			c.args = append(c.args, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name == "json" {
			continue
		}
		if slices.Contains(verb.switches, name) {
			if hasValue {
				return nil, &failure{code: exitUsage, message: "--" + name + " takes no value", next: help}
			}
			c.options[name] = ""
			continue
		}
		listed := slices.Contains(verb.lists, name)
		if !listed && !slices.Contains(verb.flags, name) {
			return nil, &failure{code: exitUsage, message: "unknown flag --" + name, valid: valid, next: help}
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, &failure{code: exitUsage, message: "--" + name + " needs a value", next: help}
			}
			i++
			value = args[i]
		}
		if listed {
			for v := range strings.SplitSeq(value, ",") {
				if v = strings.TrimSpace(v); v != "" {
					c.lists[name] = append(c.lists[name], v)
				}
			}
			continue
		}
		c.options[name] = value
	}
	return c, nil
}

func encode(w io.Writer, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "%s\n", body)
}

// report prints an error, and returns its exit code. Every error ends with the next command.
func report(env Env, asJSON bool, err error) int {
	f := asFailure(err)
	f.next = byPath(f.next, program(env))
	if asJSON {
		encode(env.Stdout, errorBody{Error: f.message, Exit: f.code, Valid: f.valid, State: f.state, URL: f.url, PID: f.pid, Browser: f.browser, Updates: f.updates, Questions: f.questions, Next: f.next})
		return f.code
	}
	fmt.Fprintf(env.Stderr, "error: %s\n", f.message)
	if len(f.valid) > 0 {
		fmt.Fprintf(env.Stderr, "valid: %s\n", strings.Join(f.valid, ", "))
	}
	if f.state != "" {
		fmt.Fprintln(env.Stderr, f.state)
	}
	fmt.Fprintf(env.Stderr, "next: %s\n", f.next)
	return f.code
}

// asFailure maps any library error to its exit code and remedy.
func asFailure(err error) *failure {
	if f, ok := errors.AsType[*failure](err); ok {
		return f
	}
	if invalid, ok := errors.AsType[*screen.InvalidError](err); ok {
		next := invalid.Next
		if next == "" {
			next = "bootstrap screens"
		}
		return &failure{code: exitUsage, message: invalid.Message, valid: invalid.Valid, next: next}
	}
	if corrupt, ok := errors.AsType[*session.CorruptError](err); ok {
		return &failure{code: exitTarget, message: corrupt.Error(), next: fmt.Sprintf("delete %s, then bootstrap serve", corrupt.Path)}
	}
	switch {
	case errors.Is(err, session.ErrNoSession):
		return &failure{code: exitEnviron, message: "no session", next: "bootstrap serve"}
	case errors.Is(err, session.ErrEnded):
		return &failure{code: exitEnviron, message: "session ended", next: "bootstrap serve"}
	case errors.Is(err, session.ErrPageGone):
		return &failure{code: exitEnviron, message: "the page is closed", next: "bootstrap serve"}
	}
	return &failure{code: exitEnviron, message: err.Error(), next: fallbackNext}
}

func (c *call) store() (session.Store, error) {
	if dir := c.env.Getenv(envDir); dir != "" {
		return session.Store{Dir: dir}, nil
	}
	home := c.env.Getenv("HOME")
	if home == "" {
		return session.Store{}, &failure{code: exitEnviron, message: "HOME isn't set, so bootstrap has no folder", next: fallbackNext}
	}
	return session.Store{Dir: filepath.Join(home, ".bootstrap")}, nil
}

// renew is for reads that need no session: when one exists, they still count as the
// agent checking in, and when none does, or the folder can't be written, the read goes on.
func (c *call) renew() {
	if store, err := c.store(); err == nil {
		store.Renew(time.Now())
	}
}

// agent resolves the app's name. known is false only when nothing says which app this is.
func (c *call) agent() (agent screen.Agent, known bool, err error) {
	if id := c.flag("agent"); id != "" {
		found, err := screen.FindAgent(id)
		if err != nil {
			var invalid *screen.InvalidError
			errors.As(err, &invalid)
			return screen.Agent{}, false, &failure{code: exitUsage, message: invalid.Message, valid: invalid.Valid, next: "bootstrap " + c.verb + " --help"}
		}
		return found, true, nil
	}
	switch {
	case c.env.Getenv("CLAUDECODE") != "":
		return screen.ClaudeCode, true, nil
	case c.env.Getenv("OPENCODE") != "":
		return screen.OpenCode, true, nil
	}
	return screen.Agent{}, false, nil
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// kv is one key=value cell, quoted when a space would break the row.
func kv(key, value string) string {
	if value == "" || strings.ContainsAny(value, " \t\n\"") {
		return key + "=" + strconv.Quote(value)
	}
	return key + "=" + value
}
