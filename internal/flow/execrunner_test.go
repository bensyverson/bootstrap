package flow_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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

// The script won't go on until the first line has reached the caller, so this
// passes only if lines arrive while the command still runs.
func TestExecRunnerStreamsLinesAsTheyCome(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	gate := filepath.Join(dir, "gate")
	script(t, filepath.Join(root, "bin/talk"), `echo "! One-time code (D7BA-CFE2) copied to clipboard"
i=0; while [ ! -e "$1" ] && [ $i -lt 300 ]; do sleep 0.01; i=$((i+1)); done
[ -e "$1" ] || { echo "never heard back" >&2; exit 9; }
echo done >&2`)
	var lines []string
	res := flow.ExecRunner{Root: root}.Stream(context.Background(), flow.Command{Path: "/bin/talk", Args: []string{gate}, Timeout: 10 * time.Second}, func(line string) {
		lines = append(lines, line)
		if len(lines) == 1 {
			os.WriteFile(gate, nil, 0o600)
		}
	})
	if res.Err != nil || res.Exit != 0 {
		t.Fatalf("result = %+v, output %q", res, res.Output)
	}
	want := []string{"! One-time code (D7BA-CFE2) copied to clipboard", "done"}
	if !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}
	if got := string(res.Output); got != strings.Join(want, "\n")+"\n" {
		t.Errorf("output = %q", got)
	}
}

func TestExecRunnerStreamTimesOut(t *testing.T) {
	root := t.TempDir()
	script(t, filepath.Join(root, "bin/slow"), `echo waiting; sleep 10`)
	var lines []string
	res := flow.ExecRunner{Root: root}.Stream(context.Background(), flow.Command{Path: "/bin/slow", Timeout: 300 * time.Millisecond}, func(line string) { lines = append(lines, line) })
	if !res.TimedOut || !slices.Equal(lines, []string{"waiting"}) {
		t.Errorf("result = %+v, lines %q", res, lines)
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
