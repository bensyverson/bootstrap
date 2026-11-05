package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServeNeedsToKnowTheAgent(t *testing.T) {
	newWorld(t).run(t, "serve").wantErr(t, 2, "which app", "valid: claude-code, opencode", "next: bootstrap serve --agent")
}

func TestServeTerseAck(t *testing.T) {
	w := newWorld(t)
	r := w.run(t, "serve", "--agent", "claude-code")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	match := regexp.MustCompile(`^serving url=http://127\.0\.0\.1:\d+/\?token=[0-9a-f]{32} pid=(\d+) browser=opened reused=no\nnext: bootstrap show welcome\n$`).FindStringSubmatch(r.stdout)
	if match == nil {
		t.Fatalf("output:\n%s", r.stdout)
	}
	pid, _ := strconv.Atoi(match[1])
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	if !alive(pid) {
		t.Error("the server isn't running")
	}
}

func TestServeTakesTheAgentFromTheEnvironmentAndTheFlagWins(t *testing.T) {
	w := newWorld(t).with("CLAUDECODE=1")
	r := w.run(t, "serve", "--json")
	var s served
	if err := json.Unmarshal([]byte(r.stdout), &s); err != nil || r.code != 0 {
		t.Fatalf("exit %d, %v\n%s", r.code, err, r.stdout)
	}
	t.Cleanup(func() { syscall.Kill(s.PID, syscall.SIGKILL) })
	if status := w.run(t, "status"); !strings.Contains(status.stdout, "agent=claude-code") {
		t.Errorf("status:\n%s", status.stdout)
	}
	w.run(t, "serve", "--agent", "opencode")
	if status := w.run(t, "status"); !strings.Contains(status.stdout, "agent=opencode") {
		t.Errorf("the flag wins:\n%s", status.stdout)
	}
}

func TestSecondServeReusesTheServer(t *testing.T) {
	w := newWorld(t)
	first := w.serve(t)
	second := w.serve(t)
	if second.PID != first.PID || second.URL != first.URL || !second.Reused {
		t.Errorf("first %+v, second %+v", first, second)
	}
	if second.Browser != "opened" {
		t.Errorf("no tab is open yet, so it should open one: %+v", second)
	}
	defer first.openTab(t)()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(w.run(t, "status").stdout, "tabs=1") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if third := w.serve(t); third.Browser != "already-open" {
		t.Errorf("a tab is open, so don't open another: %+v", third)
	}
}

func TestServeExits5WhenItCantBind(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	port := taken.Addr().(*net.TCPAddr).Port
	w := newWorld(t).with(fmt.Sprintf("BOOTSTRAP_PORT=%d", port))

	start := time.Now()
	r := w.run(t, "serve", "--agent", "claude-code")
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v", elapsed)
	}
	r.wantErr(t, 5, "can't start the page", "next: tell the person in one sentence", "chat")

	r = w.run(t, "serve", "--agent", "claude-code", "--json")
	var got struct {
		Error string
		Exit  int
		Next  string
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || r.code != 5 || got.Exit != 5 || !strings.Contains(got.Next, "chat") {
		t.Errorf("exit %d, %v, %+v\n%s", r.code, err, got, r.stdout)
	}
	// A failed start leaves no session for a later command to mistake for a running one.
	w.run(t, "status").wantErr(t, 5, "no session")
}

func TestServeExits5WhenTheFolderCantBeWritten(t *testing.T) {
	w := newWorld(t)
	blocked := filepath.Join(w.home, "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w.with("BOOTSTRAP_DIR="+filepath.Join(blocked, "sub")).run(t, "serve", "--agent", "claude-code").wantErr(t, 5, "can't start the page", "chat")
}

func TestServeExits5WhenTheBrowserCantOpen(t *testing.T) {
	w := newWorld(t).with("BOOTSTRAP_OPEN=/usr/bin/false")
	r := w.run(t, "serve", "--agent", "claude-code", "--json")
	var got struct {
		URL     string
		PID     int
		Browser string
		Exit    int
		Error   string
		Next    string
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	t.Cleanup(func() { syscall.Kill(got.PID, syscall.SIGKILL) })
	if r.code != 5 || got.Exit != 5 || got.Browser != "failed" || got.URL == "" || !strings.Contains(got.Next, "chat") {
		t.Errorf("exit %d, %+v", r.code, got)
	}
	// The link still works, so a person can open it by hand.
	if !alive(got.PID) {
		t.Error("the server should wait for someone to open the link")
	}
}

func TestLogIsOwnerOnlyInTheLiveFolder(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	for path, want := range map[string]os.FileMode{w.dir: 0o700, filepath.Join(w.dir, "session.jsonl"): 0o600, filepath.Join(w.dir, "server.json"): 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %o, want %o", path, got, want)
		}
	}
}

func TestServeDefaultsToAPrivateFolderInHome(t *testing.T) {
	w := newWorld(t)
	var env []string
	for _, e := range w.env {
		if !strings.HasPrefix(e, "BOOTSTRAP_DIR=") {
			env = append(env, e)
		}
	}
	w.env = env
	w.serve(t)
	if _, err := os.Stat(filepath.Join(w.home, ".bootstrap", "session.jsonl")); err != nil {
		t.Error(err)
	}
}

func TestRefusalsOnTheRealServer(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	resp, err := http.Get(s.origin(t) + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no token: %d", resp.StatusCode)
	}
	resp2, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != 200 || !strings.Contains(string(body), "/events") {
		t.Errorf("with the token: %d\n%s", resp2.StatusCode, body)
	}
}
