package specsync

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Stage is the canonical lifecycle position of a change. OpenSpec itself has no
// stage concept beyond active/archived; richer stages are an optional overlay
// supplied via the .status convention (see LoadChange).
type Stage string

const (
	StageActive   Stage = "active"   // default: a change living under changes/
	StageComplete Stage = "complete" // every task checked, not yet archived
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
	Links         []Ref  // resolved related issue refs from links.md
	Stage         Stage  // .status override, else derived (active/complete/archived)
	Priority      int    // from .specsync/priority, 0 if unset
	Archived      bool
}

// LoadChanges reads every change under <openspecDir>/changes, including those
// under changes/archive. openspecDir is typically ".../openspec".
func LoadChanges(openspecDir string) ([]Change, error) {
	changesDir := filepath.Join(openspecDir, "changes")

	active, err := loadChangeDir(changesDir, false, openspecDir)
	if err != nil {
		return nil, err
	}
	archived, err := loadChangeDir(filepath.Join(changesDir, "archive"), true, openspecDir)
	if err != nil {
		return nil, err
	}
	return append(active, archived...), nil
}

func loadChangeDir(dir string, archived bool, openspecDir string) ([]Change, error) {
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
		c, err := LoadChange(filepath.Join(dir, e.Name()), archived, openspecDir)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, *c)
		}
	}
	return out, nil
}

// LoadChange reads a single change folder. openspecDir is used to resolve
// slug-based entries in links.md; pass "" when not known (slug entries are
// skipped). Returns (nil, nil) when the folder has no proposal.md.
func LoadChange(dir string, archived bool, openspecDir string) (*Change, error) {
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

	// Derive completion from the task list: a change whose every task is checked
	// has finished its work but is not yet archived. Surfacing that as its own
	// stage lets trackers stop showing the change as active the moment the last
	// box is ticked, with no manual step. Archiving (a filesystem fact) still
	// wins, and an explicit .status below is the ultimate override.
	if !archived && tasksComplete(c.TasksMarkdown) {
		c.Stage = StageComplete
	}

	// Optional richer stage: a single line in <change>/.status.
	if st, err := os.ReadFile(filepath.Join(dir, ".status")); err == nil {
		if s := strings.TrimSpace(string(st)); s != "" {
			c.Stage = Stage(s)
		}
	}

	// Optional priority: <change>/.specsync/priority (gitignored, local).
	if p, err := os.ReadFile(filepath.Join(dir, ".specsync", "priority")); err == nil {
		c.Priority = atoiSafe(strings.TrimSpace(string(p)))
	}

	// Optional related issues: links.md (human/agent/machine-writable).
	c.Links = parseLinksMD(dir, openspecDir)

	return c, nil
}

// loadChangeBySlug finds a change by slug, checking active then archived dirs.
func loadChangeBySlug(openspecDir, slug string) (*Change, error) {
	dir := filepath.Join(openspecDir, "changes", slug)
	c, err := LoadChange(dir, false, openspecDir)
	if err != nil {
		return nil, fmt.Errorf("load change %q: %w", slug, err)
	}
	if c == nil {
		dir = filepath.Join(openspecDir, "changes", "archive", slug)
		c, err = LoadChange(dir, true, openspecDir)
		if err != nil {
			return nil, fmt.Errorf("load archived change %q: %w", slug, err)
		}
	}
	if c == nil {
		return nil, fmt.Errorf("no change found for slug %q", slug)
	}
	return c, nil
}

// reShorthand matches "owner/repo#N" GitHub shorthand references.
var reShorthand = regexp.MustCompile(`^[A-Za-z0-9_.\-]+/[A-Za-z0-9_.\-]+#\d+$`)

// parseLinksMD reads links.md from changeDir and resolves each entry to a Ref.
// Unresolvable slug entries (sibling not yet synced) are silently skipped;
// they appear automatically once the sibling is synced and LoadChange re-runs.
// Supported line formats:
//
//   - https://github.com/owner/repo/issues/N   (full URL)
//   - owner/repo#N                             (GitHub shorthand)
//   - some-slug                                (sibling slug, resolved via refs.json)
//   - some-slug repo:owner/name                (sibling slug + explicit repo hint)
func parseLinksMD(changeDir, openspecDir string) []Ref {
	b, err := os.ReadFile(filepath.Join(changeDir, "links.md"))
	if err != nil {
		return nil
	}
	var refs []Ref
	for _, rawLine := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		entry := strings.TrimSpace(line[2:])
		if entry == "" {
			continue
		}
		if ref := resolveEntry(entry, openspecDir); ref != nil {
			refs = append(refs, *ref)
		}
	}
	return refs
}

func resolveEntry(entry, openspecDir string) *Ref {
	// Full URL.
	if strings.HasPrefix(entry, "https://") || strings.HasPrefix(entry, "http://") {
		return refFromURL(entry)
	}

	// GitHub shorthand: owner/repo#N.
	if reShorthand.MatchString(entry) {
		idx := strings.LastIndex(entry, "#")
		repoPath := entry[:idx]
		num := entry[idx+1:]
		url := "https://github.com/" + repoPath + "/issues/" + num
		return &Ref{Provider: "github:" + repoPath, ID: num, URL: url}
	}

	// Slug with optional "repo:owner/name" hint.
	slug := entry
	repoHint := ""
	if idx := strings.Index(entry, " repo:"); idx > 0 {
		slug = strings.TrimSpace(entry[:idx])
		repoHint = strings.TrimSpace(entry[idx+6:])
	}

	if openspecDir == "" {
		return nil // can't resolve slugs without knowing where siblings live
	}

	// Try to resolve via the sibling's ref cache.
	for _, dir := range []string{
		filepath.Join(openspecDir, "changes", slug),
		filepath.Join(openspecDir, "changes", "archive", slug),
	} {
		refs, err := loadRefs(dir)
		if err != nil || len(refs) == 0 {
			continue
		}
		// Prefer the repo-hinted key, fall back to plain "github" or first found.
		if repoHint != "" {
			if ref, ok := refs["github:"+repoHint]; ok {
				r := ref
				return &r
			}
		}
		_, ref := firstRef(refs)
		r := ref
		return &r
	}
	return nil // not yet synced — will resolve on the next push after sibling syncs
}

// refFromURL builds a Ref from a full GitHub issue URL, or a bare-URL Ref for
// non-GitHub links.
func refFromURL(url string) *Ref {
	const prefix = "https://github.com/"
	if strings.HasPrefix(url, prefix) {
		rest := url[len(prefix):]
		if i := strings.Index(rest, "/issues/"); i >= 0 {
			repo := rest[:i]
			num := rest[i+8:]
			// Trim any trailing path segments.
			if j := strings.IndexByte(num, '/'); j >= 0 {
				num = num[:j]
			}
			return &Ref{Provider: "github:" + repo, ID: num, URL: url}
		}
	}
	return &Ref{URL: url}
}

// tasksComplete reports whether the tasks markdown has at least one task line
// and every task line is checked. It reuses parseTaskLine so it counts exactly
// the "- [ ]"/"- [x]" lines reconcile does — other checkbox markers (living
// plan's [~]/[>]) and prose are ignored. An empty or task-less list is not
// "complete": there is nothing to have finished.
func tasksComplete(md string) bool {
	if strings.TrimSpace(md) == "" {
		return false
	}
	any := false
	for _, line := range strings.Split(md, "\n") {
		if _, checked, ok := parseTaskLine(line); ok {
			if !checked {
				return false
			}
			any = true
		}
	}
	return any
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
