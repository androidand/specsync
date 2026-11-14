package specsync

import (
	"os/exec"
	"strings"
	"testing"
)

// TestMakeStampsVersion pins that local builds carry a real version: build
// must pass it to the linker as main.version, VERSION= must override the
// git-derived value, and a build with no version available must not stamp.
func TestMakeStampsVersion(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("make", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("make %v: %v\n%s", args, err, out)
		}
		return string(out)
	}

	if got := run("-n", "build", "VERSION=9.8.7"); !strings.Contains(got, "-X main.version=9.8.7") {
		t.Errorf("build does not stamp the overridden version:\n%s", got)
	}
	if got := strings.TrimSpace(run("-s", "version", "VERSION=9.8.7")); got != "9.8.7" {
		t.Errorf("make version = %q, want 9.8.7", got)
	}
	if got := run("-n", "build", "VERSION="); strings.Contains(got, "main.version") {
		t.Errorf("an empty VERSION must leave the binary unstamped:\n%s", got)
	}
}
