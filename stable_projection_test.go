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
