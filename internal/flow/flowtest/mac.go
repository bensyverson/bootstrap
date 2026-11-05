// Package flowtest is a pretend Mac for the flow's tests: a Runner that answers
// each command the way the real tool does, by changing files under a temporary
// root, so a test can walk the whole setup without touching this Mac.
package flowtest

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
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
	globals   map[string]string
	hooks     []string
	// Signer is the package's signer that pkgutil reports first in the chain.
	Signer string
	// Chain is any further certificates pkgutil lists before Apple's own.
	Chain []string
	// Notarized is whether pkgutil reports Apple's notary trusting the package.
	Notarized bool
	// Unsigned makes pkgutil report no signature at all.
	Unsigned bool
	// QuietInstalls leaves Apple's and Homebrew's installers running without ever finishing.
	QuietInstalls bool
	// GitHubUser is who gh api user says is signed in.
	GitHubUser GitHubUser
	// LoginWait runs while gh auth login waits for the person, after it printed its code.
	LoginWait func()
}

// GitHubUser is the part of gh api user the flow reads.
type GitHubUser struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
}

// LoginCode is the one-time code the pretend gh prints.
const LoginCode = "D7BA-CFE2"

type override struct {
	match  string
	result flow.Result
	times  int // 0 is every time
}

// New makes a Mac with nothing on it.
func New(root string) *Mac {
	return &Mac{Root: root, globals: map[string]string{}, Signer: HomebrewSigner, Notarized: true,
		GitHubUser: GitHubUser{Login: "ada", ID: 1234, Name: "Ada Lovelace"}}
}

// Globals is Git's global settings.
func (m *Mac) Globals() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.globals)
}

// SetGlobal sets one of Git's global settings, as the person might have before.
func (m *Mac) SetGlobal(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globals[key] = value
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

// Stream answers one command, handing over its output a line at a time. gh's
// sign-in prints its code, waits in LoginWait as the real one waits for the
// person, and only then finishes.
func (m *Mac) Stream(ctx context.Context, cmd flow.Command, line func(string)) flow.Result {
	if filepath.Base(cmd.Path) != "gh" || len(cmd.Args) < 2 || cmd.Args[0] != "auth" || cmd.Args[1] != "login" || m.overridden(cmd) {
		res := m.Run(ctx, cmd)
		each(res.Output, line)
		return res
	}
	m.mu.Lock()
	m.calls = append(m.calls, cmd)
	wait := m.LoginWait
	m.mu.Unlock()
	first := "! One-time code (" + LoginCode + ") copied to clipboard\nOpen this URL to continue in your web browser: https://github.com/login/device\n"
	each([]byte(first), line)
	if wait != nil {
		wait()
	}
	m.mu.Lock()
	m.loggedIn = true
	m.mu.Unlock()
	last := "✓ Authentication complete.\n✓ Logged in as " + m.GitHubUser.Login + "\n"
	each([]byte(last), line)
	return ok(first + last)
}

func each(output []byte, line func(string)) {
	for l := range strings.Lines(string(output)) {
		line(strings.TrimRight(l, "\n"))
	}
}

func (m *Mac) overridden(cmd flow.Command) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.ContainsFunc(m.overrides, func(o override) bool { return strings.Contains(cmd.String(), o.match) })
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
		return m.pkgutil(filepath.Base(arg(1)))
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
		case arg(0) == "auth" && arg(1) == "setup-git":
			return ok("")
		case arg(0) == "api" && arg(1) == "user":
			if !m.loggedIn {
				return failed(4, "To get started with GitHub CLI, please run:  gh auth login")
			}
			body, err := json.Marshal(m.GitHubUser)
			if err != nil {
				return fail(err)
			}
			return ok(string(body))
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
	}
	switch args := cmd.Args; {
	case len(args) == 4 && args[0] == "config" && args[1] == "--global" && args[2] == "--get":
		value, set := m.globals[args[3]]
		if !set {
			return failed(1, "")
		}
		return ok(value + "\n")
	case len(args) == 4 && args[0] == "config" && args[1] == "--global":
		m.globals[args[2]] = args[3]
		return ok("")
	}
	return failed(1, "git: unknown")
}

// pkgutil prints what the real one does for a package: its status, Apple's
// notary, and the chain from the signer up to Apple's root.
func (m *Mac) pkgutil(name string) flow.Result {
	if m.Unsigned {
		return failed(1, fmt.Sprintf("Package %q:\n   Status: no signature\n", name))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Package %q:\n   Status: signed by a developer certificate issued by Apple for distribution\n", name)
	if m.Notarized {
		b.WriteString("   Notarization: trusted by the Apple notary service\n")
	}
	b.WriteString("   Signed with a trusted timestamp on: 2026-10-01 18:00:00 +0000\n   Certificate Chain:\n")
	for i, cert := range slices.Concat([]string{m.Signer}, m.Chain, []string{"Developer ID Certification Authority", "Apple Root CA"}) {
		fmt.Fprintf(&b, "    %d. %s\n       Expires: 2030-01-01 00:00:00 +0000\n       SHA256 Fingerprint:\n           AB CD EF\n       ------------------------------------------------------------------------\n", i+1, cert)
	}
	return ok(b.String())
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
