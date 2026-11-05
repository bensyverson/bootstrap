// Package page is the one seam between the server and what the browser draws.
// The real page replaces what's in files/, with no change to the server.
package page

import (
	"embed"
	"io/fs"
)

//go:embed all:files
var embedded embed.FS

// Files returns the page's files. The server serves index.html at / and the rest by name.
func Files() fs.FS {
	files, err := fs.Sub(embedded, "files")
	if err != nil {
		// fs.Sub only fails on an invalid path, and this one is a constant.
		panic(err)
	}
	return files
}
