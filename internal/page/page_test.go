package page

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/bensyverson/bootstrap/internal/screen"
)

// threeSHA256 pins the vendored Three.js r128, fetched from
// https://cdnjs.cloudflare.com/ajax/libs/three.js/r128/three.min.js and treated as a data file.
const threeSHA256 = "9274bbcec8d96168626c732b5d31c775aa8cfb7eaa0599bec0c175908a2c1ce2"

// ownText is every text file we wrote, which excludes the vendored library.
func ownText(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := fs.WalkDir(Files(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(path, "vendor/") {
			return err
		}
		if strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".js") {
			body, err := fs.ReadFile(Files(), path)
			if err != nil {
				return err
			}
			out[path] = string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEveryAssetIsEmbedded(t *testing.T) {
	for _, name := range []string{
		"index.html", "page.css", "page.js", "threads.js", "live.js", "preview.js",
		"vendor/three.min.js", "vendor/three.LICENSE",
		"fonts/Grand-Medium.subset.woff2", "fonts/Grand-Regular.subset.woff2",
		"cards/jobs.jpg", "cards/demo.jpg", "cards/demo2.jpg", "cards/design.jpg",
	} {
		if _, err := fs.Stat(Files(), name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestThreeIsTheFileWeFetched(t *testing.T) {
	body, err := fs.ReadFile(Files(), "vendor/three.min.js")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != threeSHA256 {
		t.Errorf("three.min.js is %s, want %s", got, threeSHA256)
	}
}

func TestThreeKeepsItsLicense(t *testing.T) {
	body, err := fs.ReadFile(Files(), "vendor/three.LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"MIT License", "three.js authors"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the license doesn't say %q", want)
		}
	}
}

// The tools' links arrive in frames, so nothing we wrote should hold an address.
func TestNothingWeWroteNamesAnAddress(t *testing.T) {
	address := regexp.MustCompile(`(?i)https?:|//[a-z0-9.-]+\.[a-z]{2,}|cdnjs|jsdelivr|unpkg`)
	for name, body := range ownText(t) {
		if found := address.FindString(body); found != "" {
			t.Errorf("%s names an address: %q", name, found)
		}
	}
}

func TestNothingBuildsHTMLFromAString(t *testing.T) {
	banned := regexp.MustCompile(`innerHTML|outerHTML|insertAdjacentHTML|document\.write|\beval\(|new Function`)
	for name, body := range ownText(t) {
		if found := banned.FindString(body); found != "" {
			t.Errorf("%s uses %s", name, found)
		}
	}
}

// A style attribute would need 'unsafe-inline', and so would a <style> element.
func TestNoInlineStyles(t *testing.T) {
	banned := regexp.MustCompile(`<[^>]*\sstyle=|<style|setAttribute\("style"`)
	for name, body := range ownText(t) {
		if found := banned.FindString(body); found != "" {
			t.Errorf("%s has an inline style: %q", name, found)
		}
	}
}

// Words reach the page in frames. Copy with a space in it can't be a piece of code by accident.
func TestThePageHoldsNoScreenWords(t *testing.T) {
	own := ownText(t)
	for _, words := range screen.Copy() {
		if !strings.Contains(words, " ") {
			continue
		}
		for name, body := range own {
			if strings.Contains(body, words) {
				t.Errorf("%s holds the screen's words %q", name, words)
			}
		}
	}
}

func TestTheStudyIsGone(t *testing.T) {
	for _, words := range []string{"I clicked it", "I did it", "Start over", "pickClaude", "oldMac", "#study", "TOOL_MS"} {
		for name, body := range ownText(t) {
			if strings.Contains(body, words) {
				t.Errorf("%s still has %q from the prototype's study", name, words)
			}
		}
	}
}

func TestPolicyLoadsNothingFromOutside(t *testing.T) {
	policy := Policy(Files())
	for _, want := range []string{"default-src 'none'", "script-src 'self' 'sha256-", "style-src 'self'", "font-src 'self'", "img-src 'self'", "connect-src 'self'", "base-uri 'none'", "form-action 'none'", "frame-ancestors 'none'"} {
		if !strings.Contains(policy, want) {
			t.Errorf("the policy lacks %q: %s", want, policy)
		}
	}
	for _, banned := range []string{"unsafe-inline", "unsafe-eval", "http:", "https:", "*", "data:", "blob:"} {
		if strings.Contains(policy, banned) {
			t.Errorf("the policy allows %q: %s", banned, policy)
		}
	}
}

func TestPolicyHashesTheInlineScript(t *testing.T) {
	const script = "start();\n"
	sum := sha256.Sum256([]byte(script))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	got := Policy(fstest.MapFS{"index.html": {Data: []byte("<body><script>" + script + "</script></body>")}})
	if !strings.Contains(got, want) {
		t.Errorf("got %s, want it to hold %s", got, want)
	}
}

func TestPolicyForAPageWithNoScript(t *testing.T) {
	got := Policy(fstest.MapFS{})
	if !strings.Contains(got, "script-src 'self'") || strings.Contains(got, "sha256") {
		t.Errorf("got %s", got)
	}
}

func TestThePageHasOneInlineScript(t *testing.T) {
	body, err := fs.ReadFile(Files(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(body), "<script"); got != 1 {
		t.Errorf("index.html has %d script elements; the loader is the only one", got)
	}
}

// A card named in a description needs something to draw, or it would quietly vanish.
func TestEveryCardHasATemplate(t *testing.T) {
	body, err := fs.ReadFile(Files(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range Previews() {
		if p.Frame.Screen == nil || p.Frame.Screen.Card == "" {
			continue
		}
		if !strings.Contains(string(body), `data-card="`+p.Frame.Screen.Card+`"`) {
			t.Errorf("%s asks for card %q, which has no template", p.Name, p.Frame.Screen.Card)
		}
	}
}

func TestEveryCardPictureExists(t *testing.T) {
	body, err := fs.ReadFile(Files(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`data-src="([^"]+)"`).FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		t.Fatal("no pictures found, so this test checks nothing")
	}
	for _, m := range matches {
		if _, err := fs.Stat(Files(), m[1]); err != nil {
			t.Errorf("%s: %v", m[1], err)
		}
	}
}

func previewNames() []string {
	var names []string
	for _, p := range Previews() {
		names = append(names, p.Name)
	}
	return names
}

func TestPreviewsDrawEverySample(t *testing.T) {
	files, err := screen.Samples.ReadDir("samples")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no samples")
	}
	names := previewNames()
	for _, f := range files {
		name := strings.TrimSuffix(f.Name(), ".json")
		if !slices.Contains(names, name) {
			t.Errorf("no preview for sample %s", name)
		}
	}
}

func TestPreviewsDrawEachState(t *testing.T) {
	names := previewNames()
	for _, want := range []string{
		"object",
		"tool.woodcase.downloading.claude-code",
		"tool.contextfill.installing.opencode",
		"building.git.claude-code",
		"trouble.other.opencode",
		"trouble.required.claude-code",
		"welcome.answered.claude-code",
		"list.answered.opencode",
		"naming.answered.claude-code",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("no preview named %s", want)
		}
	}
}

func TestPreviewNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range previewNames() {
		if seen[name] {
			t.Errorf("%s appears twice", name)
		}
		seen[name] = true
	}
}

func TestAnsweredPreviewsSayTheyAreAnswered(t *testing.T) {
	for _, p := range Previews() {
		if got, want := p.Frame.Answered, strings.Contains(p.Name, ".answered."); got != want {
			t.Errorf("%s answered = %v", p.Name, got)
		}
	}
}

func TestTheEndPreviewIsAnEndedFrame(t *testing.T) {
	for _, name := range []string{"end.claude-code", "end.opencode"} {
		i := slices.IndexFunc(Previews(), func(p Preview) bool { return p.Name == name })
		if i < 0 {
			t.Errorf("no preview named %s", name)
			continue
		}
		if f := Previews()[i].Frame; !f.Ended || f.Lost {
			t.Errorf("%s frame = %+v", name, f)
		}
	}
}

func TestARequiredToolHasNoSkipInItsPreview(t *testing.T) {
	for _, p := range Previews() {
		if p.Name != "trouble.required.claude-code" {
			continue
		}
		for _, c := range p.Frame.Screen.Choices {
			if c.ID == "skip" {
				t.Error("a required tool can't be skipped")
			}
		}
		return
	}
	t.Error("no preview of a required tool's trouble")
}

// Ben reviews the remembered default (XbdzWw) in the preview: a remembered
// answer is the main button, even "Not now".
func TestThePreviewShowsARememberedDefault(t *testing.T) {
	want := map[string]string{"github.remembered.claude-code": "not-now", "making.remembered.claude-code": "app"}
	for _, p := range Previews() {
		id, ok := want[p.Name]
		if !ok {
			continue
		}
		delete(want, p.Name)
		var primary []string
		for _, c := range p.Frame.Screen.Choices {
			if c.Primary {
				primary = append(primary, c.ID)
			}
		}
		if len(primary) != 1 || primary[0] != id {
			t.Errorf("%s: main buttons %v, want [%s]", p.Name, primary, id)
		}
	}
	for name := range want {
		t.Errorf("no preview %s", name)
	}
}
