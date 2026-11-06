package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Ben, 2026-11-06: once the setup has put bootstrap into Homebrew it deletes
// ~/.bootstrap/bin/bootstrap, so next: lines from the downloaded copy name
// Homebrew's instead.
func TestProgramNamesHomebrewsCopyOnceItIsThere(t *testing.T) {
	home, root := t.TempDir(), t.TempDir()
	downloaded := filepath.Join(home, ".bootstrap", "bin", "bootstrap")
	env := func(exe string) Env {
		vars := map[string]string{"HOME": home, envRoot: root}
		return Env{Getenv: func(k string) string { return vars[k] }, Exe: func() (string, error) { return exe, nil }}
	}
	if got := program(env(downloaded)); got != downloaded {
		t.Errorf("with no Homebrew copy, program = %q, want %q", got, downloaded)
	}
	brewed := filepath.Join(root, "opt", "homebrew", "bin", "bootstrap")
	if err := os.MkdirAll(filepath.Dir(brewed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brewed, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := program(env(downloaded)); got != "/opt/homebrew/bin/bootstrap" {
		t.Errorf("with Homebrew's copy, program = %q, want /opt/homebrew/bin/bootstrap", got)
	}
	// A build run from anywhere else, such as a checkout, is named as it is.
	if got := program(env("/src/bootstrap/local/bs")); got != "/src/bootstrap/local/bs" {
		t.Errorf("a dev build is named %q", got)
	}
}
