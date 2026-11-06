// Package region keeps a named block of lines in a text file that people also
// edit by hand, such as an agent's instructions, between HTML comment markers.
// Everything outside the block is kept as it was.
package region

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func markers(name string) (begin, end string) {
	return "<!-- " + name + ":begin -->", "<!-- " + name + ":end -->"
}

// Merge returns file with the block called name holding body: replaced where it
// already is, or added at the end after a blank line.
func Merge(file, name, body string) string {
	begin, end := markers(name)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	block := begin + "\n" + body + end
	if i := strings.Index(file, begin); i >= 0 {
		if j := strings.Index(file[i:], end); j >= 0 {
			return file[:i] + block + file[i+j+len(end):]
		}
	}
	switch {
	case file == "":
	case strings.HasSuffix(file, "\n\n"):
	case strings.HasSuffix(file, "\n"):
		file += "\n"
	default:
		file += "\n\n"
	}
	return file + block + "\n"
}

// Write merges the block into the file at path, making the file and its folder
// when they aren't there.
func Write(path, name, body string) error {
	had, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	merged := Merge(string(had), name, body)
	if merged == string(had) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(merged), 0o644)
}

// Has reports whether the file at path holds the block called name.
func Has(path, name string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	begin, end := markers(name)
	i := strings.Index(string(body), begin)
	return i >= 0 && strings.Contains(string(body)[i:], end)
}
