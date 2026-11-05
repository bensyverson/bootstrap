package screen

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	sampleCode  = "D7BA-CFE2"
	sampleName  = "Ada Lovelace"
	sampleEmail = "1234+ada@users.noreply.github.com"
)

func TestTheCodeScreenShowsGhsCode(t *testing.T) {
	d, err := Describe(Request{Screen: "github-code", Code: sampleCode}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if d.Code != sampleCode {
		t.Errorf("code = %q", d.Code)
	}
	if !slices.ContainsFunc(d.Choices, func(c Choice) bool { return c.URL == "https://github.com/login/device" }) {
		t.Errorf("no button to GitHub's device page: %+v", d.Choices)
	}
	// The flow waits on gh, not on a click, so the lost screen can't hold it back.
	if AwaitsClick("github-code") {
		t.Error("the code screen has nothing to answer")
	}
}

func TestALinkIsNotAnAnswer(t *testing.T) {
	d, err := Describe(Request{Screen: "github-code", Code: sampleCode}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.Choices {
		if err := ValidateAnswer(d, Answer{Choice: c.ID}); err == nil {
			t.Errorf("a link button %q was taken as an answer", c.ID)
		}
	}
}

func TestTheIdentityOfferNamesWhatItWillSet(t *testing.T) {
	d, err := Describe(Request{Screen: "identity", Name: sampleName, Email: sampleEmail}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	words := plain(d.Title) + " " + plain(d.Status)
	for _, want := range []string{sampleName, sampleEmail} {
		if !strings.Contains(words, want) {
			t.Errorf("the offer doesn't show %q: %s", want, words)
		}
	}
	if !slices.ContainsFunc(d.Choices, func(c Choice) bool { return c.ID == "skip" }) {
		t.Errorf("the offer can't be skipped: %+v", d.Choices)
	}
}

// A name from GitHub is the person's own, so it goes on the page as it is, never
// read for the app's names.
func TestANameIsNotFilledIn(t *testing.T) {
	d, err := Describe(Request{Screen: "identity", Name: "{app}", Email: sampleEmail}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain(d.Title)+plain(d.Status), "{app}") {
		t.Errorf("the name was filled in: %s %s", plain(d.Title), plain(d.Status))
	}
}

func TestValidateTheFlowsOwnDetails(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		ok   bool
	}{
		{"a code", Request{Screen: "github-code", Code: sampleCode}, true},
		{"no code", Request{Screen: "github-code"}, false},
		{"a code that isn't gh's shape", Request{Screen: "github-code", Code: "click here"}, false},
		{"a code on a screen that takes none", Request{Screen: "github", Code: sampleCode}, false},
		{"an offer", Request{Screen: "identity", Name: sampleName, Email: sampleEmail}, true},
		{"an offer with no name", Request{Screen: "identity", Email: sampleEmail}, false},
		{"an offer that isn't GitHub's private address", Request{Screen: "identity", Name: sampleName, Email: "ada@example.com"}, false},
		{"a name git would read as an option", Request{Screen: "identity", Name: "-x", Email: sampleEmail}, false},
		{"a name across two lines", Request{Screen: "identity", Name: "Ada\nLovelace", Email: sampleEmail}, false},
		{"a name on a screen that takes none", Request{Screen: "welcome", Name: sampleName}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.req)
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok = %v", err, c.ok)
			}
			var inv *InvalidError
			if err != nil && !errors.As(err, &inv) {
				t.Errorf("err = %v, want InvalidError", err)
			}
		})
	}
}

func TestTypedNamesAndEmails(t *testing.T) {
	ctx := Context{Agent: ClaudeCode}
	name, err := Describe(Request{Screen: "identity-name"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	email, err := Describe(Request{Screen: "identity-email"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if name.Field == nil || name.Field.Kind != FieldName || email.Field == nil || email.Field.Kind != FieldEmail {
		t.Fatalf("fields = %+v, %+v", name.Field, email.Field)
	}
	cases := []struct {
		name string
		desc Description
		ans  Answer
		ok   bool
	}{
		{"a name with a space", name, Answer{Choice: "next", Name: sampleName}, true},
		{"a name with an accent", name, Answer{Choice: "next", Name: "Zoë Brontë"}, true},
		{"no name", name, Answer{Choice: "next", Name: "  "}, false},
		{"a name git would read as an option", name, Answer{Choice: "next", Name: "--global"}, false},
		{"a name across two lines", name, Answer{Choice: "next", Name: "Ada\nLovelace"}, false},
		{"skipping with nothing typed", name, Answer{Choice: "skip"}, true},
		{"an email", email, Answer{Choice: "next", Name: "ada@example.com"}, true},
		{"not an email", email, Answer{Choice: "next", Name: "ada"}, false},
		{"an email with a space", email, Answer{Choice: "next", Name: "ada lovelace@example.com"}, false},
		{"skipping the email", email, Answer{Choice: "skip", Name: "half typed"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateAnswer(c.desc, c.ans); (err == nil) != c.ok {
				t.Errorf("err = %v, want ok = %v", err, c.ok)
			}
		})
	}
}

func TestTheNamingFieldIsStillAFolder(t *testing.T) {
	d, err := Describe(Request{Screen: "naming"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if d.Field == nil || d.Field.Kind != FieldFolder {
		t.Errorf("field = %+v", d.Field)
	}
	if !d.Field.Folder() {
		t.Error("the naming field names a folder")
	}
}

func TestGitHubOffersToMakeAnAccount(t *testing.T) {
	d, err := Describe(Request{Screen: "github"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range d.Choices {
		ids = append(ids, c.ID)
	}
	if !slices.Equal(ids, []string{"yes", "make-one", "not-now"}) {
		t.Errorf("choices = %v", ids)
	}
	signup, err := Describe(Request{Screen: "github-signup"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(signup.Choices, func(c Choice) bool { return c.URL == "https://github.com/signup" }) {
		t.Errorf("no way back to the sign-up page: %+v", signup.Choices)
	}
	if !AwaitsClick("github-signup") {
		t.Error("the sign-up screen waits for the person to say they're done")
	}
}
