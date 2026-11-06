package detect

import (
	"os"
	"path/filepath"
	"strings"
)

// Making is what a project folder's files say it's for.
type Making string

const (
	MakingUnknown Making = ""
	MakingWebsite Making = "website"
	MakingApp     Making = "app"
)

// ProjectMaking reads a project folder's top level: a package.json file means a
// website, and a Package.swift file or an .xcodeproj means an app. When both are
// there, package.json wins, in the order Ben listed them (project/2026-11-05-bootstrap-new-plan.md).
// A folder that can't be read says nothing.
func ProjectMaking(dir string) Making {
	if isFile(filepath.Join(dir, "package.json")) {
		return MakingWebsite
	}
	if isFile(filepath.Join(dir, "Package.swift")) {
		return MakingApp
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return MakingUnknown
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
			return MakingApp
		}
	}
	return MakingUnknown
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
