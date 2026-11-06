package screen

import (
	"slices"
	"testing"
)

func places(t *testing.T, in string) []Place {
	t.Helper()
	d, err := Describe(Request{Screen: "naming", In: in}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if d.Places == nil {
		t.Fatal("naming offers no places")
	}
	return d.Places.Options
}

func TestNamingOffersProjectsAndGit(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Place
	}{
		{"", []Place{{Path: "~/Projects", Picked: true}, {Path: "~/git"}}},
		{"~/Projects", []Place{{Path: "~/Projects", Picked: true}, {Path: "~/git"}}},
		{"~/git", []Place{{Path: "~/Projects"}, {Path: "~/git", Picked: true}}},
		{"/Volumes/work", []Place{{Path: "~/Projects"}, {Path: "~/git"}, {Path: "/Volumes/work", Picked: true}}},
	} {
		if got := places(t, tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("in %q: places = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestOnlyNamingTakesAFolder(t *testing.T) {
	if err := Validate(Request{Screen: "making", In: "~/git"}); err == nil {
		t.Error("making took a folder")
	}
	if d, _ := Describe(Request{Screen: "making"}, Context{Agent: ClaudeCode}); d.Places != nil {
		t.Error("making offers places")
	}
}

func TestSafePlace(t *testing.T) {
	good := []string{"~/git", "~/Projects", "~/code/work", "/Volumes/work", "/opt/src"}
	bad := []string{"", "~", "/", "git", "./git", "~/My Projects", "~/../etc", "~/git/", "~//git", "/a/$(x)", "~user/git", "~/a\nb"}
	for _, p := range good {
		if !SafePlace(p) {
			t.Errorf("%q should be safe", p)
		}
		if err := Validate(Request{Screen: "naming", In: p}); err != nil {
			t.Errorf("naming in %q: %v", p, err)
		}
	}
	for _, p := range bad {
		if SafePlace(p) {
			t.Errorf("%q should be refused", p)
		}
		if p != "" && Validate(Request{Screen: "naming", In: p}) == nil {
			t.Errorf("naming took %q", p)
		}
	}
}

func TestAnAnswersFolderIsOneOffered(t *testing.T) {
	d, err := Describe(Request{Screen: "naming", In: "~/git"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		in string
		ok bool
	}{{"~/git", true}, {"~/Projects", true}, {"", true}, {"/etc", false}} {
		if err := ValidateAnswer(d, Answer{Choice: "next", Name: "pine-bakery", In: tc.in}); (err == nil) != tc.ok {
			t.Errorf("in %q: err = %v", tc.in, err)
		}
	}
	making, _ := Describe(Request{Screen: "making"}, Context{Agent: ClaudeCode})
	if ValidateAnswer(making, Answer{Choice: "website", In: "~/git"}) == nil {
		t.Error("a folder answered making")
	}
}

func TestPickedPlace(t *testing.T) {
	d, _ := Describe(Request{Screen: "naming", In: "~/git"}, Context{Agent: ClaudeCode})
	if got := d.Places.Picked(); got != "~/git" {
		t.Errorf("picked = %q", got)
	}
}
