package cli

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/screen"
)

// question is one with a default, as most of the flow's are.
func question() flow.Question {
	return flow.Question{
		Request: screen.Request{Screen: "settings"},
		Screen:  screen.Description{},
		Default: &screen.Answer{Choice: "yes"},
	}
}

// A question that reaches the terminal with no one to ask and no --yes is
// refused, never defaulted: the up-front list of open questions is a courtesy,
// and this is what keeps a question it misses from installing anything.
func TestNoOneToAskAndNoYesRefusesEvenAQuestionWithADefault(t *testing.T) {
	term := &terminal{out: io.Discard, answering: refuseQuestions}
	_, err := term.Ask(context.Background(), question())
	if _, ok := errors.AsType[*unanswered](err); !ok {
		t.Fatalf("Ask = %v, want an unanswered refusal", err)
	}
}

func TestYesTakesTheDefault(t *testing.T) {
	term := &terminal{out: io.Discard, answering: takeDefaults}
	a, err := term.Ask(context.Background(), question())
	if err != nil || a.Choice != "yes" {
		t.Fatalf("Ask = %+v, %v; want the default", a, err)
	}
}
