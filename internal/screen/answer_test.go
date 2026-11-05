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
