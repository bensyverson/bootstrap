package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// HiddenVerb is how serve re-launches itself in the background.
const HiddenVerb = "__server"

// FlowArg tells the hidden verb to run the setup inside the server.
const FlowArg = "--flow"

const (
	startTimeout = 10 * time.Second
	openTimeout  = 10 * time.Second
	endedWait    = 3 * time.Second
)

// Browser is what became of opening the page.
type Browser string

const (
	BrowserOpened      Browser = "opened"
	BrowserAlreadyOpen Browser = "already-open"
	BrowserFailed      Browser = "failed"
)

// StartOptions says how to start a server.
type StartOptions struct {
	Store session.Store
	Agent screen.Agent
	// Exe is this binary, re-launched as HiddenVerb with Env.
	Exe  string
	Env  []string
	Port int
	// OpenPath is the program that opens a URL in the browser.
	OpenPath string
	// Flow launches a server that runs the setup itself (FlowArg on the hidden verb).
	Flow bool
}

// Started is what serve reports.
type Started struct {
	URL     string
	PID     int
	Reused  bool
	Browser Browser
}

// StartError is a server that couldn't start, which sends the agent to the chat fallback.
type StartError struct {
	Reason string
}

func (e *StartError) Error() string { return e.Reason }

// report is the one line a background server sends back to the command that launched it.
type report struct {
	URL    string `json:"url,omitempty"`
	PID    int    `json:"pid,omitempty"`
	Reused bool   `json:"reused,omitempty"`
	// Busy means another setup holds this Mac's setup slot, so the flow can't run.
	Busy  bool   `json:"busy,omitempty"`
	Error string `json:"error,omitempty"`
}

// Start reuses a running server or launches one in the background, then opens the page.
func Start(opts StartOptions) (Started, error) {
	started, err := reuse(opts)
	if errors.Is(err, errNotRunning) {
		started, err = launch(opts)
	}
	if err != nil {
		return Started{}, err
	}
	if started.Browser == "" {
		started.Browser = openBrowser(opts.OpenPath, started.URL)
	}
	return started, nil
}

var errNotRunning = errors.New("no server running")

func reuse(opts StartOptions) (Started, error) {
	if !opts.Store.ServerRunning() {
		return Started{}, errNotRunning
	}
	state, err := opts.Store.Load()
	if err != nil && !errors.Is(err, session.ErrNoSession) {
		return Started{}, err
	}
	if state.Ended {
		// A stopped server is on its way out, and the slot is about to be free.
		for deadline := time.Now().Add(endedWait); opts.Store.ServerRunning(); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				return Started{}, &StartError{Reason: "the last server hasn't exited"}
			}
		}
		return Started{}, errNotRunning
	}
	info, err := opts.Store.ReadServerInfo()
	if err != nil {
		return Started{}, &StartError{Reason: fmt.Sprintf("a server is starting, or stuck: %v", err)}
	}
	now := time.Now()
	if state.Agent != opts.Agent {
		if err := opts.Store.SetAgent(opts.Agent, now); err != nil {
			return Started{}, err
		}
	} else if err := opts.Store.Renew(now); err != nil {
		return Started{}, err
	}
	started := Started{URL: info.URL, PID: info.PID, Reused: true}
	if state.Tabs > 0 {
		started.Browser = BrowserAlreadyOpen
	}
	return started, nil
}

func launch(opts StartOptions) (Started, error) {
	args := []string{HiddenVerb, "--agent", opts.Agent.ID, "--port", strconv.Itoa(opts.Port)}
	if opts.Flow {
		args = append(args, FlowArg)
	}
	cmd := exec.Command(opts.Exe, args...)
	cmd.Env = opts.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Started{}, &StartError{Reason: err.Error()}
	}
	if err := cmd.Start(); err != nil {
		return Started{}, &StartError{Reason: err.Error()}
	}

	lines := make(chan report, 1)
	go func() {
		var line report
		if body, err := bufio.NewReader(stdout).ReadBytes('\n'); err != nil {
			line.Error = "the background server exited without saying why"
		} else if err := json.Unmarshal(body, &line); err != nil {
			line.Error = "the background server said something unreadable"
		}
		lines <- line
	}()

	select {
	case line := <-lines:
		switch {
		case line.Error != "":
			cmd.Wait()
			return Started{}, &StartError{Reason: line.Error}
		case line.Busy:
			cmd.Wait()
			return Started{}, session.ErrSetupRunning
		case line.Reused:
			cmd.Wait()
			// Another serve won the race for the slot, so this one uses its server.
			return reuse(opts)
		}
		cmd.Process.Release()
		return Started{URL: line.URL, PID: line.PID}, nil
	case <-time.After(startTimeout):
		cmd.Process.Kill()
		cmd.Wait()
		return Started{}, &StartError{Reason: fmt.Sprintf("the background server didn't start in %v", startTimeout)}
	}
}

func openBrowser(openPath, url string) Browser {
	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, openPath, url).Run(); err != nil {
		return BrowserFailed
	}
	return BrowserOpened
}

// ChildOptions says how the background server runs.
type ChildOptions struct {
	Store session.Store
	Agent screen.Agent
	Port  int
	Page  fs.FS
	// Flow, when set, drives the setup from inside the server once the session has begun.
	Flow func(context.Context) error
}

// RunChild is the background server's whole life. It reports one line on out, then
// closes it so the command that launched it can return. The kernel drops the
// server lock when this process exits, so a later command sees the truth.
func RunChild(opts ChildOptions, out io.WriteCloser) {
	say := func(line report) {
		json.NewEncoder(out).Encode(line)
		out.Close()
	}
	release, err := opts.Store.LockServer()
	if errors.Is(err, session.ErrServerRunning) {
		say(report{Reused: true})
		return
	}
	if err != nil {
		say(report{Error: err.Error()})
		return
	}
	defer release()
	if opts.Flow != nil {
		setup, err := opts.Store.LockSetup()
		if errors.Is(err, session.ErrSetupRunning) {
			say(report{Busy: true})
			return
		}
		if err != nil {
			say(report{Error: err.Error()})
			return
		}
		defer setup()
	}

	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		say(report{Error: err.Error()})
		return
	}
	srv, err := New(Config{Store: opts.Store, Token: hex.EncodeToString(raw), Port: opts.Port, Page: opts.Page, Flow: opts.Flow})
	if err != nil {
		say(report{Error: err.Error()})
		return
	}
	pid := os.Getpid()
	if _, err := opts.Store.Begin(opts.Agent, pid, time.Now()); err != nil {
		say(report{Error: err.Error()})
		return
	}
	if err := opts.Store.WriteServerInfo(session.ServerInfo{PID: pid, URL: srv.URL(), Port: srv.Port(), Flow: opts.Flow != nil}); err != nil {
		say(report{Error: err.Error()})
		return
	}
	defer opts.Store.RemoveServerInfo()
	say(report{URL: srv.URL(), PID: pid})
	srv.Run()
}
