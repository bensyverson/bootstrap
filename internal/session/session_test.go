package session

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

var t0 = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func begun(t *testing.T) Store {
	t.Helper()
	store := Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 123, t0); err != nil {
		t.Fatal(err)
	}
	return store
}

func show(t *testing.T, store Store, req screen.Request, at time.Time) State {
	t.Helper()
	state, err := store.Show(req, 0, at)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func load(t *testing.T, store Store) State {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestLoadWithNoSession(t *testing.T) {
	_, err := Store{Dir: t.TempDir()}.Load()
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("err = %v, want ErrNoSession", err)
	}
}

func TestLogAndFolderAreOwnerOnly(t *testing.T) {
	store := begun(t)
	info, err := os.Stat(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode = %o", info.Mode().Perm())
	}
	info, err = os.Stat(store.LogPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("log mode = %o", info.Mode().Perm())
	}
	if err := store.WriteServerInfo(ServerInfo{PID: 1, URL: "http://127.0.0.1:1/?token=x", Port: 1}); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(filepath.Join(store.Dir, "server.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("server.json holds the token, mode = %o", info.Mode().Perm())
	}
}

func TestLooseLogPermissionsGetTightened(t *testing.T) {
	store := begun(t)
	if err := os.Chmod(store.LogPath(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(store.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(screen.ClaudeCode, 124, t0); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(store.LogPath()); info.Mode().Perm() != 0o600 {
		t.Errorf("log mode = %o", info.Mode().Perm())
	}
	if info, _ := os.Stat(store.Dir); info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode = %o", info.Mode().Perm())
	}
}

func TestSymlinkedLogIsRefused(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, store.LogPath()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(screen.ClaudeCode, 1, t0); err == nil {
		t.Error("a symlink at the log's path must not be followed")
	}
}

func TestCorruptLogNamesItself(t *testing.T) {
	store := begun(t)
	f, err := os.OpenFile(store.LogPath(), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("not json\n")
	f.Close()
	_, err = store.Load()
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) || corrupt.Path != store.LogPath() || corrupt.Line != 2 {
		t.Errorf("err = %#v", err)
	}
}

func TestLogIsAppendOnlyJSONL(t *testing.T) {
	store := begun(t)
	before, _ := os.ReadFile(store.LogPath())
	show(t, store, screen.Request{Screen: "welcome"}, t0)
	after, _ := os.ReadFile(store.LogPath())
	if !strings.HasPrefix(string(after), string(before)) {
		t.Error("the log changed instead of growing")
	}
	if got := strings.Count(string(after), "\n"); got != 2 {
		t.Errorf("%d lines:\n%s", got, after)
	}
}

func TestShowNeedsASession(t *testing.T) {
	_, err := Store{Dir: t.TempDir()}.Show(screen.Request{Screen: "welcome"}, 0, t0)
	if !errors.Is(err, ErrNoSession) {
		t.Errorf("err = %v", err)
	}
	store := begun(t)
	if _, err := store.Stop(t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Show(screen.Request{Screen: "welcome"}, 0, t0); !errors.Is(err, ErrEnded) {
		t.Errorf("err = %v, want ErrEnded", err)
	}
}

func TestShowRefusesASkippedTool(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "list"}, t0)
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "lets-go"}, t0); err != nil {
		t.Fatal(err)
	}
	_, err := store.Show(screen.Request{Screen: "tool", Tool: "sleepy"}, 0, t0)
	var inv *screen.InvalidError
	if !errors.As(err, &inv) {
		t.Errorf("err = %v", err)
	}
}

func TestSkippingATroubledToolForgetsIt(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "trouble", Tool: "sleepy", Reason: "network"}, t0)
	state, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "skip"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(state.Chosen, "sleepy") || !slices.Contains(state.Chosen, "peep") {
		t.Errorf("chosen = %v", state.Chosen)
	}
}

func TestEarlierAnswerIsNeverCurrent(t *testing.T) {
	store := begun(t)
	first := show(t, store, screen.Request{Screen: "welcome"}, t0)
	if _, err := store.RecordAnswer(first.Current.Seq, screen.Answer{Choice: "see-plan"}, t0); err != nil {
		t.Fatal(err)
	}
	second := show(t, store, screen.Request{Screen: "list"}, t0)
	if _, ok := second.CurrentAnswer(); ok {
		t.Error("the list screen has no answer yet")
	}
	if got := second.Answers; len(got) != 1 || got[0].Answer.Choice != "see-plan" {
		t.Errorf("status still reports every answer so far: %+v", got)
	}
}

func TestAnswerToAnEarlierScreenIsRefused(t *testing.T) {
	store := begun(t)
	first := show(t, store, screen.Request{Screen: "welcome"}, t0)
	show(t, store, screen.Request{Screen: "list"}, t0)
	_, err := store.RecordAnswer(first.Current.Seq, screen.Answer{Choice: "see-plan"}, t0)
	if !errors.Is(err, ErrStale) {
		t.Errorf("err = %v, want ErrStale", err)
	}
}

func TestSecondClickDoesNotReplaceTheFirst(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "skip-tour"}, t0); !errors.Is(err, ErrAnswered) {
		t.Errorf("err = %v, want ErrAnswered", err)
	}
	got, _ := load(t, store).CurrentAnswer()
	if got.Answer.Choice != "see-plan" {
		t.Errorf("answer = %+v", got)
	}
}

func TestAnswerOutsideTheScreenIsRefused(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "naming"}, t0)
	_, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "next", Name: "../../etc"}, t0)
	var inv *screen.InvalidError
	if !errors.As(err, &inv) {
		t.Errorf("err = %v", err)
	}
	if got := load(t, store).Answers; len(got) != 0 {
		t.Errorf("an unsafe name reached the log: %+v", got)
	}
}

func TestListAnswerKeepsRequiredTools(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "list"}, t0)
	state, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "lets-go", Tools: []string{"peep"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill", "peep"}
	if !slices.Equal(state.Chosen, want) {
		t.Errorf("chosen = %v", state.Chosen)
	}
}

// "Your turn" screens have nothing on the page to click, so waiting for a click would mean waiting forever.
func TestRestartKeepsTheSameLog(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, t0); err != nil {
		t.Fatal(err)
	}
	again, err := store.Begin(screen.ClaudeCode, 456, t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != state.ID || len(again.Answers) != 1 || again.Current == nil || again.Current.Request.Screen != "welcome" {
		t.Errorf("state = %+v", again)
	}
}

func TestBeginAfterStopStartsFresh(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, t0)
	if _, err := store.Stop(t0); err != nil {
		t.Fatal(err)
	}
	fresh, err := store.Begin(screen.OpenCode, 2, t0)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == state.ID || fresh.Current != nil || fresh.Ended || fresh.Agent != screen.OpenCode {
		t.Errorf("fresh = %+v", fresh)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "session-"+state.ID+".jsonl")); err != nil {
		t.Errorf("the old log is kept: %v", err)
	}
}

func TestRenewWithNoSessionLeavesNoTrace(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "never")}
	if err := store.Renew(t0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a read left a folder behind: %v", err)
	}
	live := begun(t)
	before := load(t, live).Seq
	if err := live.Renew(t0); err != nil {
		t.Fatal(err)
	}
	if after := load(t, live).Seq; after != before+1 {
		t.Errorf("with a session, renewing is an event: %d then %d", before, after)
	}
}

func TestTabsAreRecorded(t *testing.T) {
	store := begun(t)
	if err := store.SetTabs(1, t0); err != nil {
		t.Fatal(err)
	}
	if got := load(t, store).Tabs; got != 1 {
		t.Errorf("tabs = %d", got)
	}
	// A new server starts with no tabs, whatever the last one left.
	state, err := store.Begin(screen.ClaudeCode, 9, t0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Tabs != 0 {
		t.Errorf("tabs = %d", state.Tabs)
	}
}

func TestServerLock(t *testing.T) {
	store := begun(t)
	if store.ServerRunning() {
		t.Error("nothing holds the lock")
	}
	release, err := store.LockServer()
	if err != nil {
		t.Fatal(err)
	}
	if !store.ServerRunning() {
		t.Error("the lock is held")
	}
	if _, err := store.LockServer(); !errors.Is(err, ErrServerRunning) {
		t.Errorf("err = %v", err)
	}
	release()
	if store.ServerRunning() {
		t.Error("the lock was released")
	}
}

// The chat fallback runs wait with no page, possibly where this folder can't be written.
