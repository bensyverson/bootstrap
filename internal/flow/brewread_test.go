package flow_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bensyverson/bootstrap/internal/flow"
	"github.com/bensyverson/bootstrap/internal/session"
)

// openTimer notes how long brew-ask had been on the page when Homebrew's .pkg opened.
type openTimer struct {
	flow.Runner
	store session.Store
	shown time.Duration
}

func (o *openTimer) Run(ctx context.Context, cmd flow.Command) flow.Result {
	if strings.HasSuffix(cmd.Path, "/open") && len(cmd.Args) == 1 && strings.HasSuffix(cmd.Args[0], ".pkg") {
		if state, err := o.store.Load(); err == nil && state.Current != nil && state.Current.Request.Screen == "brew-ask" {
			o.shown = time.Since(state.Current.At)
		}
	}
	return o.Runner.Run(ctx, cmd)
}

// Ben, 2026-11-06, third clean-Mac pass: Homebrew's installer covered brew-ask
// the instant it showed, so the person never read it. The screen stays in view
// for Limits.BrewRead first.
func TestBrewAskStaysInViewBeforeTheInstallerOpens(t *testing.T) {
	w := newWorld(t)
	cfg := w.config()
	cfg.Limits.BrewRead = 150 * time.Millisecond
	timer := &openTimer{Runner: w.mac, store: w.store}
	cfg.Runner = timer
	if err := w.walkWith(cfg); err != nil {
		t.Fatal(err)
	}
	if timer.shown < cfg.Limits.BrewRead {
		t.Errorf("Homebrew's installer opened %v after brew-ask showed, want at least %v", timer.shown, cfg.Limits.BrewRead)
	}
}

func TestTheDefaultReadingPauseIsTwoSeconds(t *testing.T) {
	if got := flow.DefaultLimits().BrewRead; got != 2*time.Second {
		t.Errorf("BrewRead = %v, want 2s", got)
	}
}
