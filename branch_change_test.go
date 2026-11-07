package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProposal(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# Title\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChangeForBranch(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"fix-login-race", "42-numbered", "main", "plain"} {
		writeProposal(t, filepath.Join(root, "changes", slug))
	}
	writeProposal(t, filepath.Join(root, "changes", "archive", "2026-01-01-old-change"))

	cases := []struct {
		branch, defaultBranch, want string
	}{
		{"feat/42-fix-login-race", "main", "fix-login-race"},
		{"someuser/fix-login-race", "main", "fix-login-race"},
		{"fix-login-race", "main", "fix-login-race"},
		{"a/b/fix-login-race", "main", "fix-login-race"},
		{"feat/7-plain", "main", "plain"},
		// The issue-stripped form wins, but an unstripped match still resolves.
		{"feat/42-numbered", "main", "42-numbered"},
		{"main", "main", ""},
		{"main", "", "main"},
		{"HEAD", "main", ""},
		{"", "main", ""},
		{"feat/9-old-change", "main", ""},
		{"feat/9-unknown", "main", ""},
		{"feat/archive", "main", ""},
		{"feat/..", "main", ""},
	}
	for _, tc := range cases {
		if got := ChangeForBranch(root, tc.branch, tc.defaultBranch); got != tc.want {
			t.Errorf("ChangeForBranch(%q, default %q) = %q, want %q", tc.branch, tc.defaultBranch, got, tc.want)
		}
	}
}
