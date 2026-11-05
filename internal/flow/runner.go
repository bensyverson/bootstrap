package flow

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

// CommandKind is whether a command changes the Mac. A second walk over a finished
// setup runs only inspections, and the tests hold it to that.
type CommandKind string

const (
	Inspect CommandKind = "inspect"
	Change  CommandKind = "change"
)

// Command is one outside program, always by full path, since a session that was
// running before Homebrew installed can't find it by name.
type Command struct {
	Kind    CommandKind
	Path    string
	Args    []string
	Dir     string
	Timeout time.Duration
}

// String is the command as a person would type it, which is what a handoff shows.
func (c Command) String() string {
	words := []string{filepath.Base(c.Path)}
	for _, a := range c.Args {
		if a == "" || strings.ContainsAny(a, " \t'\"") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		words = append(words, a)
	}
	return strings.Join(words, " ")
}

// Result is what became of a command. Err means it never started.
type Result struct {
	Exit     int
	Output   []byte
	TimedOut bool
	Err      error
}

// Runner runs every outside command the flow needs. Tests use a fake, so no test touches the Mac.
type Runner interface {
	Run(ctx context.Context, cmd Command) Result
	// Stream runs a command as Run does, and hands line each line of its output,
	// without the newline, as it arrives, for a command that says something the
	// person needs before it exits. line runs on the caller's behalf, one call at a time.
	Stream(ctx context.Context, cmd Command, line func(string)) Result
}
