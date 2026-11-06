package flow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bensyverson/bootstrap/internal/detect"
)

// selfFormula is bootstrap's own formula in the tap.
const selfFormula = "bootstrap"

// moveOntoHomebrew puts bootstrap itself into Homebrew, so brew upgrade keeps it
// current, and deletes the copy SETUP.md downloaded (Ben, 2026-11-06). It has no
// card and never hands off: until it works, the downloaded copy runs the setup
// as before, and its log says why.
func (r *run) moveOntoHomebrew(trust func() error) error {
	verb, note := "upgrade", "updated"
	if _, ok := r.locate(detect.HomebrewBootstrap); !ok {
		verb, note = "install", "installed"
		if err := trust(); err != nil {
			return err
		}
	}
	cmd := Command{Kind: Change, Path: brewPath, Args: []string{verb, tap + "/" + selfFormula}}
	res, h, err := r.exec(cmd, selfFormula)
	if err != nil {
		return err
	}
	if _, ok := r.locate(detect.HomebrewBootstrap); !ok {
		return r.note("kept %s: %s exited %d", detect.DownloadedBootstrap, cmd, res.Exit)
	}
	if h != nil {
		if err := r.note("couldn't %s %s: %s exited %d", verb, selfFormula, cmd, res.Exit); err != nil {
			return err
		}
	} else if m := cellarVersion.FindSubmatch(res.Output); m != nil {
		if err := r.note("%s %s %s", note, selfFormula, m[1]); err != nil {
			return err
		}
	}
	downloaded := filepath.Join(r.f.cfg.Probe.Home, strings.TrimPrefix(detect.DownloadedBootstrap, "~/"))
	switch err := os.Remove(downloaded); {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return r.note("couldn't delete %s: %v", detect.DownloadedBootstrap, err)
	}
	return r.note("deleted %s; %s runs from now on", detect.DownloadedBootstrap, detect.HomebrewBootstrap)
}

// personCommand is a bootstrap command as the person should type it later in
// Terminal. Homebrew's installer puts its bin on every new shell's PATH
// (/etc/paths.d/homebrew), so Homebrew's copy goes by its name; the download is
// off the PATH and needs its full path.
func (r *run) personCommand(args string) string {
	if detect.Bootstrap(r.f.cfg.Probe) == detect.HomebrewBootstrap {
		return fmt.Sprintf("bootstrap %s in a new Terminal window", args)
	}
	return fmt.Sprintf("%s %s in Terminal", detect.DownloadedBootstrap, args)
}
