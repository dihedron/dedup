package command

import (
	"github.com/dihedron/dedup/commands/index"
	"github.com/dihedron/dedup/commands/version"
)

// Commands is the set of root command groups.
type Commands struct {
	// Version prints the application's version information and exits.
	Index index.Index `command:"index" alias:"idx" alias:"i" description:"Index the given directory(es) contents."`
	// Version prints the application's version information and exits.
	Version version.Version `command:"version" alias:"ver" alias:"v" description:"Show the application version and exit."`
}
