package session

import (
	"testing"

	"github.com/bensyverson/bootstrap/internal/screen"
)

func rang(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestARingWakesEveryoneWaitingAndOnlyThem(t *testing.T) {
	b := NewBell()
	first, second := b.Rung(), b.Rung()
	if rang(first) {
		t.Fatal("rang before a ring")
	}
	b.Ring()
	if !rang(first) || !rang(second) {
		t.Error("a ring didn't wake everyone waiting")
	}
	if rang(b.Rung()) {
		t.Error("a wait that began after the ring heard it")
	}
}

func TestANilBellIsSilent(t *testing.T) {
	var b *Bell
	b.Ring()
	if b.Rung() != nil {
		t.Error("a nil bell gave a channel that can close")
	}
}

func TestAStoreRingsItsBellOnEveryWrite(t *testing.T) {
	store := driven(t)
	store.Bell = NewBell()
	var shown State
	for _, w := range []struct {
		name  string
		write func() error
	}{
		{"note", func() error { _, err := store.Note("installed jobs", t0); return err }},
		{"show", func() (err error) { shown, err = store.Show(screen.Request{Screen: "welcome"}, 0, t0); return err }},
		{"answer", func() error {
			_, err := store.RecordAnswer(shown.Current.Seq, screen.Answer{Choice: "skip-tour"}, t0)
			return err
		}},
		{"tabs", func() error { return store.SetTabs(1, t0) }},
	} {
		heard := store.Bell.Rung()
		if err := w.write(); err != nil {
			t.Fatalf("%s: %v", w.name, err)
		}
		if !rang(heard) {
			t.Errorf("%s didn't ring the bell", w.name)
		}
	}
}

func TestAWriteThatFailsDoesntRing(t *testing.T) {
	store := driven(t)
	store.Bell = NewBell()
	heard := store.Bell.Rung()
	if _, err := store.TakeHandoff(t0); err == nil {
		t.Fatal("took a handoff that isn't there")
	}
	if rang(heard) {
		t.Error("a refused write rang the bell")
	}
}
