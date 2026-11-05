package detect

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bensyverson/bootstrap/catalog"
)

// Downloads lists the folders in sign.Dir holding a file that starts with sign.Prefix.
func Downloads(p Probe, sign catalog.Sign) map[string]bool {
	found := map[string]bool{}
	dir := resolve(p, sign.Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return found
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasPrefix(f.Name(), sign.Prefix) {
				found[entry.Name()] = true
				break
			}
		}
	}
	return found
}

// InstallerRunning reports whether a tool's installer is open, or false when it has none or the probe can't tell.
func InstallerRunning(p Probe, tool catalog.Tool) bool {
	return tool.Installer != "" && p.Running != nil && p.Running(tool.Installer)
}
