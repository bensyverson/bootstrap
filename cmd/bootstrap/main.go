// Command bootstrap lets a coding agent set up a new Mac. See internal/cli.
package main

import (
	"os"

	"github.com/bensyverson/bootstrap/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.SystemEnv()))
}
