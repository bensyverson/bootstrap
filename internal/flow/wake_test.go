package flow_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/session"
)

// The server records answers through the flow's own bell, so the flow acts on
// each at once: with an hour between reads, any wait that missed the bell would
// outlast the walk.
func TestTheFlowWakesOnAnAnswerWithoutWaitingOutAPoll(t *testing.T) {
	w := newWorld(t)
	w.store.Bell = session.NewBell()
	cfg := w.config()
	cfg.Poll = time.Hour
	began := time.Now()
	if err := w.walkWith(cfg); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > 10*time.Second {
		t.Errorf("the walk took %v", took)
	}
}

// The agent's resume comes from another process, which can't ring the server's
// bell, so the flow still finds it by reading the log.
func TestAResumeFromAnotherProcessStillReachesTheFlow(t *testing.T) {
	w := newWorld(t)
	w.mac.On("brew install bensyverson/tap/job", flow.Result{Exit: 1}, 1)
	w.resume = func(session.Handoff) session.Way { return session.WayRetry }
	cfg := w.config()
	cfg.Store.Bell = session.NewBell()
	cfg.Poll = 20 * time.Millisecond
	if err := w.walkWith(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.mac.Root, "/opt/homebrew/bin/job")); err != nil {
		t.Error("the retry didn't install jobs")
	}
}
