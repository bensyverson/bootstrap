package session

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

func driven(t *testing.T) Store {
	t.Helper()
	store := begun(t)
	if _, err := store.Drive(t0); err != nil {
		t.Fatal(err)
	}
	return store
}

func trouble(tool string, ways ...Way) Handoff {
	return Handoff{
		Kind: HandoffTrouble, Step: "tools", Tool: tool, Reason: ReasonExit,
		What:     "brew install bensyverson/tap/" + tool + " exited 1",
		Evidence: Evidence{Command: "brew install bensyverson/tap/" + tool, Exit: 1, Log: "/tmp/x.log", Tail: []string{"Error: no bottle"}},
		Ways:     ways,
	}
}

func TestStepsNotesAndHandoffsReplay(t *testing.T) {
	store := driven(t)
	if _, err := store.RecordStep(Step{ID: "tools", Status: StepStarted, Index: 4, Of: 13}, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Note("installed jobs", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.HandOff(trouble("sleepy", WayRetry, WaySkip, WayTalk), t0); err != nil {
		t.Fatal(err)
	}
	state := load(t, store)
	if !state.Driving {
		t.Error("the flow said it drives")
	}
	if state.Step == nil || state.Step.ID != "tools" || state.Step.Index != 4 || state.Step.Of != 13 {
		t.Errorf("step = %+v", state.Step)
	}
	if len(state.Notes) != 1 || state.Notes[0].Text != "installed jobs" || state.Notes[0].Seq == 0 {
		t.Errorf("notes = %+v", state.Notes)
	}
	if state.Open == nil || state.Open.Handoff.Tool != "sleepy" || state.Open.Handoff.Evidence.Tail[0] != "Error: no bottle" {
		t.Errorf("open = %+v", state.Open)
	}
	if len(state.Handoffs) != 1 || state.Handoffs[0].Seq != state.Open.Seq {
		t.Errorf("handoffs = %+v", state.Handoffs)
	}
}

func TestFinishedStepsAreKept(t *testing.T) {
	store := driven(t)
	for _, step := range []Step{
		{ID: "settings", Status: StepStarted},
		{ID: "settings", Status: StepDone},
		{ID: "phone", Status: StepStarted},
		{ID: "phone", Status: StepSkipped},
		{ID: "done", Status: StepStarted},
	} {
		if _, err := store.RecordStep(step, t0); err != nil {
			t.Fatal(err)
		}
	}
	got := load(t, store).Finished
	if len(got) != 2 || got["settings"] != StepDone || got["phone"] != StepSkipped {
		t.Errorf("finished = %v", got)
	}
}

func TestANewServerStopsTheDriving(t *testing.T) {
	store := driven(t)
	if _, err := store.Begin(screen.ClaudeCode, 2, t0); err != nil {
		t.Fatal(err)
	}
	if load(t, store).Driving {
		t.Error("a restarted server drives only if its flow says so again")
	}
}

func TestResumeClosesTheHandoff(t *testing.T) {
	store := driven(t)
	if _, err := store.HandOff(trouble("sleepy", WayRetry, WaySkip, WayTalk), t0); err != nil {
		t.Fatal(err)
	}
	state, err := store.Resume(WayRetry, t0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Open != nil || state.Resumed == nil || state.Resumed.Way != WayRetry || state.Resumed.For != state.Handoffs[0].Seq {
		t.Errorf("open = %+v, resumed = %+v", state.Open, state.Resumed)
	}
}

func TestResumeSkipDropsTheTool(t *testing.T) {
	store := driven(t)
	if _, err := store.HandOff(trouble("sleepy", WayRetry, WaySkip, WayTalk), t0); err != nil {
		t.Fatal(err)
	}
	state, err := store.Resume(WaySkip, t0)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(state.Chosen, "sleepy") || !slices.Contains(state.Chosen, "jobs") {
		t.Errorf("chosen = %v", state.Chosen)
	}
}

func TestResumeRefusals(t *testing.T) {
	store := driven(t)
	if _, err := store.Resume(WayRetry, t0); !errors.Is(err, ErrNoHandoff) {
		t.Errorf("nothing to resume: %v", err)
	}
	if _, err := store.HandOff(trouble("jobs", WayRetry, WayTalk), t0); err != nil {
		t.Fatal(err)
	}
	var invalid *screen.InvalidError
	if _, err := store.Resume(WaySkip, t0); !errors.As(err, &invalid) || !slices.Equal(invalid.Valid, []string{"retry"}) {
		t.Errorf("skip wasn't offered: %v", err)
	}
	if _, err := store.Resume(WayTalk, t0); !errors.As(err, &invalid) {
		t.Errorf("talking it over is done in the chat, not by resume: %v", err)
	}
}

func TestOneBlockingHandoffAtATime(t *testing.T) {
	store := driven(t)
	if _, err := store.HandOff(trouble("jobs", WayRetry), t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.HandOff(trouble("sleepy", WayRetry), t0); !errors.Is(err, ErrHandoffOpen) {
		t.Errorf("err = %v", err)
	}
	if _, err := store.HandOff(Handoff{Kind: HandoffPhone, Step: "phone", What: "the person wants to use their phone", Ways: []Way{WayTalk}}, t0); err != nil {
		t.Errorf("a task handoff doesn't block: %v", err)
	}
}

func TestHandoffMustTeach(t *testing.T) {
	store := driven(t)
	for name, h := range map[string]Handoff{
		"no ways":     {Kind: HandoffTrouble, Step: "tools", Reason: ReasonExit, What: "x exited 1"},
		"no what":     {Kind: HandoffTrouble, Step: "tools", Reason: ReasonExit, Ways: []Way{WayRetry}},
		"no reason":   {Kind: HandoffTrouble, Step: "tools", What: "x", Ways: []Way{WayRetry}},
		"odd kind":    {Kind: "odd", Step: "tools", What: "x", Ways: []Way{WayRetry}},
		"odd reason":  {Kind: HandoffTrouble, Step: "tools", Reason: "odd", What: "x", Ways: []Way{WayRetry}},
		"odd way":     {Kind: HandoffTrouble, Step: "tools", Reason: ReasonExit, What: "x", Ways: []Way{"odd"}},
		"no step":     {Kind: HandoffTrouble, Reason: ReasonExit, What: "x", Ways: []Way{WayRetry}},
		"task reason": {Kind: HandoffDone, Step: "done", Reason: ReasonExit, What: "x"},
	} {
		if _, err := store.HandOff(h, t0); err == nil {
			t.Errorf("%s: accepted %+v", name, h)
		}
	}
}

func TestLostDuringTheFlowOnlyOnAnUntakenHandoff(t *testing.T) {
	store := driven(t)
	show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, t0)
	hour := t0.Add(2 * time.Hour)
	if state := load(t, store); state.Lost(hour) || state.PastLifetime(hour) {
		t.Error("while the server drives, the page needs nothing from the agent")
	}
	if _, err := store.HandOff(trouble("sleepy", WayRetry), t0); err != nil {
		t.Fatal(err)
	}
	show(t, store, screen.Request{Screen: "chat"}, t0)
	state := load(t, store)
	if state.Lost(t0.Add(DefaultAllowance)) {
		t.Error("not lost within the allowance")
	}
	if !state.Lost(t0.Add(DefaultAllowance + time.Second)) {
		t.Error("an agent that never picks up a handoff is gone, even on the chat screen")
	}
	if _, err := store.TakeHandoff(t0); err != nil {
		t.Fatal(err)
	}
	if load(t, store).Lost(hour) {
		t.Error("once the agent has it, the chat screen waits on the person")
	}
}

func TestTakeNeedsAnOpenHandoff(t *testing.T) {
	store := driven(t)
	if _, err := store.TakeHandoff(t0); !errors.Is(err, ErrNoHandoff) {
		t.Errorf("err = %v", err)
	}
}

func TestFlowEventsNeedALiveSession(t *testing.T) {
	store := begun(t)
	if _, err := store.Stop(t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Drive(t0); !errors.Is(err, ErrEnded) {
		t.Errorf("drive: %v", err)
	}
	if _, err := store.Note("x", t0); !errors.Is(err, ErrEnded) {
		t.Errorf("note: %v", err)
	}
}
