// Package prefs keeps what bootstrap remembers between setups, in its own file
// beside its sessions: ~/.bootstrap/preferences.json (Ben, 2026-11-05), so
// removing ~/.bootstrap undoes everything.
package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is the preferences file's name in bootstrap's folder.
const FileName = "preferences.json"

// Preferences are what the person chose once and shouldn't be asked again.
type Preferences struct {
	// ProjectsIn is the folder new projects go in, as "~/git" or an absolute
	// path. Empty is the default, ~/Projects.
	ProjectsIn string `json:"projects_in,omitempty"`
}

// File is the preferences file in Dir. A File with no Dir remembers nothing.
type File struct {
	Dir string
}

// Path is the file's path.
func (f File) Path() string { return filepath.Join(f.Dir, FileName) }

// Load reads the preferences. No file is no preferences yet.
func (f File) Load() (Preferences, error) {
	var p Preferences
	if f.Dir == "" {
		return p, nil
	}
	body, err := os.ReadFile(f.Path())
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return Preferences{}, fmt.Errorf("%s isn't valid: %w", f.Path(), err)
	}
	return p, nil
}

// Update changes the preferences and writes them back whole, so a reader never
// sees half a file.
func (f File) Update(change func(*Preferences)) error {
	if f.Dir == "" {
		return nil
	}
	p, err := f.Load()
	if err != nil {
		return err
	}
	change(&p)
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(f.Dir, FileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.Path())
}
