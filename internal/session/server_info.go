package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ServerInfo is how a later serve finds the running server. It holds the token, so it's owner-only.
type ServerInfo struct {
	PID  int    `json:"pid"`
	URL  string `json:"url"`
	Port int    `json:"port"`
	// Flow says the server runs the setup itself, as run starts it; serve's doesn't.
	Flow bool `json:"flow,omitempty"`
}

func (s Store) infoPath() string { return filepath.Join(s.Dir, "server.json") }
func (s Store) lockPath() string { return filepath.Join(s.Dir, "server.lock") }

// WriteServerInfo replaces the record of the running server in one step.
func (s Store) WriteServerInfo(info ServerInfo) error {
	if err := ensureDir(s.Dir); err != nil {
		return err
	}
	body, err := json.Marshal(info)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, "server-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.infoPath())
}

// ReadServerInfo reads the record of the running server.
func (s Store) ReadServerInfo() (ServerInfo, error) {
	body, err := os.ReadFile(s.infoPath())
	if err != nil {
		return ServerInfo{}, err
	}
	var info ServerInfo
	err = json.Unmarshal(body, &info)
	return info, err
}

// RemoveServerInfo forgets the running server.
func (s Store) RemoveServerInfo() { os.Remove(s.infoPath()) }

// LockServer claims the one server slot until release, or until the process dies:
// the kernel drops the lock either way, so a crash can't leave a stale claim, and
// no command has to trust a pid that may have been reused.
func (s Store) LockServer() (release func(), err error) {
	if err := ensureDir(s.Dir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	// A command probing the lock holds it for an instant, so a refusal gets a few retries.
	for attempt := 0; ; attempt++ {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		if attempt == 4 {
			f.Close()
			return nil, ErrServerRunning
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ServerRunning reports whether a server holds the lock. It doesn't talk to the server.
func (s Store) ServerRunning() bool {
	f, err := os.Open(s.lockPath())
	if err != nil {
		return false
	}
	defer f.Close()
	return errors.Is(syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB), syscall.EWOULDBLOCK)
}
