package main

import "strings"

// resolveOrca finds Orca's CLI the way Orca's own guide prescribes:
// $ORCA_CLI_COMMAND, then orca-ide on Linux, then orca. On Linux a bare
// `orca` is the GNOME screen reader, so it is never used there. It returns ""
// when Orca is not installed.
func resolveOrca(getenv func(string) string, lookPath func(string) (string, error), goos string) string {
	if v := strings.TrimSpace(getenv("ORCA_CLI_COMMAND")); v != "" {
		return v
	}
	name := "orca"
	if goos == "linux" {
		name = "orca-ide"
	}
	if p, err := lookPath(name); err == nil {
		return p
	}
	return ""
}
