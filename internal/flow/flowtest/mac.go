// Package flowtest is a pretend Mac for the flow's tests: a Runner that answers
// each command the way the real tool does, by changing files under a temporary
// root, so a test can walk the whole setup without touching this Mac.
package flowtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/bensyverson/bootstrap/internal/flow"
)

// HomebrewSigner is what pkgutil prints for a good Homebrew package.
const HomebrewSigner = "Developer ID Installer: Homebrew Inc (927JGANW46)"

// Mac answers commands. Root stands for / and holds what Apple and Homebrew install.
type Mac struct {
	Root string

	mu        sync.Mutex
	calls     []flow.Command
	overrides []override
	loggedIn  bool
	committed map[string]bool
	hooks     []string
	// Signer is the package's signer that pkgutil reports.
	Signer string
	// QuietInstalls leaves Apple's and Homebrew's installers running without ever finishing.
	QuietInstalls bool
}

type override struct {
	match  string
	result flow.Result
	times  int // 0 is every time
}

// New makes a Mac with nothing on it.
func New(root string) *Mac {
	return &Mac{Root: root, committed: map[string]bool{}, Signer: HomebrewSigner}
}

// On makes commands containing match return result, for the next times calls, or for every call when times is 0.
func (m *Mac) On(match string, result flow.Result, times int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides = append(m.overrides, override{match: match, result: result, times: times})
}

// Clear drops every override.
func (m *Mac) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrides = nil
}

// Calls is every command run so far.
func (m *Mac) Calls() []flow.Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.calls)
}

// Changes is every command run so far that changes the Mac.
func (m *Mac) Changes() []string {
	var out []string
	for _, c := range m.Calls() {
		if c.Kind == flow.Change {
			out = append(out, c.String())
		}
	}
	return out
}

// Hooks lists the tools whose hook was installed.
func (m *Mac) Hooks() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.hooks)
}

// Place puts an executable at a path under Root.
func (m *Mac) Place(path string) error {
	full := filepath.Join(m.Root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte("#!/bin/sh\n"), 0o755)
}

// Run answers one command.
func (m *Mac) Run(ctx context.Context, cmd flow.Command) flow.Result {
	m.mu.Lock()
	m.calls = append(m.calls, cmd)
	text := cmd.String()
	for i, o := range m.overrides {
		if !strings.Contains(text, o.match) {
			continue
		}
		if o.times > 0 {
			m.overrides[i].times--
			if m.overrides[i].times == 0 {
				m.overrides = slices.Delete(m.overrides, i, i+1)
			}
		}
		m.mu.Unlock()
		return o.result
	}
	m.mu.Unlock()
	return m.act(cmd)
}

func ok(output string) flow.Result { return flow.Result{Output: []byte(output)} }

func failed(exit int, output string) flow.Result {
	return flow.Result{Exit: exit, Output: []byte(output)}
}

func fail(err error) flow.Result { return flow.Result{Exit: 1, Output: []byte(err.Error())} }

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (m *Mac) act(cmd flow.Command) flow.Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := cmd.Args
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}
	switch name := filepath.Base(cmd.Path); name {
	case "xcode-select":
		if m.QuietInstalls {
			return ok("")
		}
		if err := m.Place("/Library/Developer/CommandLineTools/usr/bin/git"); err != nil {
			return fail(err)
		}
		return ok("xcode-select: note: install requested for command line developer tools")
	case "curl":
		if err := os.WriteFile(arg(slices.Index(args, "-o")+1), []byte("pkg"), 0o600); err != nil {
			return fail(err)
		}
		return ok("")
	case "pkgutil":
		return ok("Package \"Homebrew.pkg\":\n   Status: signed by a developer certificate issued by Apple for distribution\n   Notarization: trusted by the Apple notary service\n   Certificate Chain:\n    1. " + m.Signer + "\n")
	case "open":
		if strings.HasSuffix(arg(0), ".pkg") && !m.QuietInstalls {
			if err := m.Place("/opt/homebrew/bin/brew"); err != nil {
				return fail(err)
			}
		}
		return ok("")
	case "brew":
		return m.brew(args)
	case "gh":
		switch {
		case arg(0) == "auth" && arg(1) == "status":
			if m.loggedIn {
				return ok("✓ Logged in to github.com")
			}
			return failed(1, "You are not logged into any GitHub hosts.")
		case arg(0) == "auth" && arg(1) == "login":
			m.loggedIn = true
			return ok("✓ Logged in")
		}
	case "git":
		return m.git(cmd)
	case "agents":
		return m.agents(cmd)
	case "job":
		switch arg(0) {
		case "init":
			db := filepath.Join(cmd.Dir, ".jobs.db")
			if exists(db) {
				return failed(1, ".jobs.db already exists. Use --force to overwrite")
			}
			if err := os.WriteFile(db, []byte("db"), 0o644); err != nil {
				return fail(err)
			}
			return ok("initialized")
		case "gitignore":
			return m.appendLine(filepath.Join(cmd.Dir, ".gitignore"), ".jobs.db")
		}
	}
	if len(args) > 1 && args[0] == "hook" && args[1] == "install" {
		m.hooks = append(m.hooks, filepath.Base(cmd.Path))
		return ok("installed")
	}
	return failed(127, fmt.Sprintf("the pretend Mac doesn't know %q", cmd.String()))
}

func (m *Mac) brew(args []string) flow.Result {
	switch {
	case len(args) == 2 && args[0] == "trust":
		return ok("")
	case len(args) == 2 && args[0] == "install":
		formula := args[1][strings.LastIndex(args[1], "/")+1:]
		if err := m.Place("/opt/homebrew/bin/" + formula); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("🍺  /opt/homebrew/Cellar/%s/0.1.0: 5 files, 9MB", formula))
	}
	return failed(1, "Error: Unknown command")
}

func (m *Mac) git(cmd flow.Command) flow.Result {
	switch strings.Join(cmd.Args, " ") {
	case "init":
		if err := os.MkdirAll(filepath.Join(cmd.Dir, ".git"), 0o755); err != nil {
			return fail(err)
		}
		return ok("Initialized empty Git repository")
	case "rev-parse --verify --quiet HEAD":
		if m.committed[cmd.Dir] {
			return ok("abc123")
		}
		return failed(1, "")
	case "add -A":
		return ok("")
	}
	if len(cmd.Args) > 0 && cmd.Args[0] == "commit" {
		m.committed[cmd.Dir] = true
		return ok("[main (root-commit) abc123]")
	}
	return failed(1, "git: unknown")
}

func (m *Mac) agents(cmd flow.Command) flow.Result {
	agentsMD := filepath.Join(cmd.Dir, "AGENTS.md")
	claudeMD := filepath.Join(cmd.Dir, "CLAUDE.md")
	switch {
	case len(cmd.Args) > 0 && cmd.Args[0] == "init":
		if info, err := os.Lstat(claudeMD); err == nil && info.Mode()&os.ModeSymlink == 0 {
			return failed(1, "CLAUDE.md is not a link to AGENTS.md. Run agents migrate first.")
		}
		if !exists(agentsMD) {
			if err := os.WriteFile(agentsMD, []byte("# rules\n"), 0o644); err != nil {
				return fail(err)
			}
		}
		if !exists(claudeMD) {
			if err := os.Symlink("AGENTS.md", claudeMD); err != nil {
				return fail(err)
			}
		}
		return ok("initialized")
	case len(cmd.Args) > 0 && cmd.Args[0] == "migrate":
		if err := os.Rename(claudeMD, agentsMD); err != nil {
			return fail(err)
		}
		if err := os.Symlink("AGENTS.md", claudeMD); err != nil {
			return fail(err)
		}
		return ok("migrated")
	}
	return failed(1, "agents: unknown")
}

func (m *Mac) appendLine(path, line string) flow.Result {
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	if slices.Contains(strings.Split(string(body), "\n"), line) {
		return ok("already there")
	}
	if len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		body = append(body, '\n')
	}
	if err := os.WriteFile(path, append(body, line+"\n"...), 0o644); err != nil {
		return fail(err)
	}
	return ok("added")
}
