package screen

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bensyverson/bootstrap/catalog"
)

var update = flag.Bool("update", false, "rewrite samples/ from the Go definitions")

const prototypePath = "../../project/2026-11-05-setup-page-mock/prototype/template.html"

// sampleRequest is the request behind each screen's checked-in sample.
func sampleRequest(id ID) Request {
	switch id {
	case "tool":
		return Request{Screen: id, Tool: "jobs", Status: "installing"}
	case "trouble":
		return Request{Screen: id, Tool: "sleepy", Reason: "network"}
	case "building":
		return Request{Screen: id, Status: "folder"}
	case "github-code":
		return Request{Screen: id, Code: sampleCode}
	case "identity":
		return Request{Screen: id, Name: sampleName, Email: sampleEmail}
	}
	return Request{Screen: id}
}

func TestSamplesMatchDescriptions(t *testing.T) {
	seen := map[string]bool{}
	for _, agent := range Agents() {
		for _, def := range Definitions() {
			name := fmt.Sprintf("%s.%s.json", def.Screen, agent.ID)
			seen[name] = true
			t.Run(name, func(t *testing.T) {
				var got Description
				var err error
				switch def.Screen {
				case Lost:
					got = DescribeLost(Context{Agent: agent, Carried: 0.5})
				case End:
					got = DescribeEnd(Context{Agent: agent, Carried: 0.5})
				default:
					got, err = Describe(sampleRequest(def.Screen), Context{Agent: agent, Carried: 0.5})
				}
				if err != nil {
					t.Fatal(err)
				}
				body, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				body = append(body, '\n')
				path := filepath.Join("samples", name)
				if *update {
					if err := os.WriteFile(path, body, 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%v (regenerate with go test ./internal/screen -update)", err)
				}
				if string(body) != string(want) {
					t.Errorf("%s differs from the description (regenerate with -update if the change is meant):\ngot:\n%s", path, body)
				}
			})
		}
	}
	// A sample for a screen that's gone would keep drawing on the preview route.
	files, err := os.ReadDir("samples")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !seen[f.Name()] {
			t.Errorf("samples/%s has no screen", f.Name())
		}
	}
	if len(seen) == 0 {
		t.Error("no screens defined")
	}
}

func TestSamplesAreEmbedded(t *testing.T) {
	body, err := Samples.ReadFile("samples/welcome.opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "OpenCode") {
		t.Errorf("embedded sample = %s", body)
	}
}

// fragments splits a copy string around its placeholders, leaving only the words
// the prototype should hold verbatim.
func fragments(s string) []string {
	var out []string
	for _, f := range regexp.MustCompile(`\{[a-z:-]+\}`).Split(s, -1) {
		if strings.TrimSpace(f) != "" {
			out = append(out, f)
		}
	}
	return out
}

// Words the plan changed or added on purpose, so not expected in the prototype.
var recordedCopyChanges = map[string]bool{
	"Whoops! We lost your {app} session.":         true,
	"Open it back up and type “continue”.":        true,
	OtherFailureLine:                              true,
	" didn’t install.":                            true,
	"Skip {tool}":                                 true,
	", which include Git. This can take a while.": true,
	"Your turn: click through the installer.":     true,
	"This adds ":                                  true,
	", which installs other software for you. It will ask for your Mac password.": true,
	"Apple’s tools, which include Git.":                                           true,
	"Let’s talk in {app}.":                                                        true,
	"{ai} has a question.":                                                        true,
	"That’s all for now.":                                                         true,
}

func TestCopyComesFromThePrototype(t *testing.T) {
	drafted := map[string]bool{}
	for _, sp := range specs {
		if sp.draft {
			for _, w := range sp.words() {
				drafted[w] = true
			}
		}
	}
	raw, err := os.ReadFile(prototypePath)
	if err != nil {
		t.Fatal(err)
	}
	prototype := string(raw)
	if len(Copy()) < 40 {
		t.Fatalf("Copy() returned only %d strings", len(Copy()))
	}
	for _, s := range Copy() {
		// Drafts wait on Ben's approval; TestDraftScreens keeps them listed.
		if recordedCopyChanges[s] || drafted[s] {
			continue
		}
		for _, f := range fragments(s) {
			if recordedCopyChanges[f] {
				continue
			}
			if !strings.Contains(prototype, f) {
				t.Errorf("copy %q: %q isn't in the prototype", s, f)
			}
		}
	}
}

func TestRecordedWordsAreExact(t *testing.T) {
	lost := DescribeLost(Context{Agent: OpenCode})
	if got := plain(lost.Title); got != "Whoops! We lost your OpenCode session." {
		t.Errorf("lost title = %q", got)
	}
	if got := plain(lost.Status); got != "Open it back up and type “continue”." {
		t.Errorf("lost line = %q", got)
	}
	end := DescribeEnd(Context{Agent: OpenCode})
	if got := plain(end.Title); got != "That’s all for now." {
		t.Errorf("end title = %q", got)
	}
	if got := plain(end.Status); got != "You can close this and go back to OpenCode." {
		t.Errorf("end line = %q", got)
	}
	if end.Mode != Settled || !IsSettled(End) {
		t.Errorf("the end settles the object: %+v", end)
	}
}

func TestChatScreen(t *testing.T) {
	for _, tc := range []struct {
		agent       Agent
		title, line string
	}{
		{ClaudeCode, "Let’s talk in Claude Code.", "Claude has a question."},
		{OpenCode, "Let’s talk in OpenCode.", "OpenCode has a question."},
	} {
		d, err := Describe(Request{Screen: "chat"}, Context{Agent: tc.agent})
		if err != nil {
			t.Fatal(err)
		}
		if got := plain(d.Title); got != tc.title {
			t.Errorf("title = %q", got)
		}
		if got := plain(d.Status); got != tc.line {
			t.Errorf("line = %q", got)
		}
		if len(d.Choices) != 0 || d.Mode != Waiting {
			t.Errorf("chat has no buttons and waits on the person: %+v", d)
		}
	}
	if !AwaitsChat("chat") {
		t.Error("chat waits on a reply in the chat")
	}
	if AwaitsClick("chat") {
		t.Error("chat has nothing to click")
	}
	for _, id := range []ID{"welcome", "terminal", "tool", "done", Lost} {
		if AwaitsChat(id) {
			t.Errorf("%s doesn't wait on the chat", id)
		}
	}
}

func TestTheToolsListMatchesThePrototype(t *testing.T) {
	raw, err := os.ReadFile(prototypePath)
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)const TOOLS = \[(.*?)\n\];`).FindSubmatch(raw)
	if block == nil {
		t.Fatal("no TOOLS in the prototype")
	}
	row := regexp.MustCompile(`name: "([^"]+)".*?say: "([^"]+)"(.*)`)
	matches := row.FindAllSubmatch(block[1], -1)
	if len(matches) != len(catalog.Tools()) {
		t.Fatalf("prototype has %d tools, catalog has %d", len(matches), len(catalog.Tools()))
	}
	for i, m := range matches {
		tool := catalog.Tools()[i]
		sayMatches := tool.Say == string(m[2]) || recordedCopyChanges[tool.Say]
		if tool.Name != string(m[1]) || !sayMatches {
			t.Errorf("tool %d: catalog %q %q, prototype %q %q", i, tool.Name, tool.Say, m[1], m[2])
		}
		if needed := strings.Contains(string(m[3]), "needed: true"); needed != tool.Required {
			t.Errorf("%s: required = %v, prototype needed = %v", tool.ID, tool.Required, needed)
		}
	}
}

func TestEveryListToolIsInTheCatalog(t *testing.T) {
	list, err := Describe(Request{Screen: "list"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) == 0 {
		t.Fatal("the list screen has no rows")
	}
	for _, row := range list.Rows {
		tool, ok := catalog.Find(row.ID)
		if !ok {
			t.Errorf("row %s isn't in the catalog", row.ID)
			continue
		}
		if row.Name != tool.Name || row.URL != tool.URL || row.Required != tool.Required {
			t.Errorf("row %+v disagrees with catalog %+v", row, tool)
		}
	}
}

func TestDroppedThingsAreGone(t *testing.T) {
	if len(Definitions()) == 0 || len(Copy()) == 0 {
		t.Fatal("nothing to check")
	}
	for _, def := range Definitions() {
		if def.Screen == "oldMac" || def.Screen == "old-mac" {
			t.Error("the needs-a-newer-Mac screen is dropped")
		}
	}
	for _, s := range Copy() {
		if s == "I did it" || s == "I clicked it" {
			t.Errorf("%q is not in the product", s)
		}
	}
}

func allRequests() []Request {
	var requests []Request
	for _, def := range Definitions() {
		if def.Internal {
			continue
		}
		statuses := def.Statuses
		if len(statuses) == 0 {
			statuses = []string{""}
		}
		reasons := def.Reasons
		if len(reasons) == 0 {
			reasons = []string{""}
		}
		tools := def.Tools
		if len(tools) == 0 {
			tools = []string{""}
		}
		for _, tool := range tools {
			for _, status := range statuses {
				for _, reason := range reasons {
					req := sampleRequest(def.Screen)
					req.Tool, req.Status, req.Reason = tool, status, reason
					requests = append(requests, req)
				}
			}
		}
	}
	return requests
}

func TestFilledDescriptionsLeaveNoPlaceholder(t *testing.T) {
	if len(allRequests()) < 30 {
		t.Fatalf("only %d requests", len(allRequests()))
	}
	for _, agent := range Agents() {
		for _, req := range allRequests() {
			got, err := Describe(req, Context{Agent: agent})
			if err != nil {
				t.Errorf("%+v: %v", req, err)
				continue
			}
			body, _ := json.Marshal(got)
			if strings.ContainsAny(string(body), "{}") && regexp.MustCompile(`\{(app|ai|tool|tool-link|link:[a-z]+)\}`).Match(body) {
				t.Errorf("%+v: placeholder left in %s", req, body)
			}
			if agent == OpenCode && strings.Contains(string(body), "Claude") {
				t.Errorf("%+v: Claude named under OpenCode: %s", req, body)
			}
		}
	}
}

func TestSpecificScreens(t *testing.T) {
	ctx := Context{Agent: ClaudeCode}
	tool, _ := Describe(Request{Screen: "tool", Tool: "jobs", Status: "downloading"}, ctx)
	if got := plain(tool.Title); got != "Adding Jobs" {
		t.Errorf("tool title = %q", got)
	}
	if tool.Title[1].URL != "https://github.com/bensyverson/jobs" {
		t.Errorf("tool name must link to its repo: %+v", tool.Title)
	}
	if got := plain(tool.Status); got != "Downloading Jobs" {
		t.Errorf("tool status = %q", got)
	}
	if tool.Note != "Lets Claude manage big projects." || tool.Card != "jobs" || tool.Mode != Working {
		t.Errorf("tool = %+v", tool)
	}

	ask, _ := Describe(Request{Screen: "apple-ask"}, ctx)
	if len(ask.Choices) != 0 || ask.Mode != Waiting {
		t.Errorf("apple-ask has no button and waits on the person: %+v", ask)
	}
	brew, _ := Describe(Request{Screen: "brew-ask"}, ctx)
	if got := plain(brew.Title); got != "Your turn: click through the installer." {
		t.Errorf("brew-ask title = %q", got)
	}
	if got := plain(brew.Status); got != "This adds Homebrew, which installs other software for you. It will ask for your Mac password." {
		t.Errorf("brew-ask line = %q", got)
	}
	if brew.Card != "homebrew" {
		t.Errorf("Homebrew has no other screen to show its card: %+v", brew)
	}
	if len(brew.Choices) != 0 || brew.Code != "" || brew.Mode != Waiting {
		t.Errorf("Apple blocks a pasted command, so there's nothing to paste: %+v", brew)
	}
	done, _ := Describe(Request{Screen: "done"}, ctx)
	if done.Mode != Settled {
		t.Errorf("done mode = %s", done.Mode)
	}
}

func TestTroubleOffersSkipOnlyWhenTheToolCanBeSkipped(t *testing.T) {
	ctx := Context{Agent: ClaudeCode}
	sleepy, err := Describe(Request{Screen: "trouble", Tool: "sleepy", Reason: "network"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := plain(sleepy.Title); got != "Sleepy didn’t install." {
		t.Errorf("title = %q", got)
	}
	if !slices.ContainsFunc(sleepy.Choices, func(c Choice) bool { return c.ID == "skip" && c.Label == "Skip Sleepy" }) {
		t.Errorf("sleepy can be skipped: %+v", sleepy.Choices)
	}
	jobs, err := Describe(Request{Screen: "trouble", Tool: "jobs", Reason: "network"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(jobs.Choices, func(c Choice) bool { return c.ID == "skip" }) {
		t.Errorf("Jobs is required, so nothing may offer to skip it: %+v", jobs.Choices)
	}
}

func TestListRowsFollowTheChoice(t *testing.T) {
	list, err := Describe(Request{Screen: "list"}, Context{Agent: ClaudeCode, Chosen: []string{"clt", "homebrew", "git", "agents", "jobs", "contextfill", "peep"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range list.Rows {
		want := row.Required || row.ID == "peep"
		if row.Checked != want {
			t.Errorf("%s checked = %v", row.ID, row.Checked)
		}
	}
}

func TestValidateRefusesOutsideTheClosedLists(t *testing.T) {
	cases := []struct {
		name  string
		req   Request
		valid string // must appear in Valid
	}{
		{"unknown screen", Request{Screen: "nope"}, "welcome"},
		{"oldMac is gone", Request{Screen: "oldMac"}, "welcome"},
		{"lost is the server's to show", Request{Screen: "lost"}, "welcome"},
		{"the end is the server's to show", Request{Screen: "end"}, "welcome"},
		{"tool needs a tool", Request{Screen: "tool"}, "jobs"},
		{"unknown tool", Request{Screen: "tool", Tool: "nope"}, "jobs"},
		{"homebrew has its own screens", Request{Screen: "tool", Tool: "homebrew"}, "jobs"},
		{"unknown status", Request{Screen: "tool", Tool: "jobs", Status: "nope"}, "installing"},
		{"a screen with no statuses", Request{Screen: "welcome", Status: "nope"}, ""},
		{"a screen that takes no tool", Request{Screen: "welcome", Tool: "jobs"}, ""},
		{"a screen that takes no reason", Request{Screen: "welcome", Reason: "network"}, ""},
		{"trouble needs a reason", Request{Screen: "trouble", Tool: "jobs"}, "network"},
		{"unknown reason", Request{Screen: "trouble", Tool: "jobs", Reason: "nope"}, "network"},
		{"trouble needs a tool", Request{Screen: "trouble", Reason: "network"}, "homebrew"},
		{"unknown building status", Request{Screen: "building", Status: "nope"}, "folder"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.req)
			var inv *InvalidError
			if !errors.As(err, &inv) {
				t.Fatalf("err = %v, want InvalidError", err)
			}
			if c.valid != "" && !slices.Contains(inv.Valid, c.valid) {
				t.Errorf("Valid = %v, want it to hold %s", inv.Valid, c.valid)
			}
		})
	}
}

func TestStatusDefaultsToTheFirst(t *testing.T) {
	got, err := Describe(Request{Screen: "building"}, Context{Agent: ClaudeCode})
	if err != nil {
		t.Fatal(err)
	}
	if plain(got.Status) != "Making your folder" {
		t.Errorf("status = %q", plain(got.Status))
	}
}

// Screens whose words are drafts Ben hasn't approved. Each is listed, so none can
// ship unnoticed, and the prototype check skips them. Approving a screen's words
// takes it off this list.
func TestDraftScreens(t *testing.T) {
	want := []ID{"git-ask", "github", "github-signup", "github-code", "identity", "identity-name", "identity-email", "settings"}
	if got := Drafts(); !slices.Equal(got, want) {
		t.Errorf("drafts = %v, want %v", got, want)
	}
	gitAsk := slices.IndexFunc(Definitions(), func(d Definition) bool { return d.Screen == "git-ask" })
	if gitAsk < 0 {
		t.Fatal("no git-ask screen")
	}
	// The setup never commits for the person, and the .gitignore needs no question.
	if got := Definitions()[gitAsk].Statuses; len(got) != 0 {
		t.Errorf("git-ask statuses = %v", got)
	}
}

func TestNoWordIsAPlaceholder(t *testing.T) {
	for _, w := range Copy() {
		if strings.Contains(w, "TODO") {
			t.Errorf("%q is a placeholder", w)
		}
	}
}
