package flow_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/session"
)

// raises counts the open -a calls that try to bring Apple's installer forward.
func raises(w *world) int {
	n := 0
	for _, c := range w.mac.Changes() {
		if strings.HasPrefix(c, "open -a ") {
			n++
		}
	}
	return n
}

// Criterion RnU: in the second clean-Mac trial, open -a failed with launchd's
// "Launch failed" while Apple's installer was still starting, and the flow
// handed off as trouble (project/2026-11-06-clean-mac-trial-findings.md).
func TestAppleWindowThatWontRaiseIsNotTrouble(t *testing.T) {
	w := newWorld(t)
	w.running = func(string) bool { return true }
	w.mac.On("open -a", flow.Result{Exit: 1, Output: []byte("Launch failed.")}, 0)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	state := w.state()
	if state.Finished[flow.StepApple] != session.StepDone || state.Finished[flow.StepDone] != session.StepDone || state.Open != nil {
		t.Errorf("finished = %v, open = %+v", state.Finished, state.Open)
	}
	if n := raises(w); n < 2 {
		t.Errorf("open -a ran %d times, want it retried", n)
	}
}

// Criterion 2ZA: open -a waits for Apple's installer to start, and is tried
// again when the first try fails.
func TestAppleWindowRaisesOnceTheInstallerRuns(t *testing.T) {
	w := newWorld(t)
	var mu sync.Mutex
	polls, raisesBeforeUp := 0, -1
	w.running = func(string) bool {
		mu.Lock()
		defer mu.Unlock()
		polls++
		if polls == 3 {
			raisesBeforeUp = raises(w)
		}
		return polls >= 3
	}
	w.mac.On("open -a", flow.Result{Exit: 1, Output: []byte("Launch failed.")}, 1)
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if raisesBeforeUp != 0 {
		t.Errorf("open -a ran %d times before the installer was up", raisesBeforeUp)
	}
	if n := raises(w); n != 2 {
		t.Errorf("open -a ran %d times, want one failure and one retry", n)
	}
	if state := w.state(); state.Finished[flow.StepDone] != session.StepDone || state.Open != nil {
		t.Errorf("finished = %v, open = %+v", state.Finished, state.Open)
	}
}
