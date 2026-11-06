package region

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const body = "## New projects\n\nRun bootstrap new.\n"

func TestMergeAddsTheBlockToAnEmptyFile(t *testing.T) {
	got := Merge("", "bootstrap", body)
	want := "<!-- bootstrap:begin -->\n" + body + "<!-- bootstrap:end -->\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeKeepsEveryLineThatWasThere(t *testing.T) {
	had := "# Mine\n\n- Write American English.\n- No emoji."
	got := Merge(had, "bootstrap", body)
	if !strings.HasPrefix(got, had+"\n\n<!-- bootstrap:begin -->\n") {
		t.Errorf("the old lines aren't kept, followed by a blank line and the block:\n%s", got)
	}
}

func TestMergeReplacesTheBlockInPlace(t *testing.T) {
	had := "# Mine\n\n<!-- bootstrap:begin -->\nold words\n<!-- bootstrap:end -->\n\n## After\n"
	got := Merge(had, "bootstrap", body)
	want := "# Mine\n\n<!-- bootstrap:begin -->\n" + body + "<!-- bootstrap:end -->\n\n## After\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeTwiceChangesNothing(t *testing.T) {
	once := Merge("# Mine\n", "bootstrap", body)
	if !strings.Contains(once, body) {
		t.Fatalf("the first merge lacks the block:\n%s", once)
	}
	if twice := Merge(once, "bootstrap", body); twice != once {
		t.Errorf("a second merge changed the file:\n%s", twice)
	}
}

func TestWriteMakesTheFolderAndKeepsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".config", "opencode", "AGENTS.md")
	if err := Write(path, "bootstrap", body); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Mine\n"+Merge("", "bootstrap", body)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, "bootstrap", body); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(got), "# Mine\n") || strings.Count(string(got), "bootstrap:begin") != 1 {
		t.Errorf("got:\n%s", got)
	}
}

func TestHas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if Has(path, "bootstrap") {
		t.Error("a missing file has the block")
	}
	if err := Write(path, "bootstrap", body); err != nil {
		t.Fatal(err)
	}
	if !Has(path, "bootstrap") {
		t.Error("a written file lacks the block")
	}
}
