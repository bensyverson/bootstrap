package screen

import (
	"strings"
	"testing"
)

func TestValidateAnswer(t *testing.T) {
	ctx := Context{Agent: ClaudeCode}
	describe := func(req Request) Description {
		d, err := Describe(req, ctx)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	welcome, list, naming := describe(Request{Screen: "welcome"}), describe(Request{Screen: "list"}), describe(Request{Screen: "naming"})
	cases := []struct {
		name string
		desc Description
		ans  Answer
		ok   bool
	}{
		{"a real choice", welcome, Answer{Choice: "see-plan"}, true},
		{"an unknown choice", welcome, Answer{Choice: "nope"}, false},
		{"tools on a screen with no rows", welcome, Answer{Choice: "see-plan", Tools: []string{"jobs"}}, false},
		{"a name on a screen with no field", welcome, Answer{Choice: "see-plan", Name: "x"}, false},
		{"chosen tools", list, Answer{Choice: "lets-go", Tools: []string{"sleepy"}}, true},
		{"an unknown tool", list, Answer{Choice: "lets-go", Tools: []string{"rm"}}, false},
		{"a safe name", naming, Answer{Choice: "next", Name: "pine-bakery"}, true},
		{"no name", naming, Answer{Choice: "next"}, false},
		{"an unsafe name", naming, Answer{Choice: "next", Name: "../etc"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateAnswer(c.desc, c.ans); (err == nil) != c.ok {
				t.Errorf("err = %v, want ok = %v", err, c.ok)
			}
		})
	}
}

func TestSafeFolderName(t *testing.T) {
	good := []string{"my-project", "pine_bakery", "Pine.Bakery", "a", "project2", strings.Repeat("a", 64)}
	bad := []string{"", ".", "..", ".hidden", "-rf", "a/b", "a\\b", "a b", "a;b", "$(x)", "`x`", "a\nb", "a'b", "a\"b", "~", "é", strings.Repeat("a", 65), "a|b", "a&b", "a*b"}
	for _, name := range good {
		if !SafeFolderName(name) {
			t.Errorf("%q should be safe", name)
		}
	}
	for _, name := range bad {
		if SafeFolderName(name) {
			t.Errorf("%q should be refused", name)
		}
	}
}

// People type names the way they'd say them, so the folder name is made git-friendly
// for them, rather than refusing "Pine Bakery" with an error.
func TestFolderName(t *testing.T) {
	cases := []struct {
		typed, want string
		ok          bool
	}{
		{"Pine Bakery", "pine-bakery", true},
		{"  My   App  ", "my-app", true},
		{"my-project", "my-project", true},
		{"v2.site_new", "v2.site_new", true},
		{"../etc", "", false},
		{"a; rm -rf x", "", false},
		{"café", "", false},
		{"", "", false},
		{"   ", "", false},
		{".hidden", "", false},
	}
	for _, c := range cases {
		got, ok := FolderName(c.typed)
		if got != c.want || ok != c.ok {
			t.Errorf("FolderName(%q) = %q, %v; want %q, %v", c.typed, got, ok, c.want, c.ok)
		}
	}
}

// With no button to move it on, this one screen has to say the wait can be long.
func TestAppleAskSaysItCanTakeAWhile(t *testing.T) {
	agent, err := FindAgent("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	desc, err := Describe(Request{Screen: "apple-ask"}, Context{Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain(desc.Status), "This can take a while.") {
		t.Errorf("status = %q", plain(desc.Status))
	}
}

// Ben's ruling: the title is the news, and the instruction rides under it.
func TestDoneSplitsItsTitleFromItsInstruction(t *testing.T) {
	agent, err := FindAgent("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	desc, err := Describe(Request{Screen: "done"}, Context{Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	if got := plain(desc.Title); got != "All set!" {
		t.Errorf("title = %q", got)
	}
	if got := plain(desc.Status); got != "You can close this and go back to Claude Code." {
		t.Errorf("status = %q", got)
	}
}
