// Package catalog holds the facts about each tool and the folder names that
// don't count as projects. The JSON files are embedded in the binary and are
// also read raw from the repo by the chat fallback, so they are the one source.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed tools.json
var toolsJSON []byte

//go:embed generic-folders.json
var genericJSON []byte

// Tool is one row of catalog/tools.json. Say may hold the {ai} placeholder.
type Tool struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Say      string `json:"say"`
	Required bool   `json:"required"`
	OwnStep  bool   `json:"own_step"`
	// Formula is the tool's name in bensyverson/tap, for every tool without its own step.
	Formula string   `json:"formula,omitempty"`
	Detect  []string `json:"detect"`
	// Installer is part of the installer's command line, for pgrep -f. An installer
	// that quits before Detect finds anything was canceled or failed.
	Installer string `json:"installer,omitempty"`
	// Started is where a person's go-ahead shows up, for installers that have one.
	Started *Sign `json:"started,omitempty"`
}

// Sign is a folder in Dir holding a file whose name starts with Prefix. Apple's
// updater makes one when someone clicks Agree, and removes it once it's installed.
type Sign struct {
	Dir    string `json:"dir"`
	Prefix string `json:"prefix"`
}

// load is once-only, and panics because the files are embedded: a parse failure
// is a build defect that a test catches, never a runtime condition.
var load = sync.OnceValues(func() ([]Tool, []string) {
	var tools struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		panic(fmt.Sprintf("catalog/tools.json: %v", err))
	}
	var generic struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(genericJSON, &generic); err != nil {
		panic(fmt.Sprintf("catalog/generic-folders.json: %v", err))
	}
	return tools.Tools, generic.Names
})

// Tools returns every tool, in the order the list screen shows them.
func Tools() []Tool {
	tools, _ := load()
	return tools
}

// Find returns the tool with the given id.
func Find(id string) (Tool, bool) {
	for _, t := range Tools() {
		if t.ID == id {
			return t, true
		}
	}
	return Tool{}, false
}

// IDs returns every tool id, in list order.
func IDs() []string {
	var ids []string
	for _, t := range Tools() {
		ids = append(ids, t.ID)
	}
	return ids
}

// IsGenericFolder reports whether a folder name is on the generic list, in any case.
func IsGenericFolder(name string) bool {
	_, names := load()
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}
