package main

import (
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
