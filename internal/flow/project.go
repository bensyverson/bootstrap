package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

const (
	// rulesSource is what agents init renders the rules from.
	rulesSource = "github.com/bensyverson/agents-md"
	// xcodeInAppStore opens Xcode's page in the App Store.
	xcodeInAppStore = "macappstore://apps.apple.com/app/xcode/id497799835"
	firstCommit     = "Sets up the project"
	// starterIgnore is a new project's .gitignore, before local/ and job's lines join it.
	starterIgnore = ".DS_Store\n"
	localDir      = "local"
)

// jobIdentity is the name job init records for each app's agent (project/2026-11-05-init-existing-project-findings.md).
var jobIdentity = map[string]string{screen.ClaudeCode.ID: "claude", screen.OpenCode.ID: "opencode"}

var (
	naming    = screen.Request{Screen: "naming"}
	askRepo   = screen.Request{Screen: "git-ask", Status: "repo"}
	askIgnore = screen.Request{Screen: "git-ask", Status: "ignore"}
	askCommit = screen.Request{Screen: "git-ask", Status: "commit"}
	askGitHub = screen.Request{Screen: "github"}
	making    = screen.Request{Screen: "making"}
	xcode     = screen.Request{Screen: "xcode"}
	liveYet   = screen.Request{Screen: "live-yet"}
	settings  = screen.Request{Screen: "settings"}
	phone     = screen.Request{Screen: "phone"}
)

// Answers the flow reads by their choice id.
const (
	yes      = "yes"
	notNow   = "not-now"
	appStore = "app-store"
	anApp    = "app"
)

var errNoProject = errors.New("the project steps ran before the folder step")

// project is where the project steps run: the starting folder when it's already a
// project, and otherwise ~/Projects/<name>. Nothing in the starting folder moves.
func (r *run) project() (string, bool, error) {
	if r.f.inPlace {
		return r.f.cfg.Probe.Cwd, true, nil
	}
	a, ok, err := r.answer(naming)
	if err != nil || !ok {
		return "", false, err
	}
	return filepath.Join(r.f.cfg.Probe.Home, "Projects", a.Name), true, nil
}

// mustProject is the project folder for the steps after the folder step.
func (r *run) mustProject() (string, error) {
	dir, ok, err := r.project()
	if err == nil && !ok {
		err = errNoProject
	}
	return dir, err
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// declined reports whether the person said not now to a question.
func (r *run) declined(req screen.Request) (bool, error) {
	a, ok, err := r.answer(req)
	return ok && a.Choice == notNow, err
}

// asked returns the answer, or the outcome that asks for it.
func (r *run) asked(req screen.Request) (screen.Answer, *outcome, error) {
	a, ok, err := r.answer(req)
	if err != nil || ok {
		return a, nil, err
	}
	out, _ := needs(req)
	return a, &out, nil
}

func checkFolder(r *run) (session.StepStatus, *session.Handoff, error) {
	dir, ok, err := r.project()
	if err != nil || !ok || !isDir(dir) {
		return "", nil, err
	}
	return session.StepDone, nil, nil
}

func actFolder(r *run) (outcome, error) {
	if _, ask, err := r.asked(naming); err != nil || ask != nil {
		return deref(ask, err)
	}
	dir, err := r.mustProject()
	if err != nil {
		return outcome{}, err
	}
	if exists(dir) && !isDir(dir) {
		return handOff(r.trouble(session.ReasonInTheWay, "", r.tilde(dir)+" is a file, not a folder"))
	}
	if err := r.show(screen.Request{Screen: "building", Status: "folder"}); err != nil {
		return outcome{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return handOff(r.trouble(session.ReasonInTheWay, "", fmt.Sprintf("couldn't make %s: %v", r.tilde(dir), err)))
	}
	if err := r.note("made %s", r.tilde(dir)); err != nil {
		return outcome{}, err
	}
	return done()
}

func deref(out *outcome, err error) (outcome, error) {
	if err != nil {
		return outcome{}, err
	}
	return *out, nil
}

func (r *run) git() (string, *session.Handoff) {
	if path, ok := r.located("git"); ok {
		return path, nil
	}
	h := r.trouble(session.ReasonMissing, "", "Git isn't on this Mac, though Apple's tools are")
	return "", &h
}

func checkGit(r *run) (session.StepStatus, *session.Handoff, error) {
	dir, err := r.mustProject()
	if err != nil {
		return "", nil, err
	}
	if !exists(filepath.Join(dir, ".git")) {
		no, err := r.declined(askRepo)
		if no {
			return session.StepSkipped, nil, err
		}
		return "", nil, err
	}
	no, err := r.declined(askIgnore)
	if err != nil || (!no && !exists(filepath.Join(dir, ".gitignore"))) {
		return "", nil, err
	}
	return session.StepDone, nil, nil
}

func actGit(r *run) (outcome, error) {
	dir, err := r.mustProject()
	if err != nil {
		return outcome{}, err
	}
	if !exists(filepath.Join(dir, ".git")) {
		a, ask, err := r.asked(askRepo)
		if err != nil || ask != nil {
			return deref(ask, err)
		}
		if a.Choice == notNow {
			return skipped()
		}
		git, h := r.git()
		if h != nil {
			return handOff(*h)
		}
		if err := r.show(screen.Request{Screen: "building", Status: "git"}); err != nil {
			return outcome{}, err
		}
		if _, h, err := r.exec(Command{Kind: Change, Path: git, Args: []string{"init"}, Dir: dir}, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if err := r.note("turned on Git in %s", r.tilde(dir)); err != nil {
			return outcome{}, err
		}
	}
	ignore := filepath.Join(dir, ".gitignore")
	if !exists(ignore) {
		a, ask, err := r.asked(askIgnore)
		if err != nil || ask != nil {
			return deref(ask, err)
		}
		if a.Choice == yes {
			if err := os.WriteFile(ignore, []byte(starterIgnore), 0o644); err != nil {
				return outcome{}, err
			}
			if err := r.note("added a .gitignore"); err != nil {
				return outcome{}, err
			}
		}
	}
	return done()
}

func checkGitHub(r *run) (session.StepStatus, *session.Handoff, error) {
	if _, ok := r.locate(ghPath); ok {
		res, h, err := r.exec(Command{Kind: Inspect, Path: ghPath, Args: []string{"auth", "status"}}, "", 0, 1)
		if err != nil || h != nil {
			return "", h, err
		}
		if res.Exit == 0 {
			return session.StepDone, nil, nil
		}
	}
	no, err := r.declined(askGitHub)
	if no {
		return session.StepSkipped, nil, err
	}
	return "", nil, err
}

func actGitHub(r *run) (outcome, error) {
	a, ask, err := r.asked(askGitHub)
	if err != nil || ask != nil {
		return deref(ask, err)
	}
	if a.Choice == notNow {
		return skipped()
	}
	if _, ok := r.locate(ghPath); !ok {
		install := Command{Kind: Change, Path: brewPath, Args: []string{"install", "gh"}}
		if _, h, err := r.exec(install, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if _, ok := r.locate(ghPath); !ok {
			return handOff(r.trouble(session.ReasonMissing, "", fmt.Sprintf("%s exited 0, but %s isn't there", install, ghPath)))
		}
	}
	login := Command{Kind: Change, Path: ghPath, Args: []string{"auth", "login", "--hostname", "github.com", "--web", "--git-protocol", "https", "--skip-ssh-key"}, Timeout: r.f.cfg.Limits.Login}
	if _, h, err := r.exec(login, ""); err != nil || h != nil {
		return orHandOff(h, err)
	}
	// gh's own exit isn't proof: the design confirms the sign-in took.
	if _, h, err := r.exec(Command{Kind: Inspect, Path: ghPath, Args: []string{"auth", "status"}}, ""); err != nil || h != nil {
		return orHandOff(h, err)
	}
	if err := r.note("signed in to GitHub"); err != nil {
		return outcome{}, err
	}
	return done()
}

func checkStage(r *run) (session.StepStatus, *session.Handoff, error) {
	for _, req := range []screen.Request{making, liveYet} {
		if _, ok, err := r.answer(req); err != nil || !ok {
			return "", nil, err
		}
	}
	what, _, err := r.answer(making)
	if err != nil {
		return "", nil, err
	}
	if what.Choice == anApp {
		if _, ok, err := r.answer(xcode); err != nil || !ok {
			return "", nil, err
		}
	}
	return session.StepDone, nil, nil
}

func actStage(r *run) (outcome, error) {
	what, ask, err := r.asked(making)
	if err != nil || ask != nil {
		return deref(ask, err)
	}
	var store screen.Answer
	if what.Choice == anApp {
		if store, ask, err = r.asked(xcode); err != nil || ask != nil {
			return deref(ask, err)
		}
	}
	if _, ask, err := r.asked(liveYet); err != nil || ask != nil {
		return deref(ask, err)
	}
	if store.Choice == appStore {
		if _, h, err := r.exec(Command{Kind: Change, Path: openPath, Args: []string{xcodeInAppStore}}, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if err := r.note("opened Xcode in the App Store"); err != nil {
			return outcome{}, err
		}
	}
	return done()
}

func hasLine(path, line string) bool {
	body, err := os.ReadFile(path)
	return err == nil && slices.Contains(strings.Split(string(body), "\n"), line)
}

func checkRules(r *run) (session.StepStatus, *session.Handoff, error) {
	dir, err := r.mustProject()
	if err != nil {
		return "", nil, err
	}
	if exists(filepath.Join(dir, "AGENTS.md")) && isDir(filepath.Join(dir, localDir)) &&
		hasLine(filepath.Join(dir, ".gitignore"), localDir+"/") && exists(filepath.Join(dir, ".jobs.db")) {
		return session.StepDone, nil, nil
	}
	return "", nil, nil
}

// toolPath is a required tool's full path, or a handoff when it isn't there.
func (r *run) toolPath(id string) (string, *session.Handoff) {
	if path, ok := r.located(id); ok {
		return path, nil
	}
	h := r.trouble(session.ReasonMissing, "", id+" isn't on this Mac, though the tools step finished")
	return "", &h
}

func actRules(r *run) (outcome, error) {
	dir, err := r.mustProject()
	if err != nil {
		return outcome{}, err
	}
	if err := r.show(screen.Request{Screen: "building", Status: "rules"}); err != nil {
		return outcome{}, err
	}
	if !exists(filepath.Join(dir, "AGENTS.md")) {
		agents, h := r.toolPath("agents")
		if h != nil {
			return handOff(*h)
		}
		initRules := Command{Kind: Change, Path: agents, Args: []string{"init", rulesSource}, Dir: dir}
		res, h, err := r.exec(initRules, "", 0, 1)
		if err != nil || h != nil {
			return orHandOff(h, err)
		}
		if res.Exit == 1 {
			// agents init refuses a project's own CLAUDE.md and names migrate, which keeps its words.
			if !strings.Contains(string(res.Output), "agents migrate") {
				_, h, err := r.exec(initRules, "")
				return orHandOff(h, err)
			}
			if _, h, err := r.exec(Command{Kind: Change, Path: agents, Args: []string{"migrate"}, Dir: dir}, ""); err != nil || h != nil {
				return orHandOff(h, err)
			}
			if _, h, err := r.exec(initRules, ""); err != nil || h != nil {
				return orHandOff(h, err)
			}
		}
		if err := r.note("added the rules with agents init"); err != nil {
			return outcome{}, err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, localDir), 0o755); err != nil {
		return outcome{}, err
	}
	if err := addLine(filepath.Join(dir, ".gitignore"), localDir+"/"); err != nil {
		return outcome{}, err
	}
	if !exists(filepath.Join(dir, ".jobs.db")) {
		job, h := r.toolPath("jobs")
		if h != nil {
			return handOff(*h)
		}
		state, err := r.state()
		if err != nil {
			return outcome{}, err
		}
		// job init refuses an existing .jobs.db, and --force would wipe its tasks, so it only runs without one.
		if _, h, err := r.exec(Command{Kind: Change, Path: job, Args: []string{"init", "--as", jobIdentity[state.Agent.ID]}, Dir: dir}, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if _, h, err := r.exec(Command{Kind: Change, Path: job, Args: []string{"gitignore"}, Dir: dir}, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if err := r.note("started Jobs in %s", r.tilde(dir)); err != nil {
			return outcome{}, err
		}
	}
	return done()
}

// addLine adds a line to a file unless it's there, keeping what's there.
func addLine(path, line string) error {
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if slices.Contains(strings.Split(string(body), "\n"), line) {
		return nil
	}
	if len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		body = append(body, '\n')
	}
	return os.WriteFile(path, append(body, line+"\n"...), 0o644)
}
