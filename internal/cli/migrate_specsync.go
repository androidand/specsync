package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"specsync/internal/openspec"
)

// MigrateSpecSyncOptions configures a migration run.
type MigrateSpecSyncOptions struct {
	OpenSpecDir string // path to openspec/ directory
	Strategy    string // "auto-prioritize" or "clear"
	DryRun      bool
}

// MigrateSpecSync performs backfill migration for existing changes.
// Strategies:
//   - "auto-prioritize": Estimate priority from task count, creation date
//   - "clear": Reset all to priority=0, stage=backlog (manual reprioritization)
func MigrateSpecSync(opts MigrateSpecSyncOptions) error {
	changes, err := openspec.LoadChanges(opts.OpenSpecDir)
	if err != nil {
		return fmt.Errorf("load changes: %w", err)
	}

	if len(changes) == 0 {
		fmt.Println("No changes found; nothing to migrate")
		return nil
	}

	switch opts.Strategy {
	case "auto-prioritize":
		return migrateAutoPrioritize(changes, opts.DryRun)
	case "clear":
		return migrateClear(changes, opts.DryRun)
	default:
		return fmt.Errorf("unknown strategy %q; use auto-prioritize or clear", opts.Strategy)
	}
}

// migrateAutoPrioritize estimates priorities based on heuristics.
// Higher task count → higher priority (more complex = more important).
// Older created date → higher priority (been waiting longer).
func migrateAutoPrioritize(changes []openspec.Change, dryRun bool) error {
	// Sort by task count descending, then creation date ascending.
	sort.Slice(changes, func(i, j int) bool {
		taskCountI := countTasks(changes[i])
		taskCountJ := countTasks(changes[j])
		if taskCountI != taskCountJ {
			return taskCountI > taskCountJ // More tasks = higher priority
		}
		return changes[i].Created.Before(changes[j].Created) // Older first
	})

	// Assign priorities based on position.
	// First 10% → priority 80-99 (high)
	// Next 30% → priority 50-79 (normal)
	// Next 30% → priority 30-49 (low)
	// Last 30% → priority 1-29 (very low)
	var migrated, skipped int

	for i, c := range changes {
		pri := estimatePriority(i, len(changes))

		// Check if metadata already exists.
		metaPath := filepath.Join(c.Dir, ".specsync", "metadata.json")
		meta := &specsyncMetadata{}

		if data, err := os.ReadFile(metaPath); err == nil {
			if err := json.Unmarshal(data, meta); err == nil {
				if meta.Priority != nil {
					// Already has priority; skip
					fmt.Printf("⊘ %s (already has priority=%d)\n", c.Slug, *meta.Priority)
					skipped++
					continue
				}
			}
		}

		// Set priority from estimation.
		meta.Version = 1
		meta.Priority = &pri
		meta.Stage = "backlog" // Ensure stage is set

		if !dryRun {
			if err := writeMetadata(c.Dir, meta); err != nil {
				fmt.Printf("✗ %s: failed to write metadata: %v\n", c.Slug, err)
				continue
			}
		}

		fmt.Printf("✓ %s priority=%d (tasks=%d, created=%s)\n", c.Slug, pri, countTasks(c), c.Created.Format("2006-01-02"))
		migrated++
	}

	fmt.Printf("\nMigration complete: %d prioritized, %d skipped\n", migrated, skipped)
	if dryRun {
		fmt.Println("(dry-run mode; no changes written)")
	}
	return nil
}

// migrateClear resets all changes to default state.
// Allows manual reprioritization afterward.
func migrateClear(changes []openspec.Change, dryRun bool) error {
	var cleared int

	for _, c := range changes {
		metaPath := filepath.Join(c.Dir, ".specsync", "metadata.json")

		// Always write (reset to default), even if exists.
		meta := &specsyncMetadata{
			Version: 1,
			Stage:   "backlog",
			// Priority: nil (no explicit priority)
		}

		if !dryRun {
			if err := writeMetadata(c.Dir, meta); err != nil {
				fmt.Printf("✗ %s: failed to write metadata: %v\n", c.Slug, err)
				continue
			}
		}

		fmt.Printf("✓ %s reset to backlog (priority unset)\n", c.Slug)
		cleared++
	}

	fmt.Printf("\nMigration complete: %d cleared to backlog\n", cleared)
	if dryRun {
		fmt.Println("(dry-run mode; no changes written)")
	}
	return nil
}

// estimatePriority returns a priority (1-100) based on position in sorted list.
func estimatePriority(position, total int) int {
	percentile := float64(position) / float64(total)

	if percentile < 0.1 {
		// Top 10% → priority 80-99
		return 99 - (position % 20)
	} else if percentile < 0.4 {
		// Next 30% → priority 50-79
		return 79 - ((position - total/10) % 30)
	} else if percentile < 0.7 {
		// Next 30% → priority 30-49
		return 49 - ((position - total/10 - total*3/10) % 20)
	} else {
		// Last 30% → priority 1-29
		return 29 - ((position - total/10 - total*3/10 - total*3/10) % 29)
	}
}

// countTasks counts unchecked + checked tasks in a change.
func countTasks(c openspec.Change) int {
	return len(c.GetTasks())
}

// specsyncMetadata represents .specsync/metadata.json structure.
type specsyncMetadata struct {
	Version  int    `json:"version"`
	Stage    string `json:"stage,omitempty"`
	Priority *int   `json:"priority,omitempty"`
}

// writeMetadata atomically writes .specsync/metadata.json.
func writeMetadata(changeDir string, meta *specsyncMetadata) error {
	metaDir := filepath.Join(changeDir, ".specsync")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	metaPath := filepath.Join(metaDir, "metadata.json")
	tmpPath := metaPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, metaPath)
}
