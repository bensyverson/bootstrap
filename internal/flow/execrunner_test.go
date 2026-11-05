package flow_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/flow"
)

func script(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestExecRunnerCapturesOutputAndExit(t *testing.T) {
	root := t.TempDir()
	script(t, filepath.Join(root, "usr/bin/tool"), `echo out "$1"; echo err >&2; exit 3`)
	res := flow.ExecRunner{Root: root}.Run(context.Background(), flow.Command{Path: "/usr/bin/tool", Args: []string{"one"}, Timeout: 5 * time.Second})
	if res.Err != nil || res.TimedOut || res.Exit != 3 {
		t.Fatalf("result = %+v", res)
	}
	if got := string(res.Output); got != "out one\nerr\n" {
		t.Errorf("output = %q", got)
	}
}

func TestExecRunnerRunsInDir(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	script(t, filepath.Join(root, "bin/where"), `pwd -P`)
	res := flow.ExecRunner{Root: root}.Run(context.Background(), flow.Command{Path: "/bin/where", Dir: dir, Timeout: 5 * time.Second})
	want, _ := filepath.EvalSymlinks(dir)
	if strings.TrimSpace(string(res.Output)) != want || res.Exit != 0 {
		t.Errorf("result = %+v, want dir %s", res, want)
	}
}

func TestExecRunnerWithNoRootRunsThePathAsGiven(t *testing.T) {
	res := flow.ExecRunner{}.Run(context.Background(), flow.Command{Path: "/bin/echo", Args: []string{"hi"}, Timeout: 5 * time.Second})
	if res.Err != nil || res.Exit != 0 || string(res.Output) != "hi\n" {
		t.Errorf("result = %+v", res)
	}
}

func TestExecRunnerTimesOut(t *testing.T) {
	root := t.TempDir()
	script(t, filepath.Join(root, "bin/slow"), `sleep 10`)
	began := time.Now()
	res := flow.ExecRunner{Root: root}.Run(context.Background(), flow.Command{Path: "/bin/slow", Timeout: 200 * time.Millisecond})
	if !res.TimedOut || res.Err != nil {
		t.Errorf("result = %+v", res)
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Errorf("took %v", took)
	}
}

func TestExecRunnerReportsACommandThatWontStart(t *testing.T) {
	res := flow.ExecRunner{Root: t.TempDir()}.Run(context.Background(), flow.Command{Path: "/usr/bin/xcode-select", Timeout: time.Second})
	if res.Err == nil {
		t.Errorf("result = %+v", res)
	}
}

func TestExecRunnerStopsWhenTheContextEnds(t *testing.T) {
	root := t.TempDir()
	script(t, filepath.Join(root, "bin/slow"), `sleep 10`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	began := time.Now()
	res := flow.ExecRunner{Root: root}.Run(ctx, flow.Command{Path: "/bin/slow", Timeout: time.Minute})
	if res.TimedOut {
		t.Errorf("the flow ending isn't the command's own limit: %+v", res)
	}
	if took := time.Since(began); took < 150*time.Millisecond || took > 5*time.Second {
		t.Errorf("took %v: the command should run until the flow ends, and no longer", took)
	}
	if res.Exit == 0 && res.Err == nil {
		t.Errorf("a killed command isn't a success: %+v", res)
	}
}
