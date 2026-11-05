package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type served struct {
	URL     string `json:"url"`
	PID     int    `json:"pid"`
	Browser string `json:"browser"`
	Reused  bool   `json:"reused"`
	Next    string `json:"next"`
}

func (s served) origin(t *testing.T) string {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Scheme + "://" + u.Host
}

func (s served) token(t *testing.T) string {
	t.Helper()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("token")
}

func (w *world) serve(t *testing.T) served {
	t.Helper()
	r := w.run(t, "serve", "--agent", "claude-code", "--json")
	if r.code != 0 {
		t.Fatalf("serve exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	var s served
	if err := json.Unmarshal([]byte(r.stdout), &s); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	t.Cleanup(func() { syscall.Kill(s.PID, syscall.SIGKILL) })
	return s
}

func (w *world) seq(t *testing.T, args ...string) int {
	t.Helper()
	r := w.run(t, append(append([]string{"show"}, args...), "--json")...)
	if r.code != 0 {
		t.Fatalf("show exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	var ack struct{ Seq int }
	if err := json.Unmarshal([]byte(r.stdout), &ack); err != nil {
		t.Fatal(err)
	}
	return ack.Seq
}

func (s served) click(t *testing.T, seq int, choice, extra string) (int, string) {
	t.Helper()
	body := fmt.Sprintf(`{"seq":%d,"choice":%q%s}`, seq, choice, extra)
	req, err := http.NewRequest("POST", s.origin(t)+"/answer?token="+s.token(t), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", s.origin(t))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// openTab holds an event stream open the way a browser tab does, returning its close.
func (s served) openTab(t *testing.T) func() {
	t.Helper()
	resp, err := http.Get(s.URL[:strings.Index(s.URL, "/?")] + "/events?token=" + s.token(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return func() { resp.Body.Close() }
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func TestStatusWithNoSession(t *testing.T) {
	newWorld(t).run(t, "status").wantErr(t, 5, "no session", "next: bootstrap run --agent <app>")
}

func TestShowWithNoSession(t *testing.T) {
	newWorld(t).run(t, "show", "welcome").wantErr(t, 5, "no session", "next: bootstrap serve")
}

func TestStopWithNoSession(t *testing.T) {
	newWorld(t).run(t, "stop").wantErr(t, 5, "no session", "next: bootstrap status")
}

func TestShowAckTeachesTheNextCommand(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"show", "welcome"}, "shown seq=2 screen=welcome mode=waiting progress=0\nnext: bootstrap wait answer\n"},
		{[]string{"show", "apple-ask"}, "shown seq=3 screen=apple-ask mode=waiting progress=0\nnext: bootstrap wait clt\n"},
		{[]string{"show", "brew-ask"}, "shown seq=4 screen=brew-ask mode=waiting progress=0.111\nnext: bootstrap wait brew\n"},
		{[]string{"show", "tool", "jobs", "--status", "installing", "--expect", "20m"}, "shown seq=5 screen=tool tool=jobs status=installing mode=working progress=0.333\nnext: do the work, then bootstrap show <screen>\n"},
		{[]string{"show", "done"}, "shown seq=6 screen=done mode=settled progress=1\nnext: bootstrap stop\n"},
	}
	for _, c := range cases {
		w.run(t, c.args...).want(t, 0, c.want, "")
	}
}

func TestShowJSONCarriesTheDescription(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	r := w.run(t, "show", "welcome", "--json")
	var got struct {
		Seq         int
		Screen      string
		Next        string
		Description struct {
			Title []struct{ Text string }
		}
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if got.Screen != "welcome" || got.Next != "bootstrap wait answer" || got.Description.Title[0].Text != "Let’s give Claude Code the good stuff." {
		t.Errorf("got %+v", got)
	}
}

func TestStatusReportsAClickWithNoAgent(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "welcome")
	if code, body := s.click(t, seq, "skip-tour", ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	r := w.run(t, "status")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	rows := strings.Split(r.stdout, "\n")
	if !regexp.MustCompile(`^session=[0-9a-f]{8} agent=claude-code page=running tabs=0 lost=no$`).MatchString(rows[0]) {
		t.Errorf("row 0 = %q", rows[0])
	}
	if want := fmt.Sprintf("screen=welcome seq=%d mode=waiting progress=0", seq); rows[1] != want {
		t.Errorf("row 1 = %q, want %q", rows[1], want)
	}
	if want := fmt.Sprintf("answer seq=%d screen=welcome choice=skip-tour", seq); rows[2] != want {
		t.Errorf("row 2 = %q, want %q", rows[2], want)
	}
	if rows[3] != "next: bootstrap show <screen>" {
		t.Errorf("row 3 = %q", rows[3])
	}
}

func TestStatusJSON(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "list")
	s.click(t, seq, "lets-go", `,"tools":["peep"]`)
	r := w.run(t, "status", "--json")
	var got struct {
		Session, Agent, Page, Next string
		Lost                       bool
		Screen                     struct {
			Screen string
			Seq    int
		}
		Answers []struct {
			Screen, Choice string
			Tools          []string
		}
		Chosen []string
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil || r.code != 0 {
		t.Fatalf("exit %d, %v\n%s", r.code, err, r.stdout)
	}
	if got.Screen.Screen != "list" || got.Screen.Seq != seq || len(got.Answers) != 1 || got.Answers[0].Choice != "lets-go" || len(got.Chosen) != 7 {
		t.Errorf("got %+v", got)
	}
}

func TestStatusSpellsOutWhatToInstall(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	s.click(t, w.seq(t, "list"), "lets-go", `,"tools":["peep"]`)
	if status := w.run(t, "status"); !strings.Contains(status.stdout, "\nchosen=clt,homebrew,git,agents,jobs,contextfill,peep\n") {
		t.Errorf("status:\n%s", status.stdout)
	}
}

func TestShowWhenThePageIsGoneIsSavedAndHeals(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	w.seq(t, "welcome")
	syscall.Kill(s.PID, syscall.SIGKILL)
	for alive(s.PID) {
		time.Sleep(10 * time.Millisecond)
	}
	w.run(t, "show", "list").wantErr(t, 5, "saved", "page is closed", "next: bootstrap serve")
	status := w.run(t, "status")
	if !strings.Contains(status.stdout, "page=closed") || !strings.Contains(status.stdout, "screen=list") || !strings.Contains(status.stdout, "next: bootstrap serve") {
		t.Errorf("status:\n%s", status.stdout)
	}
	// serve starts a fresh server from the same log.
	again := w.serve(t)
	if again.PID == s.PID || again.Reused {
		t.Errorf("again = %+v", again)
	}
	if status := w.run(t, "status"); !strings.Contains(status.stdout, "screen=list") || !strings.Contains(status.stdout, "page=running") {
		t.Errorf("status:\n%s", status.stdout)
	}
}

func TestUnsafeNameIsRefused(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	seq := w.seq(t, "naming")
	code, body := s.click(t, seq, "next", `,"name":"../../etc; rm -rf x"`)
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "letters, numbers and dashes") {
		t.Errorf("%d %s", code, body)
	}
	if status := w.run(t, "status"); strings.Contains(status.stdout, "answer ") {
		t.Errorf("status:\n%s", status.stdout)
	}
}

func TestCorruptLogNamesTheRemedy(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	f, err := os.OpenFile(filepath.Join(w.dir, "session.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("garbage\n")
	f.Close()
	w.run(t, "status").wantErr(t, 4, "session.jsonl", "can't be read", "next: delete")
	w.run(t, "show", "welcome").wantErr(t, 4, "session.jsonl", "next: delete")
}

func TestStop(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	w.seq(t, "welcome")
	r := w.run(t, "stop")
	if want := regexp.MustCompile(fmt.Sprintf(`^stopped session=[0-9a-f]{8} pid=%d server=exited\n$`, s.PID)); r.code != 0 || !want.MatchString(r.stdout) {
		t.Errorf("exit %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	w.run(t, "status").wantErr(t, 5, "session ended", "next: bootstrap serve")
	w.run(t, "show", "list").wantErr(t, 5, "session ended", "next: bootstrap serve")
	w.run(t, "stop").want(t, 0, "already stopped\n", "")
	if fresh := w.serve(t); fresh.PID == s.PID {
		t.Error("the next serve starts a new server")
	}
	if status := w.run(t, "status"); strings.Contains(status.stdout, "screen=") {
		t.Errorf("a new session begins empty:\n%s", status.stdout)
	}
}

// A live run of the whole loop, ending with pgrep on the printed pid.
func TestLiveRunLeavesNoProcess(t *testing.T) {
	w := newWorld(t)
	s := w.serve(t)
	if !alive(s.PID) {
		t.Fatal("not running")
	}
	closeTab := s.openTab(t)
	seq := w.seq(t, "welcome")
	if code, body := s.click(t, seq, "see-plan", ""); code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	if r := w.run(t, "wait", "answer"); r.code != 0 {
		t.Fatalf("wait exit %d: %s", r.code, r.stderr)
	}
	if r := w.run(t, "stop"); r.code != 0 {
		t.Fatalf("stop exit %d: %s", r.code, r.stderr)
	}
	closeTab()
	pidFile := filepath.Join(t.TempDir(), "pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(s.PID)), 0o644); err != nil {
		t.Fatal(err)
	}
	pgrep := exec.Command("/usr/bin/pgrep", "-F", pidFile)
	out, err := pgrep.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Errorf("pgrep found the server: %v\n%s", err, out)
	}
}

func TestEveryCommandRenewsTheAllowance(t *testing.T) {
	w := newWorld(t)
	w.serve(t)
	w.seq(t, "welcome")
	count := func() int {
		body, err := os.ReadFile(filepath.Join(w.dir, "session.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(body), `"kind":"beat"`)
	}
	for _, args := range [][]string{{"status"}, {"detect"}, {"tools"}, {"screens"}} {
		before := count()
		if r := w.run(t, args...); r.code != 0 {
			t.Fatalf("%v: exit %d", args, r.code)
		}
		if count() != before+1 {
			t.Errorf("%v didn't renew", args)
		}
	}
	before := count()
	w.run(t, "--help")
	w.run(t)
	if count() != before {
		t.Error("the primer leaves no trace")
	}
}
