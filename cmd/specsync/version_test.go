package main

import (
	"context"
	"strings"
	"testing"
)

func TestVersionStringReleased(t *testing.T) {
	orig := version
	version = "1.2.3"
	defer func() { version = orig }()
	if got := versionString(); got != "1.2.3" {
		t.Errorf("versionString() = %q, want %q", got, "1.2.3")
	}
}

func TestVersionStringDevIncludesRevision(t *testing.T) {
	orig := version
	version = "dev"
	defer func() { version = orig }()
	got := versionString()
	if got != "dev" && !strings.HasPrefix(got, "dev (") {
		t.Errorf("versionString() = %q, want %q or %q-prefixed", got, "dev", "dev (")
	}
}

func TestVersionIsARegisteredCommand(t *testing.T) {
	if _, ok := lookupCommand("version"); !ok {
		t.Fatal("version must be a registered command so help, the README and the skill list it")
	}
	orig := version
	version = "9.9.9"
	defer func() { version = orig }()
	for _, args := range [][]string{{"version"}, {"-version"}, {"--version"}} {
		cmd, rest, err := resolveSubcommand(args)
		if err != nil || cmd != "version" {
			t.Fatalf("%v resolved to %q, %v", args, cmd, err)
		}
		c, _ := lookupCommand(cmd)
		out := captureStdout(t, func() { _ = c.Run(context.Background(), rest) })
		if !strings.Contains(out, "9.9.9") {
			t.Fatalf("%v printed %q", args, out)
		}
	}
}
