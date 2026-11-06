package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapIsHomebrewsCopyOnceItIsThere(t *testing.T) {
	p := Probe{Root: t.TempDir(), Home: t.TempDir()}
	if got := Bootstrap(p); got != DownloadedBootstrap {
		t.Errorf("with no Homebrew copy, Bootstrap = %q, want %q", got, DownloadedBootstrap)
	}
	brewed := filepath.Join(p.Root, HomebrewBootstrap)
	if err := os.MkdirAll(filepath.Dir(brewed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brewed, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Bootstrap(p); got != HomebrewBootstrap {
		t.Errorf("with Homebrew's copy, Bootstrap = %q, want %q", got, HomebrewBootstrap)
	}
}

func TestIsDownloadedBootstrap(t *testing.T) {
	p := Probe{Home: "/Users/ada"}
	if !IsDownloadedBootstrap(p, "/Users/ada/.bootstrap/bin/bootstrap") {
		t.Error("the downloaded copy isn't recognized")
	}
	if IsDownloadedBootstrap(p, "/src/bootstrap/local/bs") {
		t.Error("a dev build is taken for the downloaded copy")
	}
}
