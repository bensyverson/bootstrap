package detect

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProjectStacks(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		dirs  []string
		want  []Stack
	}{
		{"an empty folder", nil, nil, nil},
		{"a go.mod", []string{"go.mod"}, nil, []Stack{StackGo}},
		{"a pyproject.toml", []string{"pyproject.toml"}, nil, []Stack{StackPython}},
		{"a requirements.txt", []string{"requirements.txt"}, nil, []Stack{StackPython}},
		{"a setup.py", []string{"setup.py"}, nil, []Stack{StackPython}},
		{"a package.json", []string{"package.json"}, nil, []Stack{StackNode}},
		{"a Package.swift", []string{"Package.swift"}, nil, []Stack{StackSwift}},
		{"an Xcode project", nil, []string{"Bakery.xcodeproj"}, []Stack{StackSwift}},
		{"several, in a fixed order", []string{"Package.swift", "package.json", "requirements.txt", "go.mod"}, nil, []Stack{StackGo, StackPython, StackNode, StackSwift}},
		{"two Python markers count once", []string{"pyproject.toml", "requirements.txt"}, nil, []Stack{StackPython}},
		{"a go.mod that's a folder says nothing", nil, []string{"go.mod"}, nil},
		{"other files say nothing", []string{"README.md", "main.go"}, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				touch(t, filepath.Join(dir, f), 0o644)
			}
			for _, d := range c.dirs {
				if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if got := ProjectStacks(dir); !slices.Equal(got, c.want) {
				t.Errorf("ProjectStacks = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProjectStacksOfAMissingFolder(t *testing.T) {
	if got := ProjectStacks(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Errorf("ProjectStacks = %q", got)
	}
}
