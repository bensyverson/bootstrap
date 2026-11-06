package detect

import "path/filepath"

// Where bootstrap itself lives. SETUP.md downloads it off the PATH, and once
// Homebrew is in, the setup installs it from the tap and deletes the download
// (Ben, 2026-11-06, project/2026-11-06-bootstrap-upgrade-plan.md).
const (
	DownloadedBootstrap = "~/.bootstrap/bin/bootstrap"
	HomebrewBootstrap   = "/opt/homebrew/bin/bootstrap"
)

// Bootstrap is the copy of bootstrap to name in what an agent or person runs
// next: Homebrew's once it's there, the download until then.
func Bootstrap(p Probe) string {
	if path, ok := Locate(p, []string{HomebrewBootstrap}); ok {
		return path
	}
	return DownloadedBootstrap
}

// IsDownloadedBootstrap says whether exe is the copy SETUP.md downloaded.
func IsDownloadedBootstrap(p Probe, exe string) bool {
	return filepath.Clean(exe) == resolve(p, DownloadedBootstrap)
}
