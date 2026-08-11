package main

import (
	"fmt"
	"os"
)

// version holds the CLI application version, defaulting to "dev".
// It can be overwritten at build time using ldflags
// (e.g. -ldflags "-X main.version=1.0.0").
var version = "dev"

// main is the CLI entrypoint. It executes the root command.
func main() {
	rootCmd := newRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
