package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectMaking(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		dirs  []string
		want  Making
	}{
		{"an empty folder", nil, nil, MakingUnknown},
		{"a package.json", []string{"package.json"}, nil, MakingWebsite},
		{"a Package.swift", []string{"Package.swift"}, nil, MakingApp},
		{"an Xcode project", nil, []string{"Bakery.xcodeproj"}, MakingApp},
		{"package.json wins over Package.swift", []string{"package.json", "Package.swift"}, nil, MakingWebsite},
		{"other files say nothing", []string{"README.md", "main.go"}, nil, MakingUnknown},
		{"a package.json that's a folder says nothing", nil, []string{"package.json"}, MakingUnknown},
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
			if got := ProjectMaking(dir); got != c.want {
				t.Errorf("ProjectMaking = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProjectMakingOfAMissingFolder(t *testing.T) {
	if got := ProjectMaking(filepath.Join(t.TempDir(), "nope")); got != MakingUnknown {
		t.Errorf("ProjectMaking = %q", got)
	}
}
