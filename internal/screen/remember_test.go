package screen

import (
	"slices"
	"strings"
	"testing"
)

// A remembered answer is the page's default: its main button (XbdzWw).
func TestARequestsDefaultIsItsMainButton(t *testing.T) {
	d, err := Describe(Request{Screen: "github", Default: "not-now"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	var main []string
	for _, c := range d.Choices {
		if c.Primary {
			main = append(main, c.ID)
		}
	}
	if !slices.Equal(main, []string{"not-now"}) {
		t.Errorf("main buttons = %v, want only not-now", main)
	}
	// The making screen has no main button of its own, but a default still is one.
	d, err = Describe(Request{Screen: "making", Default: "app"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.IndexFunc(d.Choices, func(c Choice) bool { return c.Primary }); i < 0 || d.Choices[i].ID != "app" {
		t.Errorf("making's choices = %+v, want app as the main one", d.Choices)
	}
}

func TestADefaultMustBeOneOfTheScreensChoices(t *testing.T) {
	for _, req := range []Request{{Screen: "github", Default: "maybe"}, {Screen: "fast", Default: "yes"}} {
		err := Validate(req)
		if invalid, ok := err.(*InvalidError); !ok || !strings.Contains(invalid.Message, "default") {
			t.Errorf("Validate(%+v) = %v, want one about the default", req, err)
		}
	}
}

// The naming screen's way to skip the project and keep just the tools (VGCn9C).
func TestNamingOffersJustTheTools(t *testing.T) {
	d, err := Describe(Request{Screen: "naming"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(d.Choices, func(c Choice) bool { return c.ID == JustTools && !c.Primary }) {
		t.Errorf("naming's choices = %+v, want %s beside the main one", d.Choices, JustTools)
	}
	if err := ValidateAnswer(d, Answer{Choice: JustTools}); err != nil {
		t.Errorf("just the tools needs no name, but: %v", err)
	}
	if err := ValidateAnswer(d, Answer{Choice: JustTools, In: "~/git"}); err != nil {
		t.Errorf("a folder picked before choosing just the tools is fine, but: %v", err)
	}
}

func TestDoneSaysHowToMakeAProjectLater(t *testing.T) {
	plainDone, err := Describe(Request{Screen: "done"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	tools, err := Describe(Request{Screen: "done", Status: DoneJustTools}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if plain(tools.Status) == plain(plainDone.Status) || !strings.Contains(plain(tools.Status), "project") {
		t.Errorf("done with just the tools says %q, want how to make a project later", plain(tools.Status))
	}
	// Ben, 2026-11-06: "ask Claude to run bootstrap new".
	if !strings.Contains(plain(tools.Status), "ask Claude to run bootstrap new") {
		t.Errorf("done with just the tools says %q, want it to say to ask Claude", plain(tools.Status))
	}
}
