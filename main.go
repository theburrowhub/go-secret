package main

import "github.com/theburrowhub/go-secret/cmd"

// Version information (injected at build time via ldflags)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	// Set version info in cmd package
	cmd.Version = Version
	cmd.Commit = Commit
	cmd.BuildDate = BuildDate

	cmd.Execute()
}
