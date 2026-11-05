package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWaitAnswerWithNoSession(t *testing.T) {
	newWorld(t).run(t, "wait", "answer").wantErr(t, 5, "no session", "next: bootstrap serve")
}

func TestShowRefusesASkippedTool(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "list")
	if code, body := s.click(t, seq, "lets-go", `,"tools":["peep"]`); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	w.run(t, "show", "tool", "sleepy").wantErr(t, 2, "skipped sleepy", "valid: agents, jobs, contextfill, peep", "next: bootstrap status")
}

func TestWaitAnswerReturnsTheClick(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "welcome")
	done := make(chan result, 1)
	go func() { done <- w.run(t, "wait", "answer", "--timeout", "10s") }()
	time.Sleep(200 * time.Millisecond)
	if code, body := s.click(t, seq, "see-plan", ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	select {
	case r := <-done:
		r.want(t, 0, fmt.Sprintf("answer seq=%d screen=welcome choice=see-plan\nnext: bootstrap show <screen>\n", seq), "")
	case <-time.After(15 * time.Second):
		t.Fatal("wait never returned")
	}
}

func TestWaitAnswerReturnsAClickThatCameFirst(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "list")
	if code, body := s.click(t, seq, "lets-go", `,"tools":["sleepy"]`); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	w.run(t, "wait", "answer").want(t, 0, fmt.Sprintf("answer seq=%d screen=list choice=lets-go tools=clt,homebrew,git,agents,jobs,contextfill,sleepy\nnext: bootstrap show <screen>\n", seq), "")
}

func TestWaitAnswerReturnsATypedName(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "naming")
	if code, body := s.click(t, seq, "next", `,"name":"pine-bakery"`); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	r := w.run(t, "wait", "answer", "--json")
	var got struct {
		Screen, Choice, Name string
		Seq                  int
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || r.code != 0 {
		t.Fatalf("exit %d, %v\n%s", r.code, err, r.stdout)
	}
	if got.Name != "pine-bakery" || got.Seq != seq || got.Screen != "naming" {
		t.Errorf("got %+v", got)
	}
}

func TestWaitAnswerNeverReturnsAnEarlierAnswer(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "welcome")
	if code, body := s.click(t, seq, "see-plan", ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	w.seq(t, "list")
	r := w.run(t, "wait", "answer", "--timeout", "300ms")
	r.wantErr(t, 3, "still waiting, run wait again", "screen=list", "next: bootstrap wait answer")
	if strings.Contains(r.stderr, "see-plan") {
		t.Errorf("an old answer leaked:\n%s", r.stderr)
	}
	// A click on the old screen arriving late is refused as well.
	if code, _ := s.click(t, seq, "skip-tour", ""); code != http.StatusConflict {
		t.Errorf("late click status %d", code)
	}
}

func TestWaitTimeoutJSON(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	w.seq(t, "welcome")
	r := w.run(t, "wait", "answer", "--timeout", "100ms", "--json")
	var got struct {
		Error, Next string
		Exit        int
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || r.code != 3 {
		t.Fatalf("exit %d, %v\n%s", r.code, err, r.stdout)
	}
	if got.Error != "still waiting, run wait again" || got.Next != "bootstrap wait answer" || got.Exit != 3 {
		t.Errorf("got %+v", got)
	}
}

func TestWaitAnswerOnAScreenWithNothingToAnswer(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	w.run(t, "wait", "answer").wantErr(t, 2, "no screen", "next: bootstrap show welcome")
	w.seq(t, "terminal")
	w.run(t, "wait", "answer").wantErr(t, 2, "the terminal screen has nothing to answer", "next: bootstrap wait brew")
}

func TestWaitAnswerWhenThePageIsGone(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	w.seq(t, "welcome")
	if err := syscall.Kill(s.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	for alive(s.PID) {
		time.Sleep(10 * time.Millisecond)
	}
	w.run(t, "wait", "answer", "--timeout", "2s").wantErr(t, 5, "page is closed", "next: bootstrap serve")
}

func TestWaitArrivals(t *testing.T) {
	cases := []struct {
		target, path, next string
	}{
		{"clt", "/Library/Developer/CommandLineTools/usr/bin/git", "bootstrap show <screen>"},
		{"brew", "/opt/homebrew/bin/brew", "/opt/homebrew/bin/brew --version"},
	}
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			w := newWorld(t)
			w.run(t, "wait", c.target, "--timeout", "150ms").wantErr(t, 3, "still waiting, run wait again", "next: bootstrap wait "+c.target)
			done := make(chan result, 1)
			go func() { done <- w.run(t, "wait", c.target, "--timeout", "10s") }()
			time.Sleep(200 * time.Millisecond)
			touchExec(t, filepath.Join(w.root, c.path))
			select {
			case r := <-done:
				r.want(t, 0, fmt.Sprintf("arrived target=%s path=%s\nnext: %s\n", c.target, c.path, c.next), "")
			case <-time.After(15 * time.Second):
				t.Fatal("wait never returned")
			}
		})
	}
}

func TestWaitCLTIsNotFooledByTheStub(t *testing.T) {
	w := newWorld(t)
	touchExec(t, filepath.Join(w.root, "usr/bin/git"))
	w.run(t, "wait", "clt", "--timeout", "100ms").wantErr(t, 3, "still waiting")
}
