package catalog

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestEmbeddedMatchesRawFiles(t *testing.T) {
	for name, embedded := range map[string][]byte{"tools.json": toolsJSON, "generic-folders.json": genericJSON} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, embedded) {
			t.Errorf("%s: embedded copy differs from the raw file", name)
		}
	}
}

func TestToolsLoadInListOrder(t *testing.T) {
	want := []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill", "sleepy", "peep", "woodcase"}
	if got := IDs(); !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestToolFacts(t *testing.T) {
	jobs, ok := Find("jobs")
	if !ok || jobs.Name != "Jobs" {
		t.Errorf("the tool must read as Jobs, got %+v", jobs)
	}
	for _, id := range []string{"sleepy", "peep", "woodcase"} {
		if tool, _ := Find(id); tool.Required {
			t.Errorf("%s must be skippable", id)
		}
	}
	if _, ok := Find("oldMac"); ok {
		t.Error("oldMac is not a tool")
	}
}

// /usr/bin/git is a stub until Apple's tools arrive, so it can never be evidence of anything.
func TestAppleToolsAreDetectedOnlyByTheirOwnGit(t *testing.T) {
	clt, _ := Find("clt")
	if want := []string{"/Library/Developer/CommandLineTools/usr/bin/git"}; !slices.Equal(clt.Detect, want) {
		t.Errorf("clt detect = %v, want %v", clt.Detect, want)
	}
	for _, tool := range Tools() {
		for _, path := range tool.Detect {
			if strings.HasPrefix(path, "/usr/bin/") {
				t.Errorf("%s detects by stub %s", tool.ID, path)
			}
		}
		if len(tool.Detect) == 0 {
			t.Errorf("%s has no detect paths", tool.ID)
		}
	}
}

func TestGenericFolderAnyCase(t *testing.T) {
	cases := map[string]bool{
		"Claude": true, "claude": true, "CLAUDE CODE": true, "claude code": true, "OpenCode": true,
		"pine-bakery": false, "Claude Monet": false,
	}
	for name, want := range cases {
		if got := IsGenericFolder(name); got != want {
			t.Errorf("IsGenericFolder(%q) = %v, want %v", name, got, want)
		}
	}
}

// The flow installs each tool from the tap by formula, which isn't always the tool's id.
func TestEveryTapToolNamesItsFormula(t *testing.T) {
	for _, tool := range Tools() {
		if tool.OwnStep {
			if tool.Formula != "" {
				t.Errorf("%s has its own step, not a formula", tool.ID)
			}
			continue
		}
		if tool.Formula == "" {
			t.Errorf("%s has no formula", tool.ID)
		}
	}
	if jobs, _ := Find("jobs"); jobs.Formula != "job" {
		t.Errorf("jobs installs as bensyverson/tap/job, got %q", jobs.Formula)
	}
}
