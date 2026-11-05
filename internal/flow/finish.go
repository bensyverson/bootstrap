package flow

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

func checkCommit(r *run) (session.StepStatus, *session.Handoff, error) {
	dir, err := r.mustProject()
	if err != nil {
		return "", nil, err
	}
	if !exists(filepath.Join(dir, ".git")) {
		return session.StepSkipped, nil, nil
	}
	git, h := r.git()
	if h != nil {
		return "", h, nil
	}
	res, h, err := r.exec(Command{Kind: Inspect, Path: git, Args: []string{"rev-parse", "--verify", "--quiet", "HEAD"}, Dir: dir}, "", 0, 1)
	if err != nil || h != nil {
		return "", h, err
	}
	if res.Exit == 0 {
		return session.StepDone, nil, nil
	}
	if no, err := r.declined(askCommit); no || err != nil {
		return session.StepSkipped, nil, err
	}
	return "", nil, nil
}

func actCommit(r *run) (outcome, error) {
	dir, err := r.mustProject()
	if err != nil {
		return outcome{}, err
	}
	a, ask, err := r.asked(askCommit)
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
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-m", firstCommit}} {
		if _, h, err := r.exec(Command{Kind: Change, Path: git, Args: args, Dir: dir}, ""); err != nil || h != nil {
			return orHandOff(h, err)
		}
	}
	if err := r.note("saved the first version with Git"); err != nil {
		return outcome{}, err
	}
	return done()
}

// finished is how this step ended earlier in the session, for steps that leave nothing on the Mac to check.
func (r *run) finished() (session.StepStatus, error) {
	state, err := r.state()
	return state.Finished[r.def.ID], err
}

func checkSettings(r *run) (session.StepStatus, *session.Handoff, error) {
	// Each tool's own hook install is safe to repeat, but has no check of its own
	// yet (GGbVZ5), so the session's log is the record.
	if len(r.f.cfg.Hooks) == 0 {
		return session.StepDone, nil, nil
	}
	status, err := r.finished()
	return status, nil, err
}

func actSettings(r *run) (outcome, error) {
	a, ask, err := r.asked(settings)
	if err != nil || ask != nil {
		return deref(ask, err)
	}
	if a.Choice == notNow {
		return skipped()
	}
	state, err := r.state()
	if err != nil {
		return outcome{}, err
	}
	for _, id := range r.f.cfg.Hooks {
		path, h := r.toolPath(id)
		if h != nil {
			return handOff(*h)
		}
		if _, h, err := r.exec(Command{Kind: Change, Path: path, Args: []string{"hook", "install", "--harness", state.Agent.ID}}, id); err != nil || h != nil {
			return orHandOff(h, err)
		}
		if err := r.note("installed %s's hook", id); err != nil {
			return outcome{}, err
		}
	}
	return done()
}

func checkPhone(r *run) (session.StepStatus, *session.Handoff, error) {
	status, err := r.finished()
	return status, nil, err
}

func actPhone(r *run) (outcome, error) {
	a, ask, err := r.asked(phone)
	if err != nil || ask != nil {
		return deref(ask, err)
	}
	if a.Choice == notNow {
		return skipped()
	}
	if err := r.tell(session.Handoff{Kind: session.HandoffPhone, Step: StepPhone, What: "the person wants to use their agent from their phone: explain /remote-control in the chat", Ways: []session.Way{session.WayTalk}}); err != nil {
		return outcome{}, err
	}
	return done()
}

func checkDone(r *run) (session.StepStatus, *session.Handoff, error) { return "", nil, nil }

func actDone(r *run) (outcome, error) {
	if err := r.show(screen.Request{Screen: "done"}); err != nil {
		return outcome{}, err
	}
	state, err := r.state()
	if err != nil {
		return outcome{}, err
	}
	if slices.ContainsFunc(state.Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffDone }) {
		return done()
	}
	dir, err := r.mustProject()
	if err != nil {
		return outcome{}, err
	}
	what := fmt.Sprintf("the setup is done: sum it up in the chat, and tell the person to open a new Terminal window, cd %s, and start their agent there", r.tilde(dir))
	if err := r.tell(session.Handoff{Kind: session.HandoffDone, Step: StepDone, What: what}); err != nil {
		return outcome{}, err
	}
	return done()
}
