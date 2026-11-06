package cli

import (
	"regexp"
	"strings"
)

// The setup installs bootstrap at ~/.bootstrap/bin/bootstrap, off the PATH, so
// a next: line that says plain "bootstrap" would send an agent to "command not
// found". Every next: line, on success or in an error, names the program by
// the path it ran from. Help and the primer document the verbs, so they keep
// the short name.

// commandWord is "bootstrap" where it starts a command in a next: line: at the
// start, or after a space, a semicolon or a parenthesis, and followed by a
// space or the end. "~/.bootstrap/…" isn't one.
var commandWord = regexp.MustCompile(`(^|[\s;(])bootstrap(\s|$)`)

// program is the path this binary ran from, or "bootstrap" when it can't tell.
func program(env Env) string {
	if env.Exe == nil {
		return "bootstrap"
	}
	exe, err := env.Exe()
	if err != nil || exe == "" {
		return "bootstrap"
	}
	return exe
}

// byPath names every bootstrap command in a next: line by the program's path.
func byPath(next, program string) string {
	return commandWord.ReplaceAllStringFunc(next, func(m string) string {
		return strings.Replace(m, "bootstrap", program, 1)
	})
}
