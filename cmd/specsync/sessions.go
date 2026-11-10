package main

import (
	"flag"
	"os"
	"time"

	specsync "github.com/androidand/specsync"
)

// sessionFlags are the flags that name the optional session provider.
type sessionFlags struct {
	cmd     *string
	timeout *time.Duration
}

func addSessionFlags(fs *flag.FlagSet) sessionFlags {
	return sessionFlags{
		cmd:     fs.String("sessions-cmd", "", "command that prints a JSON array of agent sessions (cwd, id, name, status, updatedAt, resume) to show how to resume each piece of work; default $SPECSYNC_SESSIONS_CMD; an empty value disables it for this run. Taken only from here or the environment, never from a repository file"),
		timeout: fs.Duration("sessions-timeout", specsync.DefaultSessionsTimeout, "how long to wait for the session provider"),
	}
}

// command returns the provider command line: the flag when it was given (even
// empty), else the environment.
func (f sessionFlags) command(fs *flag.FlagSet) string {
	given := false
	fs.Visit(func(fl *flag.Flag) {
		if fl.Name == "sessions-cmd" {
			given = true
		}
	})
	if given {
		return *f.cmd
	}
	return os.Getenv("SPECSYNC_SESSIONS_CMD")
}
