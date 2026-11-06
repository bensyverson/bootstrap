package flow_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// Ben, 2026-11-05: the rules add the modules the project's kind calls for, and
// a new project starts at stage-build. A project that's already there gets no stage.
func TestTheRulesFitWhatThePersonIsMaking(t *testing.T) {
	for _, tc := range []struct {
		making, want string
		inPlace      bool
	}{
		{"website", "agents add web stage-build", false},
		{"app", "agents add swift swiftui stage-build", false},
		{"code", "agents add stage-build", false},
		{"other", "agents add stage-build", false},
		{"website", "agents add web", true},
	} {
		w := newWorld(t)
		w.answers[asked{"making", ""}] = screen.Answer{Choice: tc.making}
		w.answers[asked{"xcode", ""}] = screen.Answer{Choice: "later"}
		if tc.inPlace {
			start := filepath.Join(w.home, "code", "bakery")
			put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
			put(t, filepath.Join(start, "index.html"), "<h1>Pine</h1>\n")
			w.probe.Cwd = start
		}
		if err := w.walk(); err != nil {
			t.Fatal(err)
		}
		changes := w.mac.Changes()
		var adds []string
		for i, c := range changes {
			if strings.HasPrefix(c, "agents add") {
				adds = append(adds, c)
				if i == 0 || !strings.HasPrefix(changes[i-1], "agents init") {
					t.Errorf("%s: %q didn't follow agents init", tc.making, c)
				}
			}
		}
		if len(adds) != 1 || adds[0] != tc.want {
			t.Errorf("%s (in place %v): ran %q, want %q", tc.making, tc.inPlace, adds, tc.want)
		}
	}
}

// Ben, 2026-11-06: a project set up in place also gets the rules for the
// languages its files show, alongside what the making answer calls for.
func TestTheRulesAddTheStacksAProjectShows(t *testing.T) {
	for _, tc := range []struct {
		name, making string
		files        []string
		want         []string
	}{
		{"a Go project", "other", []string{"go.mod"}, []string{"agents add go"}},
		{"a Python website", "website", []string{"pyproject.toml"}, []string{"agents add web python"}},
		{"a Swift app adds swift once", "app", []string{"Package.swift", "go.mod"}, []string{"agents add swift swiftui go"}},
		{"a Swift package called something else", "other", []string{"Package.swift"}, []string{"agents add swift"}},
		{"Go and Python", "other", []string{"requirements.txt", "go.mod"}, []string{"agents add go python"}},
	} {
		w := newWorld(t)
		w.answers[asked{"making", ""}] = screen.Answer{Choice: tc.making}
		w.answers[asked{"xcode", ""}] = screen.Answer{Choice: "later"}
		start := filepath.Join(w.home, "code", "bakery")
		put(t, filepath.Join(start, ".git", "HEAD"), "ref: refs/heads/main\n")
		for _, f := range tc.files {
			put(t, filepath.Join(start, f), "\n")
		}
		w.probe.Cwd = start
		if err := w.walk(); err != nil {
			t.Fatal(err)
		}
		var adds []string
		for _, c := range w.mac.Changes() {
			if strings.HasPrefix(c, "agents add") {
				adds = append(adds, c)
			}
		}
		if !slices.Equal(adds, tc.want) {
			t.Errorf("%s: ran %q, want %q", tc.name, adds, tc.want)
		}
	}
}
