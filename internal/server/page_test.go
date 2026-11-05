package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/page"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// startReal serves the real embedded page rather than the small test one.
func startReal(t *testing.T) fixture {
	t.Helper()
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, t0); err != nil {
		t.Fatal(err)
	}
	c := &clock{now: t0}
	srv, err := New(Config{Store: store, Token: token, Now: c.Now, Page: page.Files()})
	if err != nil {
		t.Fatal(err)
	}
	srv.Serve()
	t.Cleanup(srv.Close)
	return fixture{srv: srv, store: store, clock: c, host: fmt.Sprintf("127.0.0.1:%d", srv.Port())}
}

func TestEveryResponseCarriesThePolicy(t *testing.T) {
	f := startReal(t)
	want := page.Policy(page.Files())
	if want == "" {
		t.Fatal("no policy")
	}
	for _, path := range []string{"/?token=" + token, "/page.js?token=" + token, "/preview?token=" + token, "/preview.json?token=" + token} {
		resp := f.request(t, http.MethodGet, path, "", "", "")
		if got := resp.Header.Get("Content-Security-Policy"); got != want {
			t.Errorf("%s: policy = %q", path, got)
		}
	}
}

func TestAssetsAreServedAsWhatTheyAre(t *testing.T) {
	f := startReal(t)
	for path, want := range map[string]string{
		"/":                                 "text/html",
		"/page.css":                         "text/css",
		"/page.js":                          "text/javascript",
		"/vendor/three.min.js":              "text/javascript",
		"/fonts/Grand-Medium.subset.woff2":  "font/woff2",
		"/fonts/Grand-Regular.subset.woff2": "font/woff2",
		"/cards/jobs.jpg":                   "image/jpeg",
	} {
		resp := f.request(t, http.MethodGet, path+"?token="+token, "", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, want) {
			t.Errorf("%s: type %q, want %q", path, got, want)
		}
	}
}

func TestEveryAssetNeedsTheToken(t *testing.T) {
	f := startReal(t)
	for _, path := range []string{"/page.js", "/fonts/Grand-Medium.subset.woff2", "/cards/jobs.jpg", "/preview", "/preview.json"} {
		if resp := f.request(t, http.MethodGet, path, "", "", ""); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status %d without a token", path, resp.StatusCode)
		}
	}
}

func TestPreviewServesThePage(t *testing.T) {
	f := startReal(t)
	preview, err := io.ReadAll(f.request(t, http.MethodGet, "/preview?token="+token, "", "", "").Body)
	if err != nil {
		t.Fatal(err)
	}
	home, err := io.ReadAll(f.request(t, http.MethodGet, "/?token="+token, "", "", "").Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(home) == 0 || string(preview) != string(home) {
		t.Errorf("preview serves %d bytes, the page is %d", len(preview), len(home))
	}
}

func TestPreviewListsEveryState(t *testing.T) {
	f := startReal(t)
	resp := f.request(t, http.MethodGet, "/preview.json?token="+token, "", "", "")
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("type = %q", got)
	}
	var got []page.Preview
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if want := len(page.Previews()); want == 0 || len(got) != want {
		t.Errorf("got %d previews, want %d", len(got), want)
	}
}

// A preview is for looking, so it mustn't stand in for a person's tab.
func TestPreviewDoesNotCountAsATab(t *testing.T) {
	f := startReal(t)
	f.request(t, http.MethodGet, "/preview?token="+token, "", "", "")
	f.request(t, http.MethodGet, "/preview.json?token="+token, "", "", "")
	if got := f.srv.Tabs(); got != 0 {
		t.Errorf("tabs = %d", got)
	}
}

func TestLostWordsArriveBeforeTheServerGoes(t *testing.T) {
	f := startReal(t)
	e := f.connect(t)
	defer e.close()
	frame := e.next(t)
	if frame.Gone == nil || frame.Gone.Screen != screen.Lost {
		t.Fatalf("first frame = %+v", frame)
	}
	f.show(t, screen.Request{Screen: "welcome"}, t0.Add(time.Second))
	f.srv.Step(t0.Add(time.Second))
	if frame := e.next(t); frame.Gone == nil || frame.Screen.Screen != "welcome" {
		t.Errorf("next frame = %+v", frame)
	}
}
