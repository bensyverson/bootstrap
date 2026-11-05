package flow

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
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
	return r.Stream(ctx, cmd, nil)
}

// Stream runs one command as Run does, handing each line to line as the command
// writes it. Every call to line has returned by the time Stream does.
func (r ExecRunner) Stream(ctx context.Context, cmd Command, line func(string)) Result {
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
	// One writer for both, so os/exec writes from one goroutine at a time.
	out := &lineWriter{line: line}
	c.Stdout, c.Stderr = out, out
	err := c.Run()
	out.flush()
	res := Result{Output: out.all.Bytes()}
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

// lineWriter keeps a command's whole output and hands on each finished line.
type lineWriter struct {
	all     bytes.Buffer
	partial []byte
	line    func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.all.Write(p)
	if w.line == nil {
		return len(p), nil
	}
	w.partial = append(w.partial, p...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.line(strings.TrimSuffix(string(w.partial[:i]), "\r"))
		w.partial = w.partial[i+1:]
	}
}

// flush hands on a last line the command didn't end.
func (w *lineWriter) flush() {
	if w.line != nil && len(w.partial) > 0 {
		w.line(string(w.partial))
		w.partial = nil
	}
}
