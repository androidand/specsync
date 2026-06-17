package specsync

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Stage is the canonical lifecycle position of a change. OpenSpec itself has no
// stage concept beyond active/archived; richer stages are an optional overlay
// supplied via the .status convention (see LoadChange).
type Stage string

const (
	StageActive   Stage = "active"   // default: a change living under changes/
	StageArchived Stage = "archived" // change moved under changes/archive/
)

// Change is a provider-agnostic view of one OpenSpec change folder. It is the
// only thing this package reads from disk and is fully self-contained.
type Change struct {
	Dir           string // absolute path to the change folder
	Slug          string
	Title         string // first H1 of proposal.md, falling back to Slug
	Body          string // proposal.md contents
	TasksMarkdown string // tasks.md contents, may be ""
	Stage         Stage  // from .status, else derived (active/archived)
	Priority      int    // from .specsync/priority, 0 if unset
	Archived      bool
}

// LoadChanges reads every change under <openspecDir>/changes, including those
// under changes/archive. openspecDir is typically ".../openspec".
func LoadChanges(openspecDir string) ([]Change, error) {
	changesDir := filepath.Join(openspecDir, "changes")

	active, err := loadChangeDir(changesDir, false)
	if err != nil {
		return nil, err
	}
	archived, err := loadChangeDir(filepath.Join(changesDir, "archive"), true)
	if err != nil {
		return nil, err
	}
	return append(active, archived...), nil
}

func loadChangeDir(dir string, archived bool) ([]Change, error) {
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
		c, err := LoadChange(filepath.Join(dir, e.Name()), archived)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

// LoadChange reads a single change folder. It returns (nil, nil) when the folder
// has no proposal.md and so is not a real change.
func LoadChange(dir string, archived bool) (*Change, error) {
	body, err := os.ReadFile(filepath.Join(dir, "proposal.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read proposal: %w", err)
	}

	slug := filepath.Base(dir)
	c := &Change{
		Dir:      dir,
		Slug:     slug,
		Title:    firstHeading(string(body), slug),
		Body:     string(body),
		Archived: archived,
		Stage:    StageActive,
	}
	if archived {
		c.Stage = StageArchived
	}

	if tasks, err := os.ReadFile(filepath.Join(dir, "tasks.md")); err == nil {
		c.TasksMarkdown = string(tasks)
	}

	// Optional richer stage: a single line in <change>/.status. Vanilla OpenSpec
	// projects omit this and keep the active/archived default.
	if st, err := os.ReadFile(filepath.Join(dir, ".status")); err == nil {
		if s := strings.TrimSpace(string(st)); s != "" {
			c.Stage = Stage(s)
		}
	}

	// Optional priority: <change>/.specsync/priority (gitignored, local).
	if p, err := os.ReadFile(filepath.Join(dir, ".specsync", "priority")); err == nil {
		c.Priority = atoiSafe(strings.TrimSpace(string(p)))
	}

	return c, nil
}

func firstHeading(md, fallback string) string {
	sc := bufio.NewScanner(strings.NewReader(md))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return fallback
}

// atoiSafe parses a non-negative integer, returning 0 on any non-digit input.
func atoiSafe(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
