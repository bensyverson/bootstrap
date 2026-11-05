package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
)

func holdServerLock(t *testing.T, store Store) {
	t.Helper()
	release, err := store.LockServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

func TestWaitAnswerReturnsAnAnswerThatArrivedFirst(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	state := show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "skip-tour"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := store.WaitAnswer(context.Background(), time.Second)
	if err != nil || got.Answer.Choice != "skip-tour" {
		t.Errorf("got %+v, %v", got, err)
	}
	// It stays kept: asking twice is safe.
	got, err = store.WaitAnswer(context.Background(), time.Second)
	if err != nil || got.Answer.Choice != "skip-tour" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestWaitAnswerSeesAClickThatArrivesLater(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	state := show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	go func() {
		time.Sleep(150 * time.Millisecond)
		store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, time.Now())
	}()
	got, err := store.WaitAnswer(context.Background(), 5*time.Second)
	if err != nil || got.Answer.Choice != "see-plan" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestWaitAnswerNeverReturnsAnEarlierAnswer(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	first := show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	if _, err := store.RecordAnswer(first.Current.Seq, screen.Answer{Choice: "see-plan"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	show(t, store, screen.Request{Screen: "list"}, time.Now())
	_, err := store.WaitAnswer(context.Background(), 200*time.Millisecond)
	if !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestWaitAnswerErrors(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	if _, err := store.WaitAnswer(context.Background(), time.Second); !errors.Is(err, ErrNoScreen) {
		t.Errorf("no screen yet: %v", err)
	}
	show(t, store, screen.Request{Screen: "tool", Tool: "jobs"}, time.Now())
	if _, err := store.WaitAnswer(context.Background(), time.Second); !errors.Is(err, ErrNoChoices) {
		t.Errorf("nothing to answer: %v", err)
	}
	show(t, store, screen.Request{Screen: "chat"}, time.Now())
	if _, err := store.WaitAnswer(context.Background(), time.Second); !errors.Is(err, ErrNoChoices) {
		t.Errorf("a reply in the chat never reaches the page: %v", err)
	}
}

func TestWaitAnswerStopsWhenThePageIsGone(t *testing.T) {
	store := begun(t)
	show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	_, err := store.WaitAnswer(context.Background(), 5*time.Second)
	if !errors.Is(err, ErrPageGone) {
		t.Errorf("err = %v, want ErrPageGone", err)
	}
}

func TestWaitAnswerStillReturnsAnAnswerWhenThePageIsGone(t *testing.T) {
	store := begun(t)
	state := show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	if _, err := store.RecordAnswer(state.Current.Seq, screen.Answer{Choice: "see-plan"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got, err := store.WaitAnswer(context.Background(), time.Second); err != nil || got.Answer.Choice != "see-plan" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestWaitAnswerNoticesAStop(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	show(t, store, screen.Request{Screen: "welcome"}, time.Now())
	go func() {
		time.Sleep(100 * time.Millisecond)
		store.Stop(time.Now())
	}()
	if _, err := store.WaitAnswer(context.Background(), 5*time.Second); !errors.Is(err, ErrEnded) {
		t.Errorf("err = %v, want ErrEnded", err)
	}
}

func TestWaitAnswerRenews(t *testing.T) {
	store := begun(t)
	holdServerLock(t, store)
	start := time.Now()
	show(t, store, screen.Request{Screen: "welcome"}, start)
	before := load(t, store).Seq
	if _, err := store.WaitAnswer(context.Background(), 150*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("err = %v", err)
	}
	if after := load(t, store).Seq; after <= before {
		t.Error("running wait is a command, so it must renew")
	}
}

func TestWaitInstallNeedsNoWritableFolder(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	brew := filepath.Join(root, "opt/homebrew/bin/brew")
	if err := os.MkdirAll(filepath.Dir(brew), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brew, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	store := Store{Dir: filepath.Join(blocked, "sub")}
	got, err := store.WaitInstall(context.Background(), detect.Probe{Root: root, Home: home}, "homebrew", time.Second)
	if err != nil || got.Event != Arrived || got.Path != "/opt/homebrew/bin/brew" {
		t.Errorf("got %+v, err %v", got, err)
	}
}

func TestWaitInstallArrival(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	probe := detect.Probe{Root: root, Home: home}
	store := Store{Dir: filepath.Join(t.TempDir(), "none")}
	if _, err := store.WaitInstall(context.Background(), probe, "homebrew", 100*time.Millisecond); !errors.Is(err, ErrWaitTimeout) {
		t.Errorf("err = %v", err)
	}
	brew := filepath.Join(root, "opt/homebrew/bin/brew")
	if err := os.MkdirAll(filepath.Dir(brew), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brew, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := store.WaitInstall(context.Background(), probe, "homebrew", time.Second)
	if err != nil || got.Event != Arrived || got.Path != "/opt/homebrew/bin/brew" {
		t.Errorf("got %+v, err %v", got, err)
	}
}
