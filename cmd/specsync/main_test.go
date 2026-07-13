package main

import (
	"strings"
	"testing"

	"github.com/androidand/specsync"
)

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

// TestParseStatusMapping pins the -status-map syntax: comma-separated
// stage=Name pairs, Status names may contain spaces, whitespace is trimmed.
func TestParseStatusMapping(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "")

	got, err := parseStatusMapping("active=In Progress,archived=Done")
	if err != nil {
		t.Fatalf("parseStatusMapping: %v", err)
	}
	want := map[specsync.Stage]string{
		specsync.StageActive:   "In Progress",
		specsync.StageArchived: "Done",
	}
	if len(got) != len(want) {
		t.Fatalf("mapping = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("mapping[%s] = %q, want %q", k, got[k], v)
		}
	}

	if got, err := parseStatusMapping(" complete = Shipped "); err != nil || got[specsync.StageComplete] != "Shipped" {
		t.Fatalf("whitespace should be trimmed, got %v (err %v)", got, err)
	}

	if got, err := parseStatusMapping(""); err != nil || got != nil {
		t.Fatalf("empty input should yield nil mapping, got %v (err %v)", got, err)
	}
}

// TestParseStatusMappingRejectsBadInput: unknown stages, malformed pairs, and
// duplicate stages must fail loud, naming the problem.
func TestParseStatusMappingRejectsBadInput(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "")
	for input, wantErr := range map[string]string{
		"done=Done":               "unknown",
		"active":                  "must be stage=Name",
		"active=":                 "must be stage=Name",
		"=Done":                   "must be stage=Name",
		"active=Todo,active=Done": "twice",
	} {
		_, err := parseStatusMapping(input)
		if err == nil {
			t.Fatalf("parseStatusMapping(%q): expected an error", input)
		}
		if !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("parseStatusMapping(%q) error %q should mention %q", input, err, wantErr)
		}
	}
}

// TestParseStatusMappingEnvFallback: $SPECSYNC_STATUS_MAP applies when the
// flag is empty, and the flag wins when both are set.
func TestParseStatusMappingEnvFallback(t *testing.T) {
	t.Setenv("SPECSYNC_STATUS_MAP", "archived=Shipped")
	got, err := parseStatusMapping("")
	if err != nil || got[specsync.StageArchived] != "Shipped" {
		t.Fatalf("env fallback: got %v (err %v), want archived=Shipped", got, err)
	}
	got, err = parseStatusMapping("archived=Done")
	if err != nil || got[specsync.StageArchived] != "Done" {
		t.Fatalf("flag should win over env: got %v (err %v)", got, err)
	}
}

// TestBoardTargetCarriesStatusMapping: the parsed mapping must reach the
// BoardTarget the sync/pull paths hand to the library (this wiring was the
// gap that left BoardTarget.StatusMapping unreachable from the CLI).
func TestBoardTargetCarriesStatusMapping(t *testing.T) {
	t.Setenv("SPECSYNC_PROJECT", "")
	t.Setenv("SPECSYNC_STATUS_MAP", "")

	target, err := boardTarget("acme/6", "me", "active=In Progress")
	if err != nil {
		t.Fatalf("boardTarget: %v", err)
	}
	if target.StatusMapping[specsync.StageActive] != "In Progress" {
		t.Fatalf("StatusMapping not carried into BoardTarget: %+v", target)
	}

	// A syntax error in the mapping fails loud even without a project.
	if _, err := boardTarget("", "", "bogus"); err == nil {
		t.Fatal("expected an error for a malformed -status-map without a project")
	}
}
