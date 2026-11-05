// Package server shows the session to a browser. It exists only because a page
// needs live updates from a local address: it tails the log, sends new screens
// over Server-Sent Events and appends what the person decides. It holds no state
// that a restart would lose.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bensyverson/bootstrap/internal/page"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Timing around tabs and sleep.
const (
	// ClosedTabGrace covers a reload, and a connection the browser drops and remakes.
	ClosedTabGrace = 30 * time.Second
	// FirstTabGrace is long, because a browser's first launch on a new Mac can be slow,
	// and someone may have to open the link by hand.
	FirstTabGrace = 10 * time.Minute
	// sleepGap is a gap between readings that only a sleeping machine would leave.
	sleepGap    = 30 * time.Second
	defaultTick = 200 * time.Millisecond
)

// ExitReason says why a server should exit.
type ExitReason string

const (
	ExitNone      ExitReason = ""
	ExitStopped   ExitReason = "stopped"
	ExitTabClosed ExitReason = "tab closed"
	ExitNoTab     ExitReason = "no tab opened"
	ExitHourUp    ExitReason = "the agent has been gone an hour"
	ExitLogGone   ExitReason = "the log can't be read"
)

// Config sets up a server. Zero durations and a nil Now take the defaults.
type Config struct {
	Store         session.Store
	Token         string
	Port          int
	Page          fs.FS
	Now           func() time.Time
	Tick          time.Duration
	FirstTabGrace time.Duration
}

// Server is one running page.
type Server struct {
	cfg      Config
	listener net.Listener
	http     *http.Server
	origin   string
	host     string
	policy   string

	mu           sync.Mutex
	clients      map[chan []byte]bool
	lastFrame    []byte
	lastStep     time.Time
	lastDeadline time.Time
	credit       time.Duration
	everTab      bool
	emptySince   time.Time
	closeOnce    sync.Once
}

// New binds to the loopback address, and only there. A failure here is what sends
// the agent to the chat fallback.
func New(cfg Config) (*Server, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("server needs a token")
	}
	if cfg.Now == nil {
		// Round(0) drops the monotonic reading, which stops during sleep on macOS. The wall clock's jump is how sleep shows.
		cfg.Now = func() time.Time { return time.Now().Round(0) }
	}
	if cfg.Tick == 0 {
		cfg.Tick = defaultTick
	}
	if cfg.FirstTabGrace == 0 {
		cfg.FirstTabGrace = FirstTabGrace
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	now := cfg.Now()
	s := &Server{
		cfg:        cfg,
		listener:   listener,
		host:       fmt.Sprintf("127.0.0.1:%d", port),
		origin:     fmt.Sprintf("http://127.0.0.1:%d", port),
		policy:     page.Policy(cfg.Page),
		clients:    map[chan []byte]bool{},
		lastStep:   now,
		emptySince: now,
	}
	s.http = &http.Server{Handler: s.guard(s.routes()), ReadHeaderTimeout: 5 * time.Second}
	return s, nil
}

// Addr is where the server listens.
func (s *Server) Addr() net.Addr { return s.listener.Addr() }

// Port is the port the server listens on.
func (s *Server) Port() int { return s.listener.Addr().(*net.TCPAddr).Port }

// URL is the link a person opens, token included.
func (s *Server) URL() string { return fmt.Sprintf("%s/?token=%s", s.origin, s.cfg.Token) }

// Serve starts answering requests.
func (s *Server) Serve() { go s.http.Serve(s.listener) }

// Close says goodbye to every tab and stops the server.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		for client := range s.clients {
			close(client)
			delete(s.clients, client)
		}
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// Shutdown lets each stream write its last frame, and only then does Close cut what remains.
		s.http.Shutdown(ctx)
		s.http.Close()
	})
}

// Run serves until there's a reason to exit.
func (s *Server) Run() ExitReason {
	s.Serve()
	defer s.Close()
	tick := time.NewTicker(s.cfg.Tick)
	defer tick.Stop()
	for range tick.C {
		if reason := s.Step(s.cfg.Now()); reason != ExitNone {
			return reason
		}
	}
	return ExitNone
}

// Tabs counts the open pages.
func (s *Server) Tabs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// Step reads the log, updates the page, and says whether it's time to exit.
func (s *Server) Step(now time.Time) ExitReason {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.step(now)
}

func (s *Server) step(now time.Time) ExitReason {
	if gap := now.Sub(s.lastStep); gap > sleepGap {
		// A sleeping machine isn't an agent leaving, so the time it slept doesn't
		// count against the allowance or the tab grace.
		s.credit += gap
		s.emptySince = s.emptySince.Add(gap)
	}
	s.lastStep = now

	state, err := s.cfg.Store.Load()
	if err != nil {
		return ExitLogGone
	}
	if !state.Deadline.Equal(s.lastDeadline) {
		// The agent checked in after waking up, so old sleep says nothing about the new allowance.
		s.credit = 0
		s.lastDeadline = state.Deadline
	}
	awake := now.Add(-s.credit)
	s.publish(state.Frame(awake))

	switch {
	case state.Ended:
		return ExitStopped
	case state.PastLifetime(awake):
		return ExitHourUp
	case len(s.clients) > 0:
		return ExitNone
	case s.everTab && now.Sub(s.emptySince) > ClosedTabGrace:
		return ExitTabClosed
	case !s.everTab && now.Sub(s.emptySince) > s.cfg.FirstTabGrace:
		return ExitNoTab
	}
	return ExitNone
}

func (s *Server) publish(frame session.Frame) {
	body, err := json.Marshal(frame)
	if err != nil || bytes.Equal(body, s.lastFrame) {
		return
	}
	s.lastFrame = body
	for client := range s.clients {
		deliver(client, body)
	}
}

// deliver replaces whatever a slow client hasn't read, since only the latest frame matters.
func deliver(client chan []byte, body []byte) {
	select {
	case <-client:
	default:
	}
	client <- body
}
