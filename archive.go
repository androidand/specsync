package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RetentionPolicy is the retention policy applied after archiving.
type RetentionPolicy string

const (
	RetentionPolicyMove  RetentionPolicy = "move"  // relocate to changes/archive/
	RetentionPolicyPrune RetentionPolicy = "prune" // remove the local folder
)

// ArchiveOptions configures an archive run.
type ArchiveOptions struct {
	OpenSpecDir string          // path to the spec root (openspec/)
	Slug        string          // change to archive
	Provider    WorkProvider    // tracker provider for close + label
	Retain      RetentionPolicy // retention policy: move or prune
	Force       bool            // allow archiving with unchecked tasks
	DryRun      bool            // print plan without mutations
}

// ArchiveResult reports what an archive run did or would do.
type ArchiveResult struct {
	Slug           string
	Ref            Ref
	UncheckedTasks int
	Plan           []string
}

// archive performs the full archive lifecycle: final push, unchecked-task check,
// close + spec:archived label, then retention (move or prune).
func Archive(ctx context.Context, opts ArchiveOptions) (*ArchiveResult, error) {
	// Load the change.
	c, err := LoadChangeBySlug(opts.OpenSpecDir, opts.Slug)
	if err != nil {
		return nil, fmt.Errorf("load change %s: %w", opts.Slug, err)
	}

	// Resolve retention policy: explicit flag → .specsync/config → significance.
	configPolicy := readConfigRetain(c.Dir)
	retain := resolveRetainPolicy(opts.Retain, configPolicy, c.Significant)

	// Count unchecked tasks.
	total, done := CountCheckboxes(c.TasksMarkdown)
	unchecked := total - done

	// Build a plan of what would happen.
	var plan []string
	plan = append(plan, fmt.Sprintf("archive: %s (%s)", opts.Slug, c.Title))

	if unchecked > 0 && !opts.Force {
		plan = append(plan, fmt.Sprintf("  ✗ %d unchecked task(s) — use -force to override", unchecked))
		return &ArchiveResult{
			Slug:           opts.Slug,
			Ref:            Ref{},
			UncheckedTasks: unchecked,
			Plan:           plan,
		}, nil
	}

	// Final push: reconcile and push to tracker.
	plan = append(plan, "  → final push")
	if opts.DryRun {
		plan = append(plan, "  (dry-run: would push and reconcile)")
	} else {
		// Sync the change to ensure the issue is current.
		_, err := Sync(ctx, Options{
			OpenSpecDir:    opts.OpenSpecDir,
			Provider:       opts.Provider,
			Slug:           opts.Slug,
			DryRun:         false,
			CloseCompleted: true,
		})
		if err != nil {
			return nil, fmt.Errorf("final push: %w", err)
		}
	}

	// Close the issue and add spec:archived label.
	plan = append(plan, "  → close issue + add spec:archived label")
	ref, err := closeAndLabel(ctx, opts.Provider, c.Slug, opts.DryRun)
	if err != nil {
		return nil, fmt.Errorf("close + label: %w", err)
	}
	if opts.DryRun {
		plan = append(plan, "  (dry-run: would close issue and add spec:archived label)")
	} else {
		plan = append(plan, fmt.Sprintf("  → closed issue #%s, added spec:archived", ref.ID))
	}

	// Apply retention.
	switch retain {
	case RetentionPolicyMove:
		archiveDir := filepath.Join(opts.OpenSpecDir, "changes", "archive", c.Slug)
		plan = append(plan, fmt.Sprintf("  → move %s → %s", c.Dir, archiveDir))
		if opts.DryRun {
			plan = append(plan, "  (dry-run: no file changes)")
		} else {
			if err := moveChange(c.Dir, archiveDir); err != nil {
				return nil, fmt.Errorf("move: %w", err)
			}
		}
	case RetentionPolicyPrune:
		plan = append(plan, fmt.Sprintf("  → prune %s", c.Dir))
		if opts.DryRun {
			plan = append(plan, "  (dry-run: no file changes)")
		} else {
			if err := confirmClosedAndPrune(ctx, opts.Provider, ref); err != nil {
				return nil, fmt.Errorf("prune: %w", err)
			}
			if err := os.RemoveAll(c.Dir); err != nil {
				return nil, fmt.Errorf("remove change dir: %w", err)
			}
		}
	}

	return &ArchiveResult{
		Slug:           opts.Slug,
		Ref:            ref,
		UncheckedTasks: unchecked,
		Plan:           plan,
	}, nil
}

// resolveRetainPolicy determines the retention policy by resolution order:
// explicit flag → .specsync/config → significance heuristic default.
// configPolicy is pre-read from .specsync/config so the caller passes it in.
func resolveRetainPolicy(explicit RetentionPolicy, configPolicy RetentionPolicy, significant bool) RetentionPolicy {
	// 1. Explicit flag wins.
	if explicit != "" {
		return explicit
	}

	// 2. .specsync/config plain keys.
	if configPolicy != "" {
		return configPolicy
	}

	// 3. Significance heuristic: significant → move, trivial → prune.
	if significant {
		return RetentionPolicyMove
	}
	return RetentionPolicyPrune
}

// readConfigRetain reads the retain key from .specsync/config (plain text, key=value).
func readConfigRetain(changeDir string) RetentionPolicy {
	path := filepath.Join(changeDir, ".specsync", "config")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(strings.ToLower(k))
		v = strings.TrimSpace(strings.ToLower(v))
		if k == "retain" && (v == "move" || v == "prune") {
			return RetentionPolicy(v)
		}
	}
	return ""
}

// closeAndLabel ensures the spec:archived label exists, attaches it to the
// change's issue, and resolves the ref. The close itself already happened
// via the "final push" step in Archive (Sync with CloseCompleted: true,
// which renders a real WorkItem via WorkItemFor) — a second push here must
// not reconstruct one from scratch: a bare WorkItem{Slug, Closed,
// ManageClosed} has no Title, and GitHubProvider.Push has no "partial
// update" mode — it always edits the full issue, so an empty title is
// rejected by the tracker outright. EnsureLabels only makes the label
// *definition* exist in the repo (`gh label create`); actually attaching it
// to the issue needs the separate LabelApplier capability.
func closeAndLabel(ctx context.Context, provider WorkProvider, slug string, dryRun bool) (Ref, error) {
	if dryRun {
		return Ref{}, nil
	}

	// Ensure spec:archived exists as a usable label in the repo (idempotent).
	if la, ok := provider.(interface {
		EnsureLabels(context.Context, []string) error
	}); ok {
		if err := la.EnsureLabels(ctx, []string{"spec:archived"}); err != nil {
			return Ref{}, fmt.Errorf("ensure spec:archived label: %w", err)
		}
	}

	finder, ok := provider.(interface {
		Find(context.Context, string) (*Ref, error)
	})
	if !ok {
		return Ref{}, fmt.Errorf("provider does not support Find")
	}
	ref, err := finder.Find(ctx, slug)
	if err != nil {
		return Ref{}, fmt.Errorf("find existing ref: %w", err)
	}
	if ref == nil {
		return Ref{}, fmt.Errorf("no existing issue found for %s — the final push should have created one", slug)
	}

	if la, ok := provider.(LabelApplier); ok {
		if err := la.ApplyLabelDelta(ctx, ref.ID, []string{"spec:archived"}, nil); err != nil {
			return Ref{}, fmt.Errorf("apply spec:archived label: %w", err)
		}
	}

	return *ref, nil
}

// moveChange relocates a change folder to archiveDir, preserving .specsync/refs.json
// so the marker→issue link survives the move.
func moveChange(src, dst string) error {
	// Create the destination directory.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	// Copy the entire source directory to the destination.
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("copy to archive: %w", err)
	}

	// Remove the original.
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("remove original: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// confirmClosedAndPrune verifies the issue is closed before pruning. The
// issue body/title were already made current by the "final push" step in
// Archive (a real Sync, before this ever runs); there is nothing to verify
// beyond closed state, and no reason to push again (see closeAndLabel).
func confirmClosedAndPrune(ctx context.Context, provider WorkProvider, ref Ref) error {
	reader, ok := provider.(interface {
		Get(context.Context, string) (FetchedItem, error)
	})
	if !ok {
		return nil
	}
	item, err := reader.Get(ctx, ref.ID)
	if err != nil {
		return fmt.Errorf("verify issue state: %w", err)
	}
	if !item.Closed {
		return fmt.Errorf("issue #%s is not closed; cannot prune", ref.ID)
	}
	return nil
}
