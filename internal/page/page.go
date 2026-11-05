// Package page is the one seam between the server and what the browser draws.
// The real page replaces what's in files/, with no change to the server.
package page

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"io/fs"
	"regexp"
	"strings"
)

//go:embed all:files
var embedded embed.FS

var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// Files returns the page's files. The server serves index.html at / and the rest by name.
func Files() fs.FS {
	files, err := fs.Sub(embedded, "files")
	if err != nil {
		// fs.Sub only fails on an invalid path, and this one is a constant.
		panic(err)
	}
	return files
}

// Policy returns the Content-Security-Policy for a page, which proves it loads
// nothing from outside. The address's token means even the first script has to be
// inline, so the policy allows exactly that script, by its hash.
func Policy(files fs.FS) string {
	scripts := []string{"'self'"}
	if index, err := fs.ReadFile(files, "index.html"); err == nil {
		for _, match := range inlineScript.FindAllSubmatch(index, -1) {
			sum := sha256.Sum256(match[1])
			scripts = append(scripts, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	}
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + strings.Join(scripts, " "),
		"style-src 'self'",
		"font-src 'self'",
		"img-src 'self'",
		"connect-src 'self'",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; ")
}
