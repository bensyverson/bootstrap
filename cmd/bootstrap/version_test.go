package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// The release stamps its version at link time; scripts/release reads it back with --version.
const versionVar = "github.com/bensyverson/bootstrap/internal/cli.Version"

// An unstamped build reports the version the source states, which
// scripts/release writes into internal/cli/version.go so the tap's release can
// read it (Ben, 2026-11-06, wz9ORc).
func TestVersionOfAnUnstampedBuild(t *testing.T) {
	src, err := os.ReadFile("../../internal/cli/version.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^var Version = "(\d+\.\d+\.\d+)"$`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("internal/cli/version.go has no line var Version = \"X.Y.Z\":\n%s", src)
	}
	w := newWorld(t)
	w.run(t, "--version").want(t, 0, "bootstrap "+string(m[1])+"\n", "")
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
