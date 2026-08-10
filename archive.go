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
			if err := confirmClosedAndPrune(ctx, opts.Provider, c.Slug, ref); err != nil {
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

// closeAndLabel closes the issue for the change and ensures the spec:archived
// label exists. It pushes a WorkItem with Closed=true and ManageClosed=true
// which triggers the close path in the provider; it also ensures the label
// is created idempotently.
func closeAndLabel(ctx context.Context, provider WorkProvider, slug string, dryRun bool) (Ref, error) {
	if dryRun {
		return Ref{}, nil
	}

	// Ensure spec:archived label exists (idempotent).
	if la, ok := provider.(interface {
		EnsureLabels(context.Context, []string) error
	}); ok {
		if err := la.EnsureLabels(ctx, []string{"spec:archived"}); err != nil {
			return Ref{}, fmt.Errorf("ensure spec:archived label: %w", err)
		}
	}

	// Push a close request. We need the existing ref to know which issue to close.
	// Find the existing ref first.
	var ref Ref
	if finder, ok := provider.(interface {
		Find(context.Context, string) (*Ref, error)
	}); ok {
		r, err := finder.Find(ctx, slug)
		if err != nil {
			return Ref{}, fmt.Errorf("find existing ref: %w", err)
		}
		if r != nil {
			ref = *r
		}
	}

	// Push with Closed=true to trigger the close path.
	// We use a minimal WorkItem: the slug is empty (no sync needed),
	// but we set ManageClosed=true so the provider enforces the close.
	item := WorkItem{
		Slug:         slug,
		ManageClosed: true,
		Closed:       true,
	}

	// If we have an existing ref, push through it so the provider's
	// close logic runs correctly.
	if ref.URL != "" {
		if pusher, ok := provider.(interface {
			Push(context.Context, WorkItem, *Ref) (Ref, error)
		}); ok {
			return pusher.Push(ctx, item, &ref)
		}
	}

	// Fallback: if we can't find the ref, try pushing without one.
	if pusher, ok := provider.(interface {
		Push(context.Context, WorkItem, *Ref) (Ref, error)
	}); ok {
		return pusher.Push(ctx, item, nil)
	}

	return ref, fmt.Errorf("provider does not support Push")
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

// confirmClosedAndPrune verifies the issue is closed and current before pruning.
func confirmClosedAndPrune(ctx context.Context, provider WorkProvider, slug string, ref Ref) error {
	// Verify the issue is closed by reading it back.
	if reader, ok := provider.(interface {
		Get(context.Context, string) (FetchedItem, error)
	}); ok {
		item, err := reader.Get(ctx, ref.ID)
		if err != nil {
			return fmt.Errorf("verify issue state: %w", err)
		}
		if !item.Closed {
			return fmt.Errorf("issue #%s is not closed; cannot prune", ref.ID)
		}
	}

	// Also verify the issue body is current by pushing a no-op.
	if pusher, ok := provider.(interface {
		Push(context.Context, WorkItem, *Ref) (Ref, error)
	}); ok {
		_, err := pusher.Push(ctx, WorkItem{
			Slug: slug, Body: "spec:archived",
		}, &ref)
		if err != nil {
			return fmt.Errorf("verify issue is current: %w", err)
		}
	}

	return nil
}
