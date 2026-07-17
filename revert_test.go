package specsync

import (
	"strings"
	"testing"
)

func revertOf(hash, target, header string) Commit {
	return ParseCommit(hash, "a", "2026-07-17", header+"\n\nThis reverts commit "+target+".\n")
}

// TestParseCommitRevertsHash pins extraction of git's standard revert body line.
func TestParseCommitRevertsHash(t *testing.T) {
	c := revertOf("aaaaaaa1234", "08c9109bb766c000d9c6efcc119ee478e45add17", `Revert "fix: clean issue titles on pull"`)
	if c.RevertsHash != "08c9109bb766c000d9c6efcc119ee478e45add17" {
		t.Fatalf("RevertsHash = %q", c.RevertsHash)
	}
	plain := ParseCommit("bbbbbbb1234", "a", "2026-07-17", "fix: something\n\nbody mentioning reverts commit casually\n")
	if plain.RevertsHash != "" {
		t.Fatalf("non-revert commit got RevertsHash %q", plain.RevertsHash)
	}
}

// TestCancelRevertPairs pins the net-effect rules: in-range pairs vanish,
// chains resolve newest-first, and a revert of an out-of-range commit stays.
func TestCancelRevertPairs(t *testing.T) {
	fix := ParseCommit("1111111aaaa", "a", "d", "fix: shorten titles")
	other := ParseCommit("3333333cccc", "a", "d", "fix: unrelated")

	t.Run("pair cancels", func(t *testing.T) {
		rev := revertOf("2222222bbbb", "1111111aaaa", `Revert "fix: shorten titles"`)
		got := cancelRevertPairs([]Commit{fix, other, rev})
		if len(got) != 1 || got[0].Hash != other.Hash {
			t.Fatalf("want only %s to survive, got %v", other.Hash, hashes(got))
		}
	})

	t.Run("abbreviated target hash cancels", func(t *testing.T) {
		rev := revertOf("2222222bbbb", "1111111", `Revert "fix: shorten titles"`)
		got := cancelRevertPairs([]Commit{fix, rev})
		if len(got) != 0 {
			t.Fatalf("want empty, got %v", hashes(got))
		}
	})

	t.Run("revert of out-of-range commit is kept", func(t *testing.T) {
		rev := revertOf("2222222bbbb", "9999999eeee", `Revert "feat: from last release"`)
		got := cancelRevertPairs([]Commit{other, rev})
		if len(got) != 2 {
			t.Fatalf("want both kept, got %v", hashes(got))
		}
	})

	t.Run("revert of revert leaves the original", func(t *testing.T) {
		revA := revertOf("2222222bbbb", "1111111aaaa", `Revert "fix: shorten titles"`)
		revRev := revertOf("4444444dddd", "2222222bbbb", `Revert "Revert "fix: shorten titles""`)
		got := cancelRevertPairs([]Commit{fix, revA, revRev})
		if len(got) != 1 || got[0].Hash != fix.Hash {
			t.Fatalf("net effect must be the original fix, got %v", hashes(got))
		}
	})
}

// TestBuildChangelogCancelsRevertedCommits pins the end-to-end behavior that
// motivated this change: a fix that landed and was reverted before release
// must not publish a raw fallback entry for either commit.
func TestBuildChangelogCancelsRevertedCommits(t *testing.T) {
	fix := ParseCommit("1111111aaaa", "a", "d", "fix: shorten issue titles by stripping parentheticals")
	rev := revertOf("2222222bbbb", "1111111aaaa", `Revert "fix: shorten issue titles by stripping parentheticals"`)
	kept := ParseCommit("3333333cccc", "a", "d", "fix: reject unrecognized leading arguments")

	cl := BuildChangelog(TraceInput{Commits: []Commit{fix, rev, kept}}, nil)
	var texts []string
	for _, e := range cl.Entries {
		texts = append(texts, e.Text)
	}
	joined := strings.Join(texts, "\n")
	if strings.Contains(joined, "shorten issue titles") {
		t.Fatalf("reverted commit leaked into changelog: %v", texts)
	}
	if !strings.Contains(joined, "reject unrecognized leading arguments") {
		t.Fatalf("unreverted commit missing from changelog: %v", texts)
	}
}

func hashes(cs []Commit) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Hash)
	}
	return out
}
