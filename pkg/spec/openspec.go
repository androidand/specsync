package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenSpecSource loads changes from OpenSpec format (openspec/changes/ directory).
// This is the default and primary SpecSource implementation.
type OpenSpecSource struct{}

func (s OpenSpecSource) Name() string {
	return "openspec"
}

// LoadChanges loads all changes from openspec/changes, including archived.
func (s OpenSpecSource) LoadChanges(specDir string) ([]Change, error) {
	changesDir := filepath.Join(specDir, "changes")

	active, err := s.loadChangeDir(changesDir, false)
	if err != nil {
		return nil, err
	}
	archived, err := s.loadChangeDir(filepath.Join(changesDir, "archive"), true)
	if err != nil {
		return nil, err
	}
	return append(active, archived...), nil
}

func (s OpenSpecSource) loadChangeDir(dir string, archived bool) ([]Change, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var out []Change
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		c, err := s.loadChange(filepath.Join(dir, e.Name()), archived)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

// loadChange reads a single change folder. Returns (nil, nil) if proposal.md missing.
func (s OpenSpecSource) loadChange(dir string, archived bool) (*Change, error) {
	body, err := os.ReadFile(filepath.Join(dir, "proposal.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read proposal: %w", err)
	}

	slug := filepath.Base(dir)
	info, _ := os.Stat(dir)
	modified := time.Now().Unix()
	if info != nil {
		modified = info.ModTime().Unix()
	}

	c := &Change{
		Dir:      dir,
		Slug:     slug,
		Title:    firstHeading(string(body), slug),
		Body:     string(body),
		Archived: archived,
		Modified: modified,
		Metadata: &ChangeMetadata{Version: 1},
	}

	if archived {
		c.Metadata.Stage = "archived"
	} else {
		c.Metadata.Stage = "backlog"
	}

	// Load tasks.md if present
	if tasks, err := os.ReadFile(filepath.Join(dir, "tasks.md")); err == nil {
		c.TasksMarkdown = string(tasks)
	}

	// Load .specsync/metadata.json (workflow state: stage, priority)
	if meta, err := s.loadMetadata(dir); err == nil {
		c.Metadata = meta
	}

	// Recalculate stage if needed (tasks, metadata override)
	s.refreshState(c)

	return c, nil
}

// loadMetadata reads .specsync/metadata.json if present.
func (s OpenSpecSource) loadMetadata(dir string) (*ChangeMetadata, error) {
	_, err := os.ReadFile(filepath.Join(dir, ".specsync", "metadata.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &ChangeMetadata{Version: 1}, nil
		}
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	// TODO: implement JSON unmarshaling once pkg/spec is fully separated from main package
	meta := &ChangeMetadata{Version: 1}
	return meta, nil
}

// refreshState updates change stage based on task completion and metadata.
func (s OpenSpecSource) refreshState(c *Change) {
	// If archived, immutable
	if c.Archived {
		c.Metadata.Stage = "archived"
		return
	}

	// If explicitly set in metadata, use it
	if c.Metadata.Stage != "" && c.Metadata.Stage != "backlog" {
		return
	}

	// Derive from tasks if all are complete
	if c.TasksMarkdown != "" && countCheckedTasks(c.TasksMarkdown) == countTotalTasks(c.TasksMarkdown) && countTotalTasks(c.TasksMarkdown) > 0 {
		c.Metadata.Stage = "complete"
		return
	}

	// Default
	if c.Metadata.Stage == "" {
		c.Metadata.Stage = "backlog"
	}
}

// SaveChange persists metadata.json (no-op for now, used by WorkItem updates).
func (s OpenSpecSource) SaveChange(change Change) error {
	// TODO: implement atomic write to .specsync/metadata.json
	return nil
}

// Helper functions (move from existing change.go logic)

func firstHeading(md, fallback string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "##") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	return fallback
}

func countTotalTasks(md string) int {
	count := 0
	for _, line := range strings.Split(md, "\n") {
		if strings.Contains(line, "- [ ]") || strings.Contains(line, "- [x]") || strings.Contains(line, "- [X]") {
			count++
		}
	}
	return count
}

func countCheckedTasks(md string) int {
	count := 0
	for _, line := range strings.Split(md, "\n") {
		if strings.Contains(line, "- [x]") || strings.Contains(line, "- [X]") {
			count++
		}
	}
	return count
}
