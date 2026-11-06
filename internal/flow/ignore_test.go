package flow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// Ben, 2026-11-06: the .gitignore also keeps out what each stack the project's
// files show leaves behind, alongside what the making answer calls for.
func TestTheGitignoreFitsTheStacksAProjectShows(t *testing.T) {
	const python = "__pycache__/\n.venv/\n.pytest_cache/\n.mypy_cache/\n.ruff_cache/\n"
	const swift = ".build/\n.swiftpm/\nDerivedData/\nxcuserdata/\n"
	for _, tc := range []struct {
		name, making string
		files        []string
		want         string
	}{
		{"a Python project", "other", []string{"pyproject.toml"}, ".DS_Store\n" + python + "local/\n.jobs.db\n"},
		{"a Swift package called something else", "other", []string{"Package.swift"}, ".DS_Store\n" + swift + "local/\n.jobs.db\n"},
		{"a Swift app with Python keeps Swift's lines once", "app", []string{"Package.swift", "requirements.txt"}, ".DS_Store\n" + swift + python + "local/\n.jobs.db\n"},
		{"a Python website", "website", []string{"setup.py"}, ".DS_Store\nnode_modules/\n.env\ndist/\n" + python + "local/\n.jobs.db\n"},
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
		ignore, _ := os.ReadFile(filepath.Join(start, ".gitignore"))
		if got := string(ignore); got != tc.want {
			t.Errorf("%s: .gitignore = %q, want %q", tc.name, got, tc.want)
		}
	}
}
