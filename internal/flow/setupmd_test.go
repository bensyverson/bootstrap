package flow_test

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/session"
)

// SETUP.md's chat-only setup is a second copy of the flow, in prose, so its steps
// must stay the flow's steps, in the flow's order.
func TestTheChatOnlySetupFollowsTheFlowsSteps(t *testing.T) {
	body, err := os.ReadFile("../../SETUP.md")
	if err != nil {
		t.Fatal(err)
	}
	var inDoc []session.StepID
	for _, m := range regexp.MustCompile("(?m)^### \\d+\\. .* \\(`([a-z]+)`\\)$").FindAllStringSubmatch(string(body), -1) {
		inDoc = append(inDoc, session.StepID(m[1]))
	}
	var inFlow []session.StepID
	for _, s := range flow.Steps() {
		inFlow = append(inFlow, s.ID)
	}
	if !slices.Equal(inDoc, inFlow) {
		t.Errorf("SETUP.md's chat-only steps are %v, but the flow's are %v", inDoc, inFlow)
	}
}
