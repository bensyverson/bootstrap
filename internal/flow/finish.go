package flow

import (
	"fmt"
	"slices"

	"github.com/bensyverson/bootstrap/internal/screen"
	"github.com/bensyverson/bootstrap/internal/session"
)

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
	// A skip stands unless the person took the offer up front this time.
	if err != nil || status != session.StepSkipped || !r.f.accepted(OfferSettings) {
		return status, nil, err
	}
	return "", nil, nil
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
	state, err := r.state()
	if err != nil {
		return "", nil, err
	}
	// With no way to reach the phone, there is nothing to ask or tell.
	if state.Agent.RemoteControl() == "" {
		return session.StepDone, nil, nil
	}
	return state.Finished[r.def.ID], nil, nil
}

func actPhone(r *run) (outcome, error) {
	if r.f.cfg.Person != nil {
		state, err := r.state()
		if err != nil {
			return outcome{}, err
		}
		if err := r.note("%s", phoneHint(state.Agent)); err != nil {
			return outcome{}, err
		}
		return done()
	}
	a, ask, err := r.asked(phone)
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
	what := fmt.Sprintf("the person wants to use their agent from their phone: explain %s in the chat", state.Agent.RemoteControl())
	if err := r.tell(session.Handoff{Kind: session.HandoffPhone, Step: StepPhone, What: what, Ways: []session.Way{session.WayTalk}}); err != nil {
		return outcome{}, err
	}
	return done()
}

func checkDone(r *run) (session.StepStatus, *session.Handoff, error) { return "", nil, nil }

func actDone(r *run) (outcome, error) {
	tools, err := r.justTools()
	if err != nil {
		return outcome{}, err
	}
	req := screen.Request{Screen: "done"}
	if tools {
		req.Status = screen.DoneJustTools
	}
	if err := r.show(req); err != nil {
		return outcome{}, err
	}
	state, err := r.state()
	if err != nil {
		return outcome{}, err
	}
	if slices.ContainsFunc(state.Handoffs, func(l session.Logged) bool { return l.Handoff.Kind == session.HandoffDone }) {
		return done()
	}
	var what string
	if tools {
		what = fmt.Sprintf("the setup is done, with just the tools and no project: sum it up in the chat, and tell the person how to make a project later, as the page says: ask their agent to run bootstrap new, or run %s (or paste the setup link into their agent again)", r.personCommand("new <name>"))
	} else {
		dir, err := r.mustProject()
		if err != nil {
			return outcome{}, err
		}
		what = fmt.Sprintf("the setup is done: sum it up in the chat, and tell the person to open a new Terminal window, cd %s, and start their agent there", r.tilde(dir))
	}
	if err := r.tell(session.Handoff{Kind: session.HandoffDone, Step: StepDone, What: what}); err != nil {
		return outcome{}, err
	}
	return done()
}
