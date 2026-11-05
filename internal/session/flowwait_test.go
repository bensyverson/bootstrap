package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// serving holds the server slot, as a running server does, until the test ends.
func serving(t *testing.T, store Store) {
	t.Helper()
	release, err := store.LockServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

// must fails the test on a write's error: must(t)(store.Note(…)).
func must(t *testing.T) func(State, error) {
	return func(_ State, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnreadListsTheFlowsWorkInOrder(t *testing.T) {
	store := driven(t)
	must(t)(store.RecordStep(Step{ID: "tools", Status: StepStarted, Index: 4, Of: 13}, t0))
	must(t)(store.Note("installed jobs", t0))
	must(t)(store.HandOff(trouble("sleepy", WayRetry, WaySkip, WayTalk), t0))
	_, updates, err := store.Unread()
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 3 {
		t.Fatalf("updates = %+v", updates)
	}
	if updates[0].Step == nil || updates[0].Step.ID != "tools" || updates[1].Note != "installed jobs" || updates[2].Handoff == nil || updates[2].Handoff.Tool != "sleepy" {
		t.Errorf("updates = %+v", updates)
	}
	if !(updates[0].Seq < updates[1].Seq && updates[1].Seq < updates[2].Seq) {
		t.Errorf("updates aren't in log order: %+v", updates)
	}
}

func TestMarkToldMovesTheCursor(t *testing.T) {
	store := driven(t)
	must(t)(store.Note("one", t0))
	state, _ := store.Note("two", t0)
	if err := store.MarkTold(state.Seq, t0); err != nil {
		t.Fatal(err)
	}
	must(t)(store.Note("three", t0))
	_, updates, err := store.Unread()
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].Note != "three" {
		t.Errorf("updates = %+v", updates)
	}
}

func TestMarkToldCountsAsTheAgentCheckingIn(t *testing.T) {
	store := driven(t)
	later := t0.Add(time.Hour)
	if err := store.MarkTold(1, later); err != nil {
		t.Fatal(err)
	}
	if got := load(t, store).Deadline; !got.Equal(later.Add(DefaultAllowance)) {
		t.Errorf("deadline = %v", got)
	}
}

func TestWaitFlowReturnsAtATroubleHandoffAndTakesIt(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.Note("installed jobs", t0))
	must(t)(store.HandOff(trouble("sleepy", WayRetry, WaySkip, WayTalk), t0))
	told, err := store.WaitFlow(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(told.Updates) != 2 || told.Open == nil || told.Open.Handoff.Tool != "sleepy" {
		t.Errorf("told = %+v", told)
	}
	if state := load(t, store); state.Open == nil || !state.Open.Taken {
		t.Errorf("the handoff wasn't taken: %+v", state.Open)
	}
}

func TestWaitFlowRepeatsAnOpenHandoffWithNothingNew(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.HandOff(trouble("sleepy", WayRetry, WayTalk), t0))
	if _, err := store.WaitFlow(context.Background(), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	told, err := store.WaitFlow(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(told.Updates) != 0 || told.Open == nil || time.Since(began) > time.Second {
		t.Errorf("told = %+v after %v", told, time.Since(began))
	}
}

func TestWaitFlowReturnsAtAHandoffThatDoesntBlock(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.HandOff(Handoff{Kind: HandoffPhone, Step: "phone", What: "explain /remote-control", Ways: []Way{WayTalk}}, t0))
	told, err := store.WaitFlow(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(told.Updates) != 1 || told.Updates[0].Handoff.Kind != HandoffPhone || told.Open != nil {
		t.Errorf("told = %+v", told)
	}
}

func TestWaitFlowRunsOutOfBudgetWithWhatHappened(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.Note("installed jobs", t0))
	told, err := store.WaitFlow(context.Background(), 300*time.Millisecond)
	if !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("err = %v", err)
	}
	if len(told.Updates) != 1 || told.Updates[0].Note != "installed jobs" {
		t.Errorf("told = %+v", told)
	}
	if _, updates, _ := store.Unread(); len(updates) != 0 {
		t.Errorf("a told note is told again: %+v", updates)
	}
}

func TestWaitFlowAfterTheEndReturnsAtOnce(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.HandOff(Handoff{Kind: HandoffDone, Step: "done", What: "sum it up"}, t0))
	if _, err := store.WaitFlow(context.Background(), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	told, err := store.WaitFlow(context.Background(), 5*time.Second)
	if err != nil || !told.Finished || len(told.Updates) != 0 || time.Since(began) > time.Second {
		t.Errorf("told = %+v, err = %v, after %v", told, err, time.Since(began))
	}
}

func TestWaitFlowWithTheServerGone(t *testing.T) {
	store := driven(t)
	if _, err := store.WaitFlow(context.Background(), 5*time.Second); !errors.Is(err, ErrPageGone) {
		t.Errorf("err = %v", err)
	}
}

func TestWaitFlowAfterStop(t *testing.T) {
	store := driven(t)
	serving(t, store)
	must(t)(store.Stop(t0))
	if _, err := store.WaitFlow(context.Background(), 5*time.Second); !errors.Is(err, ErrEnded) {
		t.Errorf("err = %v", err)
	}
}
