package main

import (
	"os/exec"
	"strings"
	"testing"

	specsync "github.com/androidand/specsync"
)

// A table built without pull request lookups must say "Not opened" may be wrong.
func TestStatusTableWarnsWhenPullRequestsWereNotLooked(t *testing.T) {
	rows := []specsync.ChangeTopology{{Slug: "a", Title: "A", Repo: "acme/portal"}}
	for state, warns := range map[string]bool{"ok": false, "": false, "off": true, "omitted": true} {
		out := captureStdout(t, func() { printStatusTable(rows, "", nil, state, false) })
		if got := strings.Contains(out, "pull requests were not looked up"); got != warns {
			t.Errorf("request state %q: warning shown = %v, want %v\n%s", state, got, warns, out)
		}
	}
}

func TestUrlsNeedsTable(t *testing.T) {
	bin := buildSpecsyncBinary(t)
	for _, cmd := range []string{"status", "overview"} {
		c := exec.Command(bin, cmd, "-urls")
		c.Dir = t.TempDir()
		out, err := c.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "-urls needs -table") {
			t.Errorf("%s -urls: err=%v, output %q; want a failure saying -urls needs -table", cmd, err, out)
		}
	}
}
