package screen

import (
	"slices"
	"testing"
)

// A screen with a box under a long status is too tall for the space below the
// object, and would run under the progress line, so it sits low like naming.
func TestTallScreensSitLow(t *testing.T) {
	for _, id := range []ID{"naming", "identity-name", "identity-email", "github-code", "identity"} {
		def, ok := find(id)
		if !ok {
			t.Fatalf("no screen %s", id)
		}
		if def.layout != LayoutLow {
			t.Errorf("%s has layout %q, want %q", id, def.layout, LayoutLow)
		}
	}
}

// Ben, 2026-11-05: as on a Mac, the main button sits on the far right and the
// way out on the far left; the sign-up link goes between them.
func TestTheMainButtonSitsRightAndTheWayOutLeft(t *testing.T) {
	for _, tc := range []struct {
		req  Request
		want []string
	}{
		{Request{Screen: "github-signup"}, []string{"not-now", "signup", "made-it"}},
		{Request{Screen: "github"}, []string{"not-now", "make-one", "yes"}},
		{Request{Screen: "settings"}, []string{"not-now", "yes"}},
		{Request{Screen: "welcome"}, []string{"skip-tour", "see-plan"}},
		{Request{Screen: "xcode"}, []string{"later", "app-store"}},
		{Request{Screen: "identity-name"}, []string{"skip", "next"}},
		{Request{Screen: "trouble", Tool: "sleepy", Reason: "other"}, []string{"ask", "skip", "retry"}},
		// Answers of equal weight keep their order.
		{Request{Screen: "making"}, []string{"website", "app", "code", "other"}},
	} {
		d, err := Describe(tc.req, Context{Agent: ClaudeCode})
		if err != nil {
			t.Fatalf("%s: %v", tc.req.Screen, err)
		}
		var got []string
		for _, c := range d.Choices {
			got = append(got, c.ID)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s buttons = %v, want %v", tc.req.Screen, got, tc.want)
		}
		if tc.req.Screen != "making" && !d.Choices[len(d.Choices)-1].Primary {
			t.Errorf("%s: the main button isn't on the right: %+v", tc.req.Screen, d.Choices)
		}
	}
}

// Ben, 2026-11-05: answers of equal weight have no main button.
func TestEqualAnswersHaveNoMainButton(t *testing.T) {
	for _, id := range []ID{"making"} {
		d, err := Describe(Request{Screen: id}, Context{Agent: ClaudeCode})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range d.Choices {
			if c.Primary {
				t.Errorf("%s: %s is a main button", id, c.ID)
			}
		}
	}
}
