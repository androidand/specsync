package specsync

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRankPriorArtMatchesOnOneSharedDistinctiveWord(t *testing.T) {
	// The case that motivated #164: two differently-phrased titles sharing one
	// distinctive word.
	open := []OpenIssue{
		{Number: 128, Title: "Widget sync starves under load"},
		{Number: 131, Title: "Totals double-counted in invoices"},
	}
	got := RankPriorArt("Remediate widget backlog", open, 3)
	if len(got) != 1 || got[0].Number != 128 || got[0].SharedList() != "widget" {
		t.Fatalf("got %#v, want only #128 sharing \"widget\"", got)
	}
}

func TestRankPriorArtIgnoresUnrelatedAndGenericWords(t *testing.T) {
	open := []OpenIssue{{Number: 1, Title: "Fix the update support for changes"}}
	if got := RankPriorArt("Update support when changes happen", open, 3); len(got) != 0 {
		t.Errorf("generic words matched: %#v", got)
	}
	if got := RankPriorArt("Rewrite parser", []OpenIssue{{Number: 2, Title: "Colour palette"}}, 3); len(got) != 0 {
		t.Errorf("unrelated titles matched: %#v", got)
	}
}

func TestRankPriorArtMostSharedFirstAndCapped(t *testing.T) {
	open := []OpenIssue{
		{Number: 1, Title: "billing export"},
		{Number: 2, Title: "billing export retries"},
		{Number: 3, Title: "billing"},
		{Number: 4, Title: "export"},
	}
	got := RankPriorArt("billing export retries", open, 2)
	if len(got) != 2 || got[0].Number != 2 {
		t.Fatalf("got %#v, want 2 results led by #2", got)
	}
}

// fakeSyncGH answers the gh calls a create-or-update sync makes. markerHit is
// the issue Find returns for the change's marker ("" for none).
func fakeSyncGH(markerHit string, openJSON string, openErr error, creates *int) func(context.Context, ...string) (string, error) {
	return func(_ context.Context, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "issue list") && strings.Contains(joined, "--search"):
			return markerHit, nil
		case strings.HasPrefix(joined, "issue list") && strings.Contains(joined, "--state open"):
			return openJSON, openErr
		case strings.HasPrefix(joined, "issue create"):
			*creates++
			return "https://github.com/o/r/issues/77", nil
		case strings.HasPrefix(joined, "issue view"):
			return `{"labels":[],"state":"OPEN"}`, nil
		}
		return "", nil
	}
}

func syncOne(t *testing.T, run func(context.Context, ...string) (string, error)) Result {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "changes", "widget-backlog", "proposal.md"), "# Remediate widget backlog\n\n## Why\nx\n")
	mustWrite(t, filepath.Join(root, "changes", "widget-backlog", "tasks.md"), "- [ ] 1. a\n")
	res, err := Sync(context.Background(), Options{OpenSpecDir: root, Slug: "widget-backlog", Provider: NewGitHubProviderFuncWithRepo("o/r", run)})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return res
}

func TestSyncCreateSurfacesPriorArt(t *testing.T) {
	var creates int
	res := syncOne(t, fakeSyncGH("[]", `[{"number":128,"title":"Widget sync starves under load","url":"u"}]`, nil, &creates))
	if creates != 1 || res.Created != 1 {
		t.Fatalf("creates=%d res=%#v, want one create", creates, res)
	}
	if len(res.Items) != 1 || len(res.Items[0].PriorArt) != 1 || res.Items[0].PriorArt[0].Number != 128 {
		t.Fatalf("prior art = %#v, want #128", res.Items[0].PriorArt)
	}
}

func TestSyncPriorArtLookupFailureChangesNothing(t *testing.T) {
	var creates int
	res := syncOne(t, fakeSyncGH("[]", "", context.DeadlineExceeded, &creates))
	if creates != 1 || res.Created != 1 || len(res.Items[0].PriorArt) != 0 {
		t.Fatalf("creates=%d res=%#v, want a plain create with no hints", creates, res)
	}
}

// A run with no cached ref that finds the issue by its marker updates it; it
// must say "updated", not "created" (the #130 sync printed created … /0).
func TestSyncAdoptedByMarkerReportsUpdateNotCreate(t *testing.T) {
	var creates int
	hit := `[{"number":5,"url":"https://github.com/o/r/issues/5","body":"<!-- specsync:change=widget-backlog -->"}]`
	res := syncOne(t, fakeSyncGH(hit, `[{"number":128,"title":"Widget sync","url":"u"}]`, nil, &creates))
	if creates != 0 {
		t.Fatalf("created %d issues, want 0", creates)
	}
	if res.Created != 0 || res.Updated != 1 || res.Items[0].Created || len(res.Items[0].PriorArt) != 0 {
		t.Fatalf("res = %#v, want one update and no prior-art hint", res)
	}
}
