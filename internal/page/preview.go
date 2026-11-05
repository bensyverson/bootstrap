package page

import (
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Preview is one screen in one state, for review and still tests.
type Preview struct {
	Name  string        `json:"name"`
	Frame session.Frame `json:"frame"`
}

// Previews lists every screen in each state: the object alone, each checked-in
// sample, and the states one sample per screen can't hold, built the same way the
// server builds a real frame. Nothing here is a second copy of any screen's words.
func Previews() []Preview {
	previews := []Preview{{Name: "object"}}
	samples, err := fs.ReadDir(screen.Samples, "samples")
	if err != nil {
		// The samples are embedded at build time, and this path is a constant.
		panic(err)
	}
	for _, f := range samples {
		body, err := fs.ReadFile(screen.Samples, "samples/"+f.Name())
		if err != nil {
			panic(err)
		}
		var d screen.Description
		if err := json.Unmarshal(body, &d); err != nil {
			panic(err)
		}
		previews = append(previews, Preview{Name: strings.TrimSuffix(f.Name(), ".json"), Frame: frameOf(d)})
	}
	for _, agent := range screen.Agents() {
		previews = append(previews, statesFor(agent)...)
	}
	return previews
}

func frameOf(d screen.Description) session.Frame {
	return session.Frame{Screen: &d, Lost: d.Screen == screen.Lost, Ended: d.Screen == screen.End}
}

func statesFor(agent screen.Agent) []Preview {
	ctx := screen.Context{Agent: agent, Carried: 0.5}
	var out []Preview
	add := func(detail string, req screen.Request, answered bool) {
		d, err := screen.Describe(req, ctx)
		if err != nil {
			// Every request here is a constant from the closed lists, and a test draws them all.
			panic(err)
		}
		frame := frameOf(d)
		frame.Answered = answered
		out = append(out, Preview{Name: string(req.Screen) + "." + detail + "." + agent.ID, Frame: frame})
	}
	for _, def := range screen.Definitions() {
		switch {
		case len(def.Tools) > 0 && len(def.Statuses) > 0:
			for _, tool := range def.Tools {
				for _, status := range def.Statuses {
					add(tool+"."+status, screen.Request{Screen: def.Screen, Tool: tool, Status: status}, false)
				}
			}
		case len(def.Statuses) > 0:
			for _, status := range def.Statuses {
				add(status, screen.Request{Screen: def.Screen, Status: status}, false)
			}
		}
	}
	add("other", screen.Request{Screen: "trouble", Tool: "sleepy", Reason: "other"}, false)
	add("required", screen.Request{Screen: "trouble", Tool: "homebrew", Reason: "network"}, false)
	for _, id := range []screen.ID{"welcome", "list", "making", "naming"} {
		add("answered", screen.Request{Screen: id}, true)
	}
	return out
}
