package detect

import (
	"os"
	"path/filepath"
	"strings"
)

// Stack is a language a project folder's files show it's written in.
type Stack string

const (
	StackGo     Stack = "go"
	StackPython Stack = "python"
	StackSwift  Stack = "swift"
)

// stackMarkers are the top-level files that show each stack, in the order
// ProjectStacks reports them.
var stackMarkers = []struct {
	stack Stack
	files []string
}{
	{StackGo, []string{"go.mod"}},
	{StackPython, []string{"pyproject.toml", "requirements.txt", "setup.py"}},
	{StackSwift, []string{"Package.swift"}},
}

// ProjectStacks reads a project folder's top level for each stack's marker
// files; an .xcodeproj also shows Swift. A folder that can't be read says nothing.
func ProjectStacks(dir string) []Stack {
	var out []Stack
	for _, m := range stackMarkers {
		for _, f := range m.files {
			if isFile(filepath.Join(dir, f)) {
				out = append(out, m.stack)
				break
			}
		}
	}
	if len(out) > 0 && out[len(out)-1] == StackSwift {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
			return append(out, StackSwift)
		}
	}
	return out
}
