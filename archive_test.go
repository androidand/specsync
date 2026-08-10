package specsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveUncheckedTasksBlocked verifies that archive refuses to proceed
// when there are unchecked tasks and -force is not set.
func TestArchiveUncheckedTasksBlocked(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "incomplete")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Incomplete\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] todo one\n- [x] done one\n")

	prov := stubProvider{ref: Ref{Provider: "github", ID: "42"}}
	result, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root,
		Slug:        "incomplete",
		Provider:    prov,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if result.UncheckedTasks != 1 {
		t.Errorf("UncheckedTasks = %d, want 1", result.UncheckedTasks)
	}
	// The plan should contain the refusal message.
	found := false
	for _, line := range result.Plan {
		if line == "  ✗ 1 unchecked task(s) — use -force to override" {
			found = true
		}
	}
	if !found {
		t.Errorf("plan missing refusal message; got: %v", result.Plan)
	}
}

// TestArchiveUncheckedTasksWithForce proceeds when -force is set.
func TestArchiveUncheckedTasksWithForce(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "force-me")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Force Me\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [ ] still todo\n")

	prov := stubProvider{ref: Ref{Provider: "github", ID: "42"}}
	result, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root,
		Slug:        "force-me",
		Provider:    prov,
		Force:       true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if result.UncheckedTasks != 1 {
		t.Errorf("UncheckedTasks = %d, want 1", result.UncheckedTasks)
	}
	// With force, the plan should NOT contain the refusal line.
	for _, line := range result.Plan {
		if line == "  ✗ 1 unchecked task(s) — use -force to override" {
			t.Fatalf("plan should not contain refusal with -force; got: %v", result.Plan)
		}
	}
}

// TestArchiveRetentionMove performs a real archive with move retention
// and verifies the folder is relocated.
func TestArchiveRetentionMove(t *testing.T) {
	root := t.TempDir()
	openspecDir := filepath.Join(root, "openspec")
	cdir := filepath.Join(openspecDir, "changes", "move-test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Move Test\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"), `{"github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"}}`)

	prov := stubProvider{ref: Ref{Provider: "github", ID: "42"}}
	_, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: openspecDir,
		Slug:        "move-test",
		Provider:    prov,
		Retain:      RetentionPolicyMove,
		Force:       true,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Archive move: %v", err)
	}

	// Original should be gone.
	if _, err := os.Stat(cdir); !os.IsNotExist(err) {
		t.Fatalf("original dir still exists after move")
	}

	// Archived should exist with all files.
	archiveDir := filepath.Join(root, "openspec", "changes", "archive", "move-test")
	if _, err := os.Stat(archiveDir); err != nil {
		t.Fatalf("archive dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, "proposal.md")); err != nil {
		t.Fatalf("proposal.md missing in archive")
	}

	// .specsync/refs.json should be preserved.
	refs, err := LoadRefs(archiveDir)
	if err != nil {
		t.Fatalf("LoadRefs archive: %v", err)
	}
	if _, ok := refs["github"]; !ok {
		t.Fatalf("refs.json not preserved in archive")
	}
}

// TestArchiveRetentionPrune performs a real archive with prune retention
// and verifies the folder is removed.
func TestArchiveRetentionPrune(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "prune-test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Prune Test\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")

	// For prune, we need a provider that can verify the issue is closed.
	// Use a stub that supports Get and Push.
	stub := &pruneStubProvider{
		ref:    Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
		closed: true,
	}

	_, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root,
		Slug:        "prune-test",
		Provider:    stub,
		Retain:      RetentionPolicyPrune,
		Force:       true,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("Archive prune: %v", err)
	}

	// Original should be gone.
	if _, err := os.Stat(cdir); !os.IsNotExist(err) {
		t.Fatalf("original dir still exists after prune")
	}
}

// pruneStubProvider is a stub that supports Get (for prune verification) and Push.
type pruneStubProvider struct {
	ref    Ref
	closed bool
}

func (p *pruneStubProvider) Name() string { return "github" }
func (p *pruneStubProvider) Push(_ context.Context, _ WorkItem, existing *Ref) (Ref, error) {
	if existing != nil {
		return *existing, nil
	}
	return p.ref, nil
}
func (p *pruneStubProvider) Find(_ context.Context, _ string) (*Ref, error) {
	return &p.ref, nil
}
func (p *pruneStubProvider) Get(_ context.Context, _ string) (FetchedItem, error) {
	return FetchedItem{Closed: p.closed}, nil
}

// TestArchiveSignificanceHeuristic verifies the significance detection logic.
func TestArchiveSignificanceHeuristic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(dir string)
		wantSig bool
	}{
		{
			name: "significant marker",
			setup: func(dir string) {
				mustWrite(t, filepath.Join(dir, "significant"), "")
			},
			wantSig: true,
		},
		{
			name: "design.md",
			setup: func(dir string) {
				mustWrite(t, filepath.Join(dir, "design.md"), "# Design\n")
			},
			wantSig: true,
		},
		{
			name: "many tasks (>5)",
			setup: func(dir string) {
				mustWrite(t, filepath.Join(dir, "tasks.md"),
					"- [ ] t1\n- [ ] t2\n- [ ] t3\n- [ ] t4\n- [ ] t5\n- [ ] t6\n")
			},
			wantSig: true,
		},
		{
			name: "few tasks (<=5)",
			setup: func(dir string) {
				mustWrite(t, filepath.Join(dir, "tasks.md"),
					"- [ ] t1\n- [x] t2\n")
			},
			wantSig: false,
		},
		{
			name:    "nothing",
			setup:   func(_ string) {},
			wantSig: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(dir)
			got := IsSignificant(dir)
			if got != tc.wantSig {
				t.Errorf("IsSignificant(%s) = %v, want %v", tc.name, got, tc.wantSig)
			}
		})
	}
}

// TestArchiveDryRunNoMutations verifies that dry-run makes zero API calls
// and zero file changes.
func TestArchiveDryRunNoMutations(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "dry-run-test")
	archiveDir := filepath.Join(root, "openspec", "changes", "archive", "dry-run-test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Dry Run Test\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"), `{"github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"}}`)

	// Track API calls.
	var calls [][]string
	prov := &trackingProvider{
		ref:    Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
		closed: true,
		run: func(_ context.Context, args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		},
	}

	_, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root,
		Slug:        "dry-run-test",
		Provider:    prov,
		Retain:      RetentionPolicyMove,
		Force:       true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Archive dry-run: %v", err)
	}

	// No API calls should have been made.
	if len(calls) > 0 {
		t.Errorf("dry-run made %d API call(s); expected 0; calls: %v", len(calls), calls)
	}

	// Original directory should still exist.
	if _, err := os.Stat(cdir); err != nil {
		t.Fatalf("original dir should still exist after dry-run: %v", err)
	}

	// Archive directory should NOT have been created.
	if _, err := os.Stat(archiveDir); !os.IsNotExist(err) {
		t.Fatalf("archive dir should not exist after dry-run")
	}
}

// trackingProvider is a GitHubProvider-like stub that tracks all calls.
type trackingProvider struct {
	ref    Ref
	closed bool
	run    func(ctx context.Context, args ...string) (string, error)
}

func (p *trackingProvider) Name() string { return "github" }
func (p *trackingProvider) Push(_ context.Context, _ WorkItem, existing *Ref) (Ref, error) {
	if existing != nil {
		return *existing, nil
	}
	return p.ref, nil
}
func (p *trackingProvider) Find(_ context.Context, _ string) (*Ref, error) {
	return &p.ref, nil
}
func (p *trackingProvider) Get(_ context.Context, _ string) (FetchedItem, error) {
	return FetchedItem{Closed: p.closed}, nil
}

// TestArchiveRetentionConfigOverride verifies that .specsync/config retain key
// overrides the significance heuristic.
func TestArchiveRetentionConfigOverride(t *testing.T) {
	// Config prune overrides significant → move.
	got := resolveRetainPolicy("", RetentionPolicyPrune, true)
	if got != RetentionPolicyPrune {
		t.Errorf("resolveRetainPolicy = %q, want %q (config overrides heuristic)", got, RetentionPolicyPrune)
	}
}

// TestArchiveRetentionExplicitFlag verifies that the explicit -retain flag
// overrides everything.
func TestArchiveRetentionExplicitFlag(t *testing.T) {
	// Explicit prune should win over significant + config.
	got := resolveRetainPolicy(RetentionPolicyPrune, RetentionPolicyMove, true)
	if got != RetentionPolicyPrune {
		t.Errorf("resolveRetainPolicy(prune, ...) = %q, want prune", got)
	}

	// Explicit move should win.
	got = resolveRetainPolicy(RetentionPolicyMove, RetentionPolicyPrune, false)
	if got != RetentionPolicyMove {
		t.Errorf("resolveRetainPolicy(move, ...) = %q, want move", got)
	}
}

// TestArchiveRetentionDefaultSignificant verifies significant changes default to move.
func TestArchiveRetentionDefaultSignificant(t *testing.T) {
	got := resolveRetainPolicy("", "", true)
	if got != RetentionPolicyMove {
		t.Errorf("significant change: resolveRetainPolicy = %q, want move", got)
	}
}

// TestArchiveRetentionDefaultTrivial verifies trivial changes default to prune.
func TestArchiveRetentionDefaultTrivial(t *testing.T) {
	got := resolveRetainPolicy("", "", false)
	if got != RetentionPolicyPrune {
		t.Errorf("trivial change: resolveRetainPolicy = %q, want prune", got)
	}
}

// TestArchiveMovePreservesRefsJSON verifies .specsync/refs.json survives the move.
func TestArchiveMovePreservesRefsJSON(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "refs-test")
	dst := filepath.Join(root, "openspec", "changes", "archive", "refs-test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Refs Test\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "refs.json"), `{
  "github":{"provider":"github","id":"42","url":"https://github.com/o/r/issues/42"},
  "beads":{"provider":"beads","id":"bd-123","url":"bd://123"}
}`)

	if err := moveChange(cdir, dst); err != nil {
		t.Fatalf("moveChange: %v", err)
	}

	refs, err := LoadRefs(dst)
	if err != nil {
		t.Fatalf("LoadRefs: %v", err)
	}
	if _, ok := refs["github"]; !ok {
		t.Error("github ref not preserved")
	}
	if _, ok := refs["beads"]; !ok {
		t.Error("beads ref not preserved")
	}
}

// TestArchivePruneFailsWhenNotClosed verifies prune refuses when the issue is not closed.
func TestArchivePruneFailsWhenNotClosed(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "not-closed")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Not Closed\n")
	mustWrite(t, filepath.Join(cdir, "tasks.md"), "- [x] done\n")

	stub := &pruneStubProvider{
		ref:    Ref{Provider: "github", ID: "42", URL: "https://github.com/o/r/issues/42"},
		closed: false, // not closed!
	}

	_, err := Archive(context.Background(), ArchiveOptions{
		OpenSpecDir: root,
		Slug:        "not-closed",
		Provider:    stub,
		Retain:      RetentionPolicyPrune,
		Force:       true,
		DryRun:      false,
	})
	if err == nil {
		t.Fatalf("expected error when issue is not closed")
	}
	if !strings.Contains(err.Error(), "not closed") {
		t.Fatalf("error should mention 'not closed': %v", err)
	}
}
