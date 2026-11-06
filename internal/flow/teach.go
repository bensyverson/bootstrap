package flow

import (
	"fmt"
	"path/filepath"

	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/region"
	"github.com/bensyverson/bootstrap/internal/screen"
)

// With the helpers on, bootstrap tells every later session how to start a
// project, in the agent's instructions file for the whole Mac, between markers so
// a later setup updates it in place (Ben, 2026-11-06, S2lZZq). It overrides, for
// this one block, the 2026-11-05 ruling that bootstrap leaves an agent's settings
// to the tools' own hook install.

// teachRegion names the block in the agent's instructions file.
const teachRegion = "bootstrap"

// teaching is the block's words, approved by Ben on 2026-11-06, naming bootstrap as cmd.
func teaching(cmd string) string {
	return fmt.Sprintf("## New projects\n\n"+
		"When asked to set up or bootstrap a new project, ask what you should call it and then run `%[1]s new <name> --yes` and follow its `next:` lines. "+
		"This sets up the folder with Git and Agents using the user's defaults. "+
		"To set up the folder you're already in, run `%[1]s new --yes` with no name. "+
		"`%[1]s new --help` lists the options.\n", cmd)
}

// rulesPath is the agent's instructions file for the whole Mac.
func (r *run) rulesPath(agent screen.Agent) string {
	return filepath.Join(r.f.cfg.Probe.Home, agent.RulesFile())
}

// teach writes the block, naming bootstrap where the person's shell finds it.
func (r *run) teach(agent screen.Agent) error {
	cmd := "bootstrap"
	if detect.Bootstrap(r.f.cfg.Probe) != detect.HomebrewBootstrap {
		cmd = detect.DownloadedBootstrap
	}
	path := r.rulesPath(agent)
	if err := region.Write(path, teachRegion, teaching(cmd)); err != nil {
		return err
	}
	return r.note("told %s how to start a project, in %s", agent.AI, Place(r.f.cfg.Probe.Home, path))
}

// taught reports whether the agent's instructions say how to start a project.
func (r *run) taught(agent screen.Agent) bool {
	return region.Has(r.rulesPath(agent), teachRegion)
}
