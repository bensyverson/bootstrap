package session

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
)

func startedStore(t *testing.T) Store {
	t.Helper()
	agent, err := screen.FindAgent("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	store := Store{Dir: t.TempDir()}
	if _, err := store.Begin(agent, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Show(screen.Request{Screen: "welcome"}, 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	return store
}

func appendRaw(t *testing.T, store Store, text string) {
	t.Helper()
	f, err := os.OpenFile(store.LogPath(), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

// A reader can catch a line mid-write, and a crash can leave one forever. Neither
// is corruption: the event never finished, so it never happened.
func TestLoadIgnoresAHalfWrittenLastLine(t *testing.T) {
	store := startedStore(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	appendRaw(t, store, `{"seq":99,"kind":"sh`)
	after, err := store.Load()
	if err != nil {
		t.Fatalf("a half-written line should be ignored, got %v", err)
	}
	if after.Seq != before.Seq {
		t.Errorf("seq = %d, want %d", after.Seq, before.Seq)
	}
}

// Appending straight after a crash's leftovers would glue two events into one bad line.
func TestWritingAfterAHalfWrittenLineKeepsTheLogReadable(t *testing.T) {
	store := startedStore(t)
	appendRaw(t, store, `{"seq":99,"kind":"sh`)
	if _, err := store.Show(screen.Request{Screen: "list"}, 0, time.Now()); err != nil {
		t.Fatalf("show after a half-written line: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load after the repair: %v", err)
	}
	if state.Current == nil || state.Current.Request.Screen != "list" {
		t.Errorf("current = %+v, want the list screen", state.Current)
	}
}

// A finished line that can't be read really is damage, and hiding it would lose events.
func TestABrokenFinishedLineIsStillCorrupt(t *testing.T) {
	store := startedStore(t)
	appendRaw(t, store, "not json\n")
	_, err := store.Load()
	var corrupt *CorruptError
	if !errors.As(err, &corrupt) {
		t.Errorf("err = %v, want a CorruptError", err)
	}
}
