package session

import "sync"

// Bell tells waiters in this process that the log has changed, so they read it
// now instead of at their next poll. It says nothing about what changed: the log
// stays the one source of truth. Writes from other processes don't ring it.
type Bell struct {
	mu   sync.Mutex
	rung chan struct{}
}

// NewBell makes a bell that no one has rung.
func NewBell() *Bell { return &Bell{rung: make(chan struct{})} }

// Ring wakes everyone waiting on Rung. A nil bell is silent.
func (b *Bell) Ring() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	close(b.rung)
	b.rung = make(chan struct{})
}

// Rung closes at the next ring. Take it before reading the log, so a write
// between the read and the wait isn't missed. A nil bell's never closes.
func (b *Bell) Rung() <-chan struct{} {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rung
}
