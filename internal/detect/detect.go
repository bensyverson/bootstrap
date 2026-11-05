// Package detect reports what's on this Mac and what kind of folder this is.
// It only reports: what to do about any of it is the agent's call.
package detect

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/bensyverson/bootstrap/catalog"
)

const unknownValue = "unknown"

// systemVersionFile is read directly because it needs no tool, and it's stock on every macOS.
const systemVersionFile = "/System/Library/CoreServices/SystemVersion.plist"

var productVersion = regexp.MustCompile(`<key>ProductVersion</key>\s*<string>([^<]+)</string>`)

// ignoredFiles don't mean a person put anything here: Finder makes the first two.
var ignoredFiles = map[string]bool{".DS_Store": true, ".localized": true, ".git": true}

// FolderKind is the closed list of what a folder can be.
type FolderKind string

const (
	KindHome      FolderKind = "home"
	KindDesktop   FolderKind = "desktop"
	KindDocuments FolderKind = "documents"
	KindDownloads FolderKind = "downloads"
	KindGeneric   FolderKind = "generic"
	KindProject   FolderKind = "project"
)

// Presence is yes, no, or unknown when macOS refuses to let us look.
type Presence string

const (
	Yes     Presence = "yes"
	No      Presence = "no"
	Unknown Presence = "unknown"
)

// ToolState is whether a tool is on this Mac.
type ToolState string

const (
	Present ToolState = "present"
	Missing ToolState = "missing"
)

// Folder describes the working folder.
type Folder struct {
	Path  string     `json:"path"`
	Kind  FolderKind `json:"kind"`
	Files Presence   `json:"files"`
	Git   Presence   `json:"git"`
}

// Tool is one catalog tool's state, with the path that proved it.
type Tool struct {
	ID    string    `json:"id"`
	State ToolState `json:"state"`
	Path  string    `json:"path,omitempty"`
}

// Report is everything detect says.
type Report struct {
	Chip   string `json:"chip"`
	Arch   string `json:"arch"`
	MacOS  string `json:"macos"`
	Folder Folder `json:"folder"`
	Tools  []Tool `json:"tools"`
}

// Probe says where to look. Root prefixes absolute paths so tests can use a fake filesystem.
type Probe struct {
	Root string
	Home string
	Cwd  string
	Chip func() string
}

// Run takes the whole report.
func Run(p Probe) Report {
	report := Report{
		Chip:   unknownValue,
		Arch:   runtime.GOARCH,
		MacOS:  macOSVersion(p),
		Folder: inspectFolder(p),
	}
	if p.Chip != nil {
		if chip := p.Chip(); chip != "" {
			report.Chip = chip
		}
	}
	for _, tool := range catalog.Tools() {
		row := Tool{ID: tool.ID, State: Missing}
		if path, ok := Locate(p, tool.Detect); ok {
			row.State, row.Path = Present, path
		}
		report.Tools = append(report.Tools, row)
	}
	return report
}

// Locate returns the first of a tool's detect paths that holds an executable, as the path a person would type.
func Locate(p Probe, paths []string) (string, bool) {
	for _, path := range paths {
		info, err := os.Stat(resolve(p, path))
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return path, true
		}
	}
	return "", false
}

func resolve(p Probe, path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(p.Home, rest)
	}
	return filepath.Join(p.Root, path)
}

func macOSVersion(p Probe) string {
	body, err := os.ReadFile(filepath.Join(p.Root, systemVersionFile))
	if err != nil {
		return unknownValue
	}
	match := productVersion.FindSubmatch(body)
	if match == nil {
		return unknownValue
	}
	return string(match[1])
}

func inspectFolder(p Probe) Folder {
	folder := Folder{Path: p.Cwd, Files: listFiles(p.Cwd), Git: No}
	if _, err := os.Lstat(filepath.Join(p.Cwd, ".git")); err == nil {
		folder.Git = Yes
	} else if !errors.Is(err, fs.ErrNotExist) {
		folder.Git = Unknown
	}
	folder.Kind = classify(p.Home, p.Cwd, folder.Git)
	return folder
}

func listFiles(dir string) Presence {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Unknown
	}
	for _, entry := range entries {
		if !ignoredFiles[entry.Name()] {
			return Yes
		}
	}
	return No
}

func classify(home, cwd string, git Presence) FolderKind {
	home, cwd = canonical(home), canonical(cwd)
	standard := map[string]FolderKind{
		home:                             KindHome,
		filepath.Join(home, "Desktop"):   KindDesktop,
		filepath.Join(home, "Documents"): KindDocuments,
		filepath.Join(home, "Downloads"): KindDownloads,
	}
	// macOS folders are case-insensitive by default, so ~/desktop is ~/Desktop.
	for path, kind := range standard {
		if strings.EqualFold(path, cwd) {
			return kind
		}
	}
	if git != Yes && catalog.IsGenericFolder(filepath.Base(cwd)) {
		return KindGeneric
	}
	return KindProject
}

func canonical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}
