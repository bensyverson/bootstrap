package flow

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// DefaultHomebrewPkg is where Homebrew publishes its latest signed .pkg.
const DefaultHomebrewPkg = "https://github.com/Homebrew/brew/releases/latest/download/Homebrew.pkg"

// pipeGrace is how long a finished or killed command's children may hold its
// output open, so a stray background process can't hang the flow.
const pipeGrace = 2 * time.Second

// ExecRunner is the real Runner: it runs each command with os/exec, within the
// command's own timeout, in its Dir, with stdout and stderr captured together.
// It knows nothing about any one installer; those live in the steps.
type ExecRunner struct {
	// Root is put in front of every command's path. It's empty outside tests,
	// where it matches detect.Probe.Root, so a test's pretend Mac holds its programs.
	Root string
}

// Run runs one command. A command that can't start comes back with Err set.
func (r ExecRunner) Run(ctx context.Context, cmd Command) Result {
	runCtx, cancel := ctx, context.CancelFunc(func() {})
	if cmd.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cmd.Timeout)
	}
	defer cancel()
	c := exec.CommandContext(runCtx, filepath.Join(r.Root, cmd.Path), cmd.Args...)
	c.Dir = cmd.Dir
	// Its own process group, so a timeout stops whatever it started too, as brew starts others.
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = pipeGrace
	out, err := c.CombinedOutput()
	res := Result{Output: out}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.Exit = exitErr.ExitCode()
	case c.ProcessState == nil:
		res.Err = err
	default:
		res.Exit = c.ProcessState.ExitCode()
	}
	// Only the command's own limit is a timeout; the flow ending is the caller's to see.
	if ctx.Err() == nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
	}
	return res
}
