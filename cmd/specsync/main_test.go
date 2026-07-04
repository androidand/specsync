package main

import "testing"

// TestIsVersionArg pins the dispatch predicate the main switch uses for the
// version subcommand, so the wiring cannot silently regress.
func TestIsVersionArg(t *testing.T) {
	for _, arg := range []string{"version", "-version", "--version"} {
		if !isVersionArg(arg) {
			t.Errorf("isVersionArg(%q) = false, want true", arg)
		}
	}
	for _, arg := range []string{"sync", "pull", "scan", "-v", ""} {
		if isVersionArg(arg) {
			t.Errorf("isVersionArg(%q) = true, want false", arg)
		}
	}
}

// TestVersionDefault ensures source builds report a non-empty placeholder.
func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version must default to a non-empty value (expected \"dev\")")
	}
}
