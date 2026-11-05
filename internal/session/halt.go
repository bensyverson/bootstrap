package session

import (
	"errors"
	"fmt"
	"time"
)

// ErrFlowStopped is a flow that stopped before the end, as a halt records it.
var ErrFlowStopped = errors.New("the setup stopped")

// Halt records that the flow stopped early, and why, so a wait can say so
// instead of waiting on a flow that's gone. A new drive clears it.
func (s Store) Halt(why string, now time.Time) (State, error) {
	return s.appendLive(now, func(State) (event, error) { return event{Kind: kindHalt, Text: why}, nil })
}

func haltErr(state State) error { return fmt.Errorf("%w: %s", ErrFlowStopped, state.Halted) }
