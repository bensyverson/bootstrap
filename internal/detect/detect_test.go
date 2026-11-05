package detect

import (
	"os"
	"path/filepath"
	"testing"
)

const appleGit = "/Library/Developer/CommandLineTools/usr/bin/git"

func touch(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, mode); err != nil {
		t.Fatal(err)
	}
}

func toolState(t *testing.T, r Report, id string) Tool {
	t.Helper()
	for _, tool := range r.Tools {
		if tool.ID == id {
			return tool
		}
	}
	t.Fatalf("no %s row in %+v", id, r.Tools)
	return Tool{}
}

func TestToolStates(t *testing.T) {
	cases := []struct {
		name     string
		files    []string // under the fake root; ~ is the fake home
		id       string
		state    ToolState
		wantPath string
	}{
		{"apple absent on an empty disk", nil, "clt", Missing, ""},
		{"apple absent while /usr/bin/git exists", []string{"/usr/bin/git"}, "clt", Missing, ""},
		{"git absent while /usr/bin/git exists", []string{"/usr/bin/git"}, "git", Missing, ""},
		{"apple present by its own git", []string{appleGit}, "clt", Present, appleGit},
		{"git present through apple's tools", []string{appleGit}, "git", Present, appleGit},
		{"git present through homebrew", []string{"/opt/homebrew/bin/git"}, "git", Present, "/opt/homebrew/bin/git"},
		{"homebrew absent", nil, "homebrew", Missing, ""},
		{"homebrew present", []string{"/opt/homebrew/bin/brew"}, "homebrew", Present, "/opt/homebrew/bin/brew"},
		{"jobs absent", nil, "jobs", Missing, ""},
		{"jobs present by its command name", []string{"/opt/homebrew/bin/job"}, "jobs", Present, "/opt/homebrew/bin/job"},
		{"sleepy present in the home go bin", []string{"~/go/bin/sleepy"}, "sleepy", Present, "~/go/bin/sleepy"},
		{"woodcase present", []string{"/opt/homebrew/bin/woodcase"}, "woodcase", Present, "/opt/homebrew/bin/woodcase"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, home := t.TempDir(), t.TempDir()
			for _, f := range c.files {
				if f[0] == '~' {
					touch(t, filepath.Join(home, f[1:]), 0o755)
				} else {
					touch(t, filepath.Join(root, f), 0o755)
				}
			}
			got := toolState(t, Run(Probe{Root: root, Home: home, Cwd: home}), c.id)
			if got.State != c.state || got.Path != c.wantPath {
				t.Errorf("got %+v, want state %s path %q", got, c.state, c.wantPath)
			}
		})
	}
}

func TestToolNeedsToBeExecutable(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	touch(t, filepath.Join(root, "/opt/homebrew/bin/brew"), 0o644)
	if got := toolState(t, Run(Probe{Root: root, Home: home, Cwd: home}), "homebrew"); got.State != Missing {
		t.Errorf("a file that can't run is not Homebrew: %+v", got)
	}
}

func TestFolderKinds(t *testing.T) {
	cases := []struct {
		name  string
		path  string // relative to the fake home
		files []string
		kind  FolderKind
		have  Presence
		git   Presence
	}{
		{"home", ".", nil, KindHome, No, No},
		{"desktop", "Desktop", nil, KindDesktop, No, No},
		{"documents", "Documents", nil, KindDocuments, No, No},
		{"downloads", "Downloads", []string{"a.zip"}, KindDownloads, Yes, No},
		{"a generic name", "Claude", nil, KindGeneric, No, No},
		{"a generic name with a space", "Claude Code", nil, KindGeneric, No, No},
		{"a generic name in another case", "work/CLAUDE", nil, KindGeneric, No, No},
		{"an ordinary folder", "pine-bakery", []string{"index.html"}, KindProject, Yes, No},
		{"an empty ordinary folder", "pine-bakery", nil, KindProject, No, No},
		{"a Git repo named Claude is a project", "Claude", []string{".git/HEAD"}, KindProject, No, Yes},
		{"a worktree's .git file also counts", "Claude", []string{".git"}, KindProject, No, Yes},
		{"Finder's own files aren't files", "pine", []string{".DS_Store", ".localized"}, KindProject, No, No},
		{"a Git repo at home is still home", ".", []string{".git/HEAD"}, KindHome, No, Yes},
		{"a folder inside Documents goes by its own name", "Documents/pine", nil, KindProject, No, No},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			cwd := filepath.Join(home, c.path)
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, f := range c.files {
				touch(t, filepath.Join(cwd, f), 0o644)
			}
			got := Run(Probe{Root: t.TempDir(), Home: home, Cwd: cwd}).Folder
			if got.Kind != c.kind || got.Files != c.have || got.Git != c.git {
				t.Errorf("got %+v, want kind=%s files=%s git=%s", got, c.kind, c.have, c.git)
			}
		})
	}
}

// macOS can refuse to list ~/Desktop. "No files" would be a plausible wrong answer.
func TestFolderFilesUnknownWhenUnreadable(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "locked")
	if err := os.Mkdir(cwd, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cwd, 0o755) })
	got := Run(Probe{Root: t.TempDir(), Home: home, Cwd: cwd}).Folder
	if got.Files != Unknown {
		t.Errorf("files = %s, want unknown", got.Files)
	}
}

func TestSymlinkedHomeStillMatches(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := Run(Probe{Root: t.TempDir(), Home: link, Cwd: real}).Folder.Kind; got != KindHome {
		t.Errorf("kind = %s, want home", got)
	}
}

func TestMacOSVersionAndChip(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	plist := filepath.Join(root, "System/Library/CoreServices/SystemVersion.plist")
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `<?xml version="1.0"?><plist version="1.0"><dict>
	<key>BuildID</key><string>X</string>
	<key>ProductVersion</key><string>26.1</string>
	</dict></plist>`
	if err := os.WriteFile(plist, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Run(Probe{Root: root, Home: home, Cwd: home, Chip: func() string { return "Apple M9" }})
	if got.MacOS != "26.1" || got.Chip != "Apple M9" || got.Arch == "" {
		t.Errorf("got %+v", got)
	}
}

func TestUnknownMacOSAndChipSaySo(t *testing.T) {
	home := t.TempDir()
	got := Run(Probe{Root: t.TempDir(), Home: home, Cwd: home, Chip: func() string { return "" }})
	if got.MacOS != "unknown" || got.Chip != "unknown" {
		t.Errorf("got macos=%q chip=%q, want unknown", got.MacOS, got.Chip)
	}
}

func TestDiskSpaceForApplesStep(t *testing.T) {
	cases := []struct {
		name  string
		free  func(string) (uint64, error)
		space Space
	}{
		{"plenty", func(string) (uint64, error) { return 120e9, nil }, SpaceEnough},
		{"exactly enough", func(string) (uint64, error) { return AppleStepNeeds, nil }, SpaceEnough},
		{"short, as the VM was", func(string) (uint64, error) { return 18_700_000_000, nil }, SpaceShort},
		{"macOS won't say", func(string) (uint64, error) { return 0, os.ErrPermission }, SpaceUnknown},
		{"nothing to ask", nil, SpaceUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			got := Run(Probe{Root: t.TempDir(), Home: home, Cwd: home, Free: c.free}).Disk
			if got.AppleStep != c.space {
				t.Errorf("apple step = %s, want %s", got.AppleStep, c.space)
			}
			if c.space != SpaceUnknown && got.Free == 0 {
				t.Errorf("free bytes missing: %+v", got)
			}
		})
	}
}
