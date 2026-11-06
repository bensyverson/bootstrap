package prefs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/prefs"
)

func TestNoFileIsNoPreferences(t *testing.T) {
	got, err := prefs.File{Dir: filepath.Join(t.TempDir(), "missing")}.Load()
	if err != nil || got != (prefs.Preferences{}) {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestAnUpdateIsKeptInPreferencesJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".bootstrap")
	f := prefs.File{Dir: dir}
	if err := f.Update(func(p *prefs.Preferences) { p.ProjectsIn = "~/code" }); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "preferences.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"projects_in": "~/code"`) {
		t.Errorf("preferences.json = %s", body)
	}
	got, err := f.Load()
	if err != nil || got.ProjectsIn != "~/code" {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestAFileThatIsNotJSONIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "preferences.json"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (prefs.File{Dir: dir}).Load(); err == nil || !strings.Contains(err.Error(), "preferences.json") {
		t.Errorf("err = %v, want one naming the file", err)
	}
}

func TestNoFolderRemembersNothing(t *testing.T) {
	var f prefs.File
	if err := f.Update(func(p *prefs.Preferences) { p.ProjectsIn = "~/code" }); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Load(); err != nil || got != (prefs.Preferences{}) {
		t.Errorf("Load = %+v, %v", got, err)
	}
}
