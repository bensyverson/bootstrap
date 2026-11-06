package prefs_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/internal/prefs"
)

func TestNoFileIsNoPreferences(t *testing.T) {
	got, err := prefs.File{Dir: filepath.Join(t.TempDir(), "missing")}.Load()
	if err != nil || !reflect.DeepEqual(got, prefs.Preferences{}) {
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
	if got, err := f.Load(); err != nil || !reflect.DeepEqual(got, prefs.Preferences{}) {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestAnAnswerIsRememberedByItsName(t *testing.T) {
	dir := t.TempDir()
	f := prefs.File{Dir: dir}
	if err := f.Update(func(p *prefs.Preferences) { p.Remember(prefs.GitHub, "not-now") }); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "preferences.json"))
	if !strings.Contains(string(body), `"github": "not-now"`) {
		t.Errorf("preferences.json = %s", body)
	}
	got, err := f.Load()
	if err != nil {
		t.Fatal(err)
	}
	if choice, ok := got.Answer(prefs.GitHub); !ok || choice != "not-now" {
		t.Errorf("Answer(github) = %q, %v", choice, ok)
	}
	if _, ok := got.Answer(prefs.Settings); ok {
		t.Error("settings was never answered, but Answer says it was")
	}
}

func TestEntriesListWhatIsRememberedInTheOrderASetupMeetsIt(t *testing.T) {
	var p prefs.Preferences
	p.Remember(prefs.Settings, "not-now")
	p.Remember(prefs.GitHub, "not-now")
	p.ProjectsIn = "~/git"
	p.SkipTools = []string{"peep", "woodcase"}
	want := []prefs.Entry{
		{Name: prefs.SkipTools, Value: "peep,woodcase"},
		{Name: prefs.ProjectsIn, Value: "~/git"},
		{Name: prefs.GitHub, Value: "not-now"},
		{Name: prefs.Settings, Value: "not-now"},
	}
	if got := p.Entries(); !slices.Equal(got, want) {
		t.Errorf("Entries = %+v, want %+v", got, want)
	}
}

func TestForgetDropsOneAndKeepsTheRest(t *testing.T) {
	var p prefs.Preferences
	p.Remember(prefs.GitHub, "not-now")
	p.Remember(prefs.Settings, "yes")
	p.ProjectsIn = "~/git"
	p.SkipTools = []string{"peep"}
	for _, n := range []prefs.Name{prefs.GitHub, prefs.ProjectsIn, prefs.SkipTools} {
		if !p.Forget(n) {
			t.Errorf("Forget(%s) = false, though it was remembered", n)
		}
	}
	if p.Forget(prefs.Phone) {
		t.Error("Forget(phone) = true, though nothing was remembered")
	}
	want := []prefs.Entry{{Name: prefs.Settings, Value: "yes"}}
	if got := p.Entries(); !slices.Equal(got, want) {
		t.Errorf("Entries = %+v, want %+v", got, want)
	}
}

func TestParseNameKnowsEveryName(t *testing.T) {
	for _, n := range prefs.Names() {
		if got, err := prefs.ParseName(string(n)); err != nil || got != n {
			t.Errorf("ParseName(%s) = %q, %v", n, got, err)
		}
	}
}

func TestParseNameRefusesAnUnknownNameListingTheValidOnes(t *testing.T) {
	_, err := prefs.ParseName("gihtub")
	unknown, ok := errors.AsType[*prefs.UnknownNameError](err)
	if !ok {
		t.Fatalf("err = %v, want an UnknownNameError", err)
	}
	if !strings.Contains(unknown.Error(), "gihtub") || !slices.Contains(unknown.Valid, "github") {
		t.Errorf("err = %q, valid = %v", unknown.Error(), unknown.Valid)
	}
}
