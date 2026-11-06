package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

const token = "test-token"

var t0 = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

type fixture struct {
	srv   *Server
	store session.Store
	clock *clock
	host  string
}

func start(t *testing.T) fixture {
	t.Helper()
	store := session.Store{Dir: filepath.Join(t.TempDir(), "bootstrap")}
	if _, err := store.Begin(screen.ClaudeCode, 1, t0); err != nil {
		t.Fatal(err)
	}
	c := &clock{now: t0}
	srv, err := New(Config{
		Store: store,
		Token: token,
		Now:   c.Now,
		Page:  fstest.MapFS{"index.html": {Data: []byte("<h1>page</h1>")}, "a.js": {Data: []byte("js")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Serve()
	t.Cleanup(srv.Close)
	return fixture{srv: srv, store: store, clock: c, host: fmt.Sprintf("127.0.0.1:%d", srv.Port())}
}

// walk steps the server through time in steps short enough that none looks like a sleep.
func (f fixture) walk(from, to time.Time) ExitReason {
	var reason ExitReason
	for at := from; !at.After(to); at = at.Add(10 * time.Second) {
		if reason = f.srv.Step(at); reason != ExitNone {
			return reason
		}
	}
	return f.srv.Step(to)
}

func (f fixture) url(path string) string { return "http://" + f.host + path }

func (f fixture) request(t *testing.T, method, path, host, origin, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, f.url(path), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (f fixture) post(t *testing.T, body string) (int, string) {
	t.Helper()
	resp := f.request(t, http.MethodPost, "/answer?token="+token, "", "http://"+f.host, body)
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

func (f fixture) show(t *testing.T, req screen.Request, at time.Time) session.State {
	t.Helper()
	state, err := f.store.Show(req, 0, at)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// events is a plain SSE client.
type events struct {
	frames chan session.Frame
	close  func()
}

func (f fixture) connect(t *testing.T) *events {
	t.Helper()
	resp := f.request(t, http.MethodGet, "/events?token="+token, "", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("content type %q", got)
	}
	e := &events{frames: make(chan session.Frame, 32), close: func() { resp.Body.Close() }}
	go func() {
		defer close(e.frames)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if data, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
				var frame session.Frame
				if json.Unmarshal([]byte(data), &frame) == nil {
					e.frames <- frame
				}
			}
		}
	}()
	return e
}

func (e *events) next(t *testing.T) session.Frame {
	t.Helper()
	select {
	case frame, ok := <-e.frames:
		if !ok {
			t.Fatal("the stream closed")
		}
		return frame
	case <-time.After(5 * time.Second):
		t.Fatal("no frame arrived")
	}
	return session.Frame{}
}

func TestRefusals(t *testing.T) {
	f := start(t)
	good := func() string { return "http://" + f.host }
	cases := []struct {
		name         string
		method, path string
		host, origin string
		want         int
	}{
		{"missing token", "GET", "/", "", "", http.StatusForbidden},
		{"wrong token", "GET", "/?token=nope", "", "", http.StatusForbidden},
		{"empty token", "GET", "/?token=", "", "", http.StatusForbidden},
		{"wrong host", "GET", "/?token=" + token, "evil.example", "", http.StatusForbidden},
		{"localhost is not the host we gave", "GET", "/?token=" + token, strings.Replace(f.host, "127.0.0.1", "localhost", 1), "", http.StatusForbidden},
		{"wrong host port", "GET", "/?token=" + token, "127.0.0.1:1", "", http.StatusForbidden},
		{"wrong origin", "GET", "/?token=" + token, "", "http://evil.example", http.StatusForbidden},
		{"wrong origin scheme", "GET", "/?token=" + token, "", "https://" + f.host, http.StatusForbidden},
		{"null origin", "GET", "/?token=" + token, "", "null", http.StatusForbidden},
		{"events without a token", "GET", "/events", "", "", http.StatusForbidden},
		{"events with a wrong origin", "GET", "/events?token=" + token, "", "http://evil.example", http.StatusForbidden},
		{"answer without a token", "POST", "/answer", "", good(), http.StatusForbidden},
		{"answer with a wrong origin", "POST", "/answer?token=" + token, "", "http://evil.example", http.StatusForbidden},
		{"answer with no origin", "POST", "/answer?token=" + token, "", "", http.StatusForbidden},
		{"answer from the wrong host", "POST", "/answer?token=" + token, "evil.example", good(), http.StatusForbidden},
		{"files need the token too", "GET", "/a.js", "", "", http.StatusForbidden},
		{"unknown paths need the token too", "GET", "/nope", "", "", http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := f.request(t, c.method, c.path, c.host, c.origin, `{"seq":1,"choice":"x"}`)
			if resp.StatusCode != c.want {
				t.Errorf("status %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestAllowedRequests(t *testing.T) {
	f := start(t)
	resp := f.request(t, "GET", "/?token="+token, "", "", "")
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "<h1>page</h1>" {
		t.Errorf("page: %d %q", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("the URL holds the token, so it must not leak by referrer: %q", got)
	}
	if got := f.request(t, "GET", "/a.js?token="+token, "", "", "").StatusCode; got != 200 {
		t.Errorf("file status %d", got)
	}
	if got := f.request(t, "GET", "/nope?token="+token, "", "", "").StatusCode; got != 404 {
		t.Errorf("unknown path status %d", got)
	}
	if got := f.request(t, "GET", "/?token="+token, "", "http://"+f.host, "").StatusCode; got != 200 {
		t.Errorf("our own origin is fine: %d", got)
	}
	if got := f.request(t, "GET", "/answer?token="+token, "", "", "").StatusCode; got != 405 {
		t.Errorf("answer by GET: %d", got)
	}
}

func TestAnswerNeedsJSONContentType(t *testing.T) {
	f := start(t)
	state := f.show(t, screen.Request{Screen: "welcome"}, t0)
	req, _ := http.NewRequest("POST", f.url("/answer?token="+token), strings.NewReader(fmt.Sprintf(`{"seq":%d,"choice":"see-plan"}`, state.Current.Seq)))
	req.Header.Set("Origin", "http://"+f.host)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestBindsOnlyToLoopback(t *testing.T) {
	f := start(t)
	addr := f.srv.Addr()
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || !tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("listening on %v", addr)
	}
	// Reachable on loopback, and not by any other address this machine has.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ipnet.IP.String(), fmt.Sprint(f.srv.Port())), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			t.Errorf("reachable on %s", ipnet.IP)
		}
	}
}

func TestBindFailureIsReported(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	_, err = New(Config{Store: session.Store{Dir: t.TempDir()}, Token: token, Port: taken.Addr().(*net.TCPAddr).Port, Page: fstest.MapFS{}})
	if err == nil {
		t.Error("the port was taken")
	}
}

func TestEventsSendScreens(t *testing.T) {
	f := start(t)
	state := f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	if got := e.next(t); got.Screen == nil || got.Screen.Screen != "welcome" || got.Seq != state.Current.Seq {
		t.Fatalf("frame = %+v", got)
	}
	f.show(t, screen.Request{Screen: "list"}, t0)
	f.srv.Step(t0)
	if got := e.next(t); got.Screen.Screen != "list" || len(got.Screen.Rows) == 0 {
		t.Errorf("frame = %+v", got)
	}
}

func TestSameFrameIsNotResent(t *testing.T) {
	f := start(t)
	f.show(t, screen.Request{Screen: "welcome"}, t0)
	e := f.connect(t)
	defer e.close()
	e.next(t)
	f.srv.Step(t0)
	f.srv.Step(t0.Add(time.Second))
	select {
	case got := <-e.frames:
		t.Errorf("nothing changed, but got %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAnswerRefusals(t *testing.T) {
	f := start(t)
	first := f.show(t, screen.Request{Screen: "welcome"}, t0)
	naming := f.show(t, screen.Request{Screen: "naming"}, t0)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"an earlier screen", fmt.Sprintf(`{"seq":%d,"choice":"see-plan"}`, first.Current.Seq), http.StatusConflict},
		{"an unknown choice", fmt.Sprintf(`{"seq":%d,"choice":"nope","name":"x"}`, naming.Current.Seq), http.StatusUnprocessableEntity},
		{"an unsafe name", fmt.Sprintf(`{"seq":%d,"choice":"next","name":"../x"}`, naming.Current.Seq), http.StatusUnprocessableEntity},
		{"a command in a name", fmt.Sprintf(`{"seq":%d,"choice":"next","name":"$(rm -rf x)"}`, naming.Current.Seq), http.StatusUnprocessableEntity},
		{"not JSON", `nope`, http.StatusBadRequest},
		{"an unknown field", fmt.Sprintf(`{"seq":%d,"choice":"next","name":"x","extra":1}`, naming.Current.Seq), http.StatusBadRequest},
		{"a folder not offered", fmt.Sprintf(`{"seq":%d,"choice":"next","name":"x","in":"/etc"}`, naming.Current.Seq), http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, body := f.post(t, c.body); code != c.want {
				t.Errorf("status %d, want %d: %s", code, c.want, body)
			}
		})
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Answers) != 0 {
		t.Errorf("a refused answer reached the log: %+v", state.Answers)
	}
	code, body := f.post(t, fmt.Sprintf(`{"seq":%d,"choice":"next","name":"my-project","in":"%s"}`, naming.Current.Seq, screen.OtherPlace))
	if code != 200 {
		t.Fatalf("a safe name: %d %s", code, body)
	}
	if state, _ := f.store.Load(); len(state.Answers) != 1 || state.Answers[0].Answer.In != screen.OtherPlace {
		t.Errorf("the folder picked wasn't kept: %+v", state.Answers)
	}
	if code, _ := f.post(t, fmt.Sprintf(`{"seq":%d,"choice":"next","name":"other"}`, naming.Current.Seq)); code != http.StatusConflict {
		t.Errorf("a second answer: %d", code)
	}
}

func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if check() {
			return
		}
	}
	t.Fatal("timed out")
}
