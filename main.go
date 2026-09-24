// Command mega is a developer-friendly CLI for MEGA cloud storage.
package main

import (
	"os"

	"github.com/manzanit0/mega-cli/internal/cmd"
)

// main runs the CLI and exits with its status code.
func main() {
	os.Exit(cmd.Execute())
}
