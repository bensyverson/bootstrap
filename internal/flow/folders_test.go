package flow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/prefs"
	"github.com/bensyverson/bootstrap/internal/screen"
)

// remember gives the world a preferences file, holding projectsIn when it's set.
func (w *world) remember(projectsIn string) {
	w.t.Helper()
	w.prefs = prefs.File{Dir: w.t.TempDir()}
	if projectsIn == "" {
		return
	}
	if err := w.prefs.Update(func(p *prefs.Preferences) { p.ProjectsIn = projectsIn }); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) remembered() string {
	w.t.Helper()
	p, err := w.prefs.Load()
	if err != nil {
		w.t.Fatal(err)
	}
	return p.ProjectsIn
}

func isFolder(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestANewProjectGoesInTheFolderGivenAndItIsRemembered(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.remember("")
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", In: "~/git"}); err != nil {
		t.Fatal(err)
	}
	if !isFolder(filepath.Join(w.home, "git", "pine-bakery", ".git")) {
		t.Error("the project isn't in ~/git")
	}
	if isFolder(filepath.Join(w.home, "Projects")) {
		t.Error("made ~/Projects for a project in ~/git")
	}
	if got := w.remembered(); got != "~/git" {
		t.Errorf("remembered %q, want ~/git", got)
	}
}

func TestThePageOffersTheRememberedFolderAndUsesIt(t *testing.T) {
	w := newWorld(t)
	w.remember("~/git")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !isFolder(filepath.Join(w.home, "git", "pine-bakery")) {
		t.Error("the page's project isn't in the remembered ~/git")
	}
	for _, a := range w.state().Answers {
		if a.Request.Screen == "naming" && (a.Request.In != "~/git" || a.Answer.In != "~/git") {
			t.Errorf("naming was %+v, answered %+v; want ~/git offered and kept", a.Request, a.Answer)
		}
	}
}

func TestThePageRemembersTheFolderPicked(t *testing.T) {
	w := newWorld(t)
	w.remember("")
	w.answers[asked{"naming", ""}] = screen.Answer{Choice: "next", Name: "Pine Bakery", In: "~/git"}
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if !isFolder(filepath.Join(w.home, "git", "pine-bakery")) {
		t.Error("the project isn't in the folder picked")
	}
	if got := w.remembered(); got != "~/git" {
		t.Errorf("remembered %q, want ~/git", got)
	}
}

// A project named before a folder was remembered stays where it was made.
func TestARememberedFolderNeverMovesAProject(t *testing.T) {
	w := newWorld(t)
	w.remember("")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(w.home, "Projects", "pine-bakery")
	before := snapshot(t, project)
	w.remember("~/git")
	if err := w.walk(); err != nil {
		t.Fatal(err)
	}
	if isFolder(filepath.Join(w.home, "git")) {
		t.Error("a remembered folder made a second copy of the project")
	}
	if after := snapshot(t, project); len(after) != len(before) {
		t.Error("the project changed")
	}
}

func TestAnInPlaceSetupIgnoresTheRememberedFolder(t *testing.T) {
	w := newWorld(t)
	w.withTools()
	w.remember("~/git")
	repo := filepath.Join(w.home, "code", "bakery")
	put(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	w.probe.Cwd = repo
	// A name and folder given in place are set aside with the rest of the naming question.
	if err := w.runAt(&person{}, flow.Given{Name: "pine-bakery", In: "~/git"}); err != nil {
		t.Fatal(err)
	}
	for _, a := range w.state().Answers {
		if a.Request.Screen == "naming" {
			t.Errorf("an in-place setup recorded a name: %+v", a)
		}
	}
	if isFolder(filepath.Join(w.home, "git")) {
		t.Error("an in-place setup made the remembered folder")
	}
	if !isFolder(filepath.Join(repo, "local")) {
		t.Error("the setup didn't happen in place")
	}
}
