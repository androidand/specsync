package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A repo supplied via -repo and one auto-detected from the git remote must key
// the ref cache identically, so a ref saved by pull is found by a later sync.
func TestGitHubNameKeyIsRepoStable(t *testing.T) {
	explicit := NewGitHubProviderFuncWithRepo("o/r", func(context.Context, ...string) (string, error) {
		return "", nil
	})
	auto := NewGitHubProviderFunc(func(_ context.Context, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
			return "o/r", nil
		}
		return "", nil
	})
	if explicit.Name() != "github:o/r" {
		t.Fatalf("explicit key = %q, want github:o/r", explicit.Name())
	}
	if auto.Name() != explicit.Name() {
		t.Fatalf("auto-detected key %q != explicit key %q", auto.Name(), explicit.Name())
	}
}

// A refs.json written before the key became repo-qualified must still resolve:
// the legacy bare "github" key updates the linked issue (never creates) and is
// re-saved under the canonical key.
func TestSyncLegacyKeyUpdatesAndMigrates(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# C1\n\nbody\n")
	if err := saveRef(cdir, "github", Ref{Provider: "github", ID: "7", URL: "https://github.com/o/r/issues/7"}); err != nil {
		t.Fatalf("seed legacy ref: %v", err)
	}

	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("o/r", func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "view":
			return `{"labels":[]}`, nil
		case args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case args[0] == "issue" && args[1] == "create":
			return "https://github.com/o/r/issues/99", nil
		default:
			return "", nil
		}
	})

	if _, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Slug: "c1"}); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if findCall(calls, "issue", "create") != nil {
		t.Fatalf("legacy-key hit must update, not create: %v", calls)
	}
	if findCall(calls, "issue", "edit", "7") == nil {
		t.Fatalf("expected `issue edit 7`, calls: %v", calls)
	}

	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs["github:o/r"].ID != "7" {
		t.Fatalf("ref not migrated to canonical key: %#v", refs)
	}
	if _, ok := refs["github"]; ok {
		t.Fatalf("legacy entry must be deleted after migration, or link keeps preferring it: %#v", refs)
	}
}

// A legacy bare-"github" ref pointing at another repo must not satisfy the
// fallback: syncing with -repo ownerB/repoB while the cache holds an ownerA
// issue would otherwise edit an unrelated issue number in repoB. The guarded
// miss falls through to marker lookup, then creates in the target repo, and
// the foreign legacy entry is preserved.
func TestSyncLegacyKeyCrossRepoDoesNotClobber(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "c1")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# C1\n\nbody\n")
	legacy := Ref{Provider: "github", ID: "7", URL: "https://github.com/ownerA/repoA/issues/7"}
	if err := saveRef(cdir, "github", legacy); err != nil {
		t.Fatalf("seed legacy ref: %v", err)
	}

	var calls [][]string
	prov := NewGitHubProviderFuncWithRepo("ownerB/repoB", func(_ context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "issue" && args[1] == "list":
			return "[]", nil
		case args[0] == "issue" && args[1] == "create":
			return "https://github.com/ownerB/repoB/issues/99", nil
		default:
			return "", nil
		}
	})

	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov, Slug: "c1"})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if findCall(calls, "issue", "edit") != nil {
		t.Fatalf("cross-repo legacy ref must never be edited: %v", calls)
	}
	if findCall(calls, "issue", "create") == nil {
		t.Fatalf("expected create in the target repo, calls: %v", calls)
	}
	if res.Created != 1 {
		t.Fatalf("Created = %d, want 1", res.Created)
	}

	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if refs["github:ownerB/repoB"].ID != "99" {
		t.Fatalf("new issue not cached under canonical key: %#v", refs)
	}
	if refs["github"] != legacy {
		t.Fatalf("foreign legacy entry must be preserved: %#v", refs)
	}
}

// A legacy bare-"github" entry whose URL doesn't parse as a GitHub issue URL
// belongs to no repo: the guarded fallback can never use it, so migrating to a
// canonical key drops it instead of leaving garbage that shadows real refs.
func TestSaveRefDropsUnparseableLegacyEntry(t *testing.T) {
	cdir := filepath.Join(t.TempDir(), "changes", "c1")
	if err := saveRef(cdir, "github", Ref{Provider: "github", ID: "7", URL: "not-a-url"}); err != nil {
		t.Fatalf("seed legacy ref: %v", err)
	}
	if err := saveRef(cdir, "github:o/r", Ref{Provider: "github:o/r", ID: "9", URL: "https://github.com/o/r/issues/9"}); err != nil {
		t.Fatalf("save canonical ref: %v", err)
	}
	refs, err := loadRefs(cdir)
	if err != nil {
		t.Fatalf("loadRefs: %v", err)
	}
	if _, ok := refs["github"]; ok {
		t.Fatalf("unparseable legacy entry must be dropped on migration: %#v", refs)
	}
	if refs["github:o/r"].ID != "9" {
		t.Fatalf("canonical ref missing: %#v", refs)
	}
}

// firstRef must prefer canonical repo-qualified keys over the legacy bare
// "github" entry (which may be stale), and pick deterministically.
func TestFirstRefPrefersCanonicalKey(t *testing.T) {
	stale := Ref{ID: "7", URL: "https://github.com/old/old/issues/7"}
	canonical := Ref{ID: "9", URL: "https://github.com/a/a/issues/9"}
	refs := map[string]Ref{
		"beads":      {ID: "b-1"},
		"github":     stale,
		"github:b/b": {ID: "12", URL: "https://github.com/b/b/issues/12"},
		"github:a/a": canonical,
	}
	if key, ref := firstRef(refs); key != "github:a/a" || ref != canonical {
		t.Fatalf("firstRef = %q %#v, want github:a/a (first canonical key in sorted order)", key, ref)
	}

	if key, ref := firstRef(map[string]Ref{"github": stale, "beads": {ID: "b-1"}}); key != "github" || ref != stale {
		t.Fatalf("firstRef = %q %#v, want the bare github key when no canonical key exists", key, ref)
	}
	if key, _ := firstRef(map[string]Ref{"beads": {ID: "b-1"}}); key != "beads" {
		t.Fatalf("firstRef = %q, want beads as the only remaining key", key)
	}
}

// The core invariant: a pulled change whose ref cache is later lost is still
// updated (not duplicated) because pull persisted the identity marker, which
// Find rediscovers.
func TestPullThenCacheLossRediscoversViaMarker(t *testing.T) {
	root := t.TempDir()
	issue := fakeIssue{
		Number: 7,
		URL:    "https://github.com/o/r/issues/7",
		Title:  "Round trip",
		State:  "open",
		Body:   "# Round trip\n\nbody\n",
	}
	// body is the issue's live body; pull's marker upsert mutates it in place.
	body := issue.Body
	runner := func(_ context.Context, args ...string) (string, error) {
		switch {
		case args[0] == "repo" && args[1] == "view":
			return "o/r", nil
		case args[0] == "issue" && args[1] == "view":
			if jsonFields(args) == "labels" {
				return `{"labels":[]}`, nil
			}
			i := issue
			i.Body = body
			b, _ := json.Marshal(i)
			return string(b), nil
		case args[0] == "issue" && args[1] == "edit":
			if v := flagValue(args, "--body"); v != "" {
				body = v
			}
			return "", nil
		case args[0] == "issue" && args[1] == "list":
			return fmt.Sprintf(`[{"number":7,"url":%q,"body":%q}]`, issue.URL, body), nil
		default:
			return "", nil
		}
	}

	prov := NewGitHubProviderFunc(runner)
	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: root, Provider: prov, IssueID: "7"})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if !strings.Contains(body, marker(res.Slug)) {
		t.Fatalf("pull did not persist the identity marker into the issue body: %q", body)
	}

	// Simulate total cache loss, then sync via a fresh auto-detect provider.
	if err := os.RemoveAll(filepath.Join(res.Dir, ".specsync")); err != nil {
		t.Fatalf("clear cache: %v", err)
	}
	var calls [][]string
	prov2 := NewGitHubProviderFunc(func(ctx context.Context, args ...string) (string, error) {
		calls = append(calls, args)
		return runner(ctx, args...)
	})
	if _, err := Sync(context.Background(), Options{OpenSpecDir: root, Provider: prov2, Slug: res.Slug}); err != nil {
		t.Fatalf("Sync after cache loss: %v", err)
	}
	if findCall(calls, "issue", "create") != nil {
		t.Fatalf("must not create a duplicate; calls: %v", calls)
	}
	if findCall(calls, "issue", "edit", "7") == nil {
		t.Fatalf("expected `issue edit 7` via marker rediscovery; calls: %v", calls)
	}
}

// A dry-run pull reports the marker it would add but writes nothing to GitHub.
func TestPullDryRunPreviewsMarkerNoWrite(t *testing.T) {
	root := t.TempDir()
	issue := fakeIssue{Number: 7, URL: "u", Title: "T", State: "open", Body: "# T\n\nbody\n"}
	var calls [][]string
	prov := NewGitHubProviderFunc(ghRunner(issue, &calls))

	res, err := Pull(context.Background(), PullOptions{OpenSpecDir: root, Provider: prov, IssueID: "7", DryRun: true})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Marker != marker(res.Slug) {
		t.Fatalf("marker preview = %q, want %q", res.Marker, marker(res.Slug))
	}
	if res.MarkerPresent {
		t.Fatalf("marker should be reported absent for a body without it")
	}
	if findCall(calls, "issue", "edit") != nil {
		t.Fatalf("dry-run must not edit the issue; calls: %v", calls)
	}
}
