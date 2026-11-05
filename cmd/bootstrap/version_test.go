package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The release stamps its version at link time; scripts/release reads it back with --version.
const versionVar = "github.com/bensyverson/bootstrap/internal/cli.Version"

func TestVersionOfAnUnstampedBuild(t *testing.T) {
	w := newWorld(t)
	w.run(t, "--version").want(t, 0, "bootstrap dev\n", "")
}

func TestVersionOfAStampedBuild(t *testing.T) {
	stamped := filepath.Join(t.TempDir(), "bootstrap")
	build := exec.Command("go", "build", "-ldflags", "-X "+versionVar+"=9.8.7", "-o", stamped, ".")
	build.Env = append(build.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	w := newWorld(t)
	cmd := exec.Command(stamped, "--version")
	cmd.Env = w.env
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "bootstrap 9.8.7\n" {
		t.Errorf("got %q, want %q", out, "bootstrap 9.8.7\n")
	}
}
