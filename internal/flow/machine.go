package flow

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bensyverson/bootstrap/catalog"
	"github.com/bensyverson/bootstrap/internal/detect"
	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

// Outside programs, by full path: a session that was running before Homebrew
// installed can't find anything by name (project/2026-11-05-vm-run-findings.md).
const (
	xcodeSelectPath = "/usr/bin/xcode-select"
	curlPath        = "/usr/bin/curl"
	pkgutilPath     = "/usr/sbin/pkgutil"
	openPath        = "/usr/bin/open"
	brewPath        = "/opt/homebrew/bin/brew"
	ghPath          = "/opt/homebrew/bin/gh"
)

const (
	tap = "bensyverson/tap"
	// homebrewTeam is the Developer ID that signs Homebrew's .pkg.
	homebrewTeam = "927JGANW46"
	homebrewPkg  = "Homebrew.pkg"
)

var steps = []stepDef{
	{ID: StepPlan, Screens: []screen.ID{"welcome", "list", "fast"}, Asks: []screen.ID{"welcome", "list"}, check: checkPlan, act: actPlan},
	{ID: StepApple, Screens: []screen.ID{"apple-ask", "apple-wait"}, check: checkApple, act: actApple},
	{ID: StepHomebrew, Screens: []screen.ID{"brew-ask"}, check: checkHomebrew, act: actHomebrew},
	{ID: StepTools, Screens: []screen.ID{"tool"}, check: checkTools, act: actTools},
	{ID: StepFolder, Screens: []screen.ID{"naming", "building"}, Asks: []screen.ID{"naming"}, check: checkFolder, act: actFolder},
	{ID: StepGit, Screens: []screen.ID{"git-ask", "building"}, Asks: []screen.ID{"git-ask"}, Skippable: true, check: checkGit, act: actGit},
	{ID: StepGitHub, Screens: []screen.ID{"github"}, Asks: []screen.ID{"github"}, Skippable: true, check: checkGitHub, act: actGitHub},
	{ID: StepStage, Screens: []screen.ID{"making", "xcode", "live-yet"}, Asks: []screen.ID{"making", "xcode", "live-yet"}, check: checkStage, act: actStage},
	{ID: StepRules, Screens: []screen.ID{"building"}, check: checkRules, act: actRules},
	{ID: StepCommit, Screens: []screen.ID{"git-ask"}, Asks: []screen.ID{"git-ask"}, Skippable: true, check: checkCommit, act: actCommit},
	{ID: StepSettings, Screens: []screen.ID{"settings"}, Asks: []screen.ID{"settings"}, Skippable: true, check: checkSettings, act: actSettings},
	{ID: StepPhone, Screens: []screen.ID{"phone"}, Asks: []screen.ID{"phone"}, Skippable: true, check: checkPhone, act: actPhone},
	{ID: StepDone, Screens: []screen.ID{"done"}, check: checkDone, act: actDone},
}

var (
	welcome = screen.Request{Screen: "welcome"}
	list    = screen.Request{Screen: "list"}
)

func (r *run) locate(paths ...string) (string, bool) { return detect.Locate(r.f.cfg.Probe, paths) }

func (r *run) located(id string) (string, bool) {
	tool, _ := catalog.Find(id)
	return r.locate(tool.Detect...)
}

func checkPlan(r *run) (session.StepStatus, *session.Handoff, error) {
	a, ok, err := r.answer(welcome)
	if err != nil || !ok {
		return "", nil, err
	}
	if a.Choice == "skip-tour" {
		return session.StepDone, nil, nil
	}
	_, ok, err = r.answer(list)
	if err != nil || !ok {
		return "", nil, err
	}
	return session.StepDone, nil, nil
}

func actPlan(r *run) (outcome, error) {
	a, ok, err := r.answer(welcome)
	if err != nil {
		return outcome{}, err
	}
	if !ok {
		return needs(welcome)
	}
	if a.Choice == "see-plan" {
		if _, ok, err := r.answer(list); err != nil || !ok {
			if err != nil {
				return outcome{}, err
			}
			return needs(list)
		}
	}
	if err := r.show(screen.Request{Screen: "fast"}); err != nil {
		return outcome{}, err
	}
	return done()
}

func checkApple(r *run) (session.StepStatus, *session.Handoff, error) {
	if _, ok := r.located("clt"); ok {
		return session.StepDone, nil, nil
	}
	return "", nil, nil
}

// watch waits for an installer, then says what happened as a handoff, or nil once the tool arrived.
func (r *run) watch(id, name string) (session.Progress, *session.Handoff, error) {
	limit := r.f.cfg.Limits.Apple
	if id == "homebrew" {
		limit = r.f.cfg.Limits.Homebrew
	}
	got, err := session.WatchInstall(r.ctx, r.f.cfg.Probe, id, limit)
	switch {
	case errors.Is(err, session.ErrWaitTimeout):
		h := r.limitPassed(id, fmt.Sprintf("%s didn't arrive within %v", name, limit), limit)
		return got, &h, nil
	case err != nil:
		return got, nil, err
	case got.Event == session.Canceled:
		h := r.trouble(session.ReasonCanceled, id, fmt.Sprintf("the installer quit before %s arrived", name))
		return got, &h, nil
	}
	return got, nil, nil
}

func actApple(r *run) (outcome, error) {
	if disk := detect.Run(r.f.cfg.Probe).Disk; disk.AppleStep == detect.SpaceShort {
		h := r.trouble(session.ReasonDiskShort, "clt", fmt.Sprintf("only %.1f GB is free, and Apple's tools need %.0f GB", float64(disk.Free)/1e9, float64(detect.AppleStepNeeds)/1e9))
		return handOff(h)
	}
	if err := r.show(screen.Request{Screen: "apple-ask"}); err != nil {
		return outcome{}, err
	}
	if _, h, err := r.exec(Command{Kind: Change, Path: xcodeSelectPath, Args: []string{"--install"}}, "clt"); err != nil || h != nil {
		return orHandOff(h, err)
	}
	if err := r.note("installing Apple's tools: the person is clicking through Apple's window"); err != nil {
		return outcome{}, err
	}
	for {
		got, h, err := r.watch("clt", "Apple's tools")
		if err != nil || h != nil {
			return orHandOff(h, err)
		}
		if got.Event == session.Arrived {
			if err := r.note("installed Apple's tools"); err != nil {
				return outcome{}, err
			}
			return done()
		}
		if err := r.note("the person agreed to Apple's install, which is downloading"); err != nil {
			return outcome{}, err
		}
		if err := r.show(screen.Request{Screen: "apple-wait"}); err != nil {
			return outcome{}, err
		}
	}
}

// orHandOff turns a command's handoff or error into the step's outcome.
func orHandOff(h *session.Handoff, err error) (outcome, error) {
	if err != nil {
		return outcome{}, err
	}
	return handOff(*h)
}

func checkHomebrew(r *run) (session.StepStatus, *session.Handoff, error) {
	if _, ok := r.located("homebrew"); ok {
		return session.StepDone, nil, nil
	}
	return "", nil, nil
}

func actHomebrew(r *run) (outcome, error) {
	pkg := filepath.Join(r.f.logs(), homebrewPkg)
	if _, h, err := r.exec(Command{Kind: Change, Path: curlPath, Args: []string{"-fsSL", "-o", pkg, r.f.cfg.HomebrewPkg}}, "homebrew"); err != nil || h != nil {
		return orHandOff(h, err)
	}
	// Checked by signer, not a pinned checksum, because the version moves. spctl
	// alone isn't trusted: a VM rejected a good package.
	res, h, err := r.exec(Command{Kind: Inspect, Path: pkgutilPath, Args: []string{"--check-signature", pkg}}, "homebrew")
	if err != nil || h != nil {
		return orHandOff(h, err)
	}
	if !strings.Contains(string(res.Output), "("+homebrewTeam+")") {
		h := r.trouble(session.ReasonUnsigned, "homebrew", fmt.Sprintf("%s isn't signed by Homebrew (%s)", homebrewPkg, homebrewTeam))
		h.Evidence = session.Evidence{Command: Command{Path: pkgutilPath, Args: []string{"--check-signature", pkg}}.String(), Tail: tail(res.Output)}
		return handOff(h)
	}
	if err := r.show(screen.Request{Screen: "brew-ask"}); err != nil {
		return outcome{}, err
	}
	if _, h, err := r.exec(Command{Kind: Change, Path: openPath, Args: []string{pkg}}, "homebrew"); err != nil || h != nil {
		return orHandOff(h, err)
	}
	if err := r.note("installing Homebrew: the person is clicking through Homebrew's installer"); err != nil {
		return outcome{}, err
	}
	if _, h, err := r.watch("homebrew", "Homebrew"); err != nil || h != nil {
		return orHandOff(h, err)
	}
	if err := r.note("installed Homebrew"); err != nil {
		return outcome{}, err
	}
	return done()
}

// picked is the tools the person kept that come from the tap, in list order.
func (r *run) picked() ([]catalog.Tool, error) {
	state, err := r.state()
	if err != nil {
		return nil, err
	}
	chosen := state.Chosen
	if chosen == nil {
		chosen = screen.KeepRequired(screen.AllToolIDs())
	}
	var out []catalog.Tool
	for _, id := range chosen {
		if tool, ok := catalog.Find(id); ok && !tool.OwnStep {
			out = append(out, tool)
		}
	}
	return out, nil
}

func checkTools(r *run) (session.StepStatus, *session.Handoff, error) {
	tools, err := r.picked()
	if err != nil {
		return "", nil, err
	}
	for _, tool := range tools {
		if _, ok := r.locate(tool.Detect...); !ok {
			return "", nil, nil
		}
	}
	return session.StepDone, nil, nil
}

var cellarVersion = regexp.MustCompile(`Cellar/[^/\s]+/([^/:\s]+)`)

func actTools(r *run) (outcome, error) {
	tools, err := r.picked()
	if err != nil {
		return outcome{}, err
	}
	trusted := false
	for _, tool := range tools {
		if _, ok := r.locate(tool.Detect...); ok {
			continue
		}
		if !trusted {
			// A new account refuses the tap until it's trusted (project/2026-11-05-vm-run-findings.md).
			if _, h, err := r.exec(Command{Kind: Change, Path: brewPath, Args: []string{"trust", tap}}, ""); err != nil || h != nil {
				return orHandOff(h, err)
			}
			trusted = true
		}
		if err := r.show(screen.Request{Screen: "tool", Tool: tool.ID, Status: "installing"}); err != nil {
			return outcome{}, err
		}
		install := Command{Kind: Change, Path: brewPath, Args: []string{"install", tap + "/" + tool.Formula}}
		res, h, err := r.exec(install, tool.ID)
		if err != nil || h != nil {
			return orHandOff(h, err)
		}
		if _, ok := r.locate(tool.Detect...); !ok {
			h := r.trouble(session.ReasonMissing, tool.ID, fmt.Sprintf("%s exited 0, but %s isn't there", install, tool.Detect[0]))
			h.Evidence = session.Evidence{Command: install.String(), Log: filepath.Join(r.f.logs(), tool.ID+".log"), Tail: tail(res.Output)}
			return handOff(h)
		}
		line := "installed " + tool.ID
		if m := cellarVersion.FindSubmatch(res.Output); m != nil {
			line += " " + string(m[1])
		}
		if err := r.note("%s", line); err != nil {
			return outcome{}, err
		}
	}
	return done()
}
