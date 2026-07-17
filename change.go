package specsync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TaskProgress represents the completion state of a change's tasks.
type TaskProgress string

const (
	TaskProgressNoTasks    TaskProgress = "no-tasks"      // no tasks.md file
	TaskProgressNotStarted TaskProgress = "not-started"   // 0/N tasks complete
	TaskProgressInProgress TaskProgress = "in-progress"   // 0 < X < N
	TaskProgressComplete   TaskProgress = "complete"      // N/N tasks complete
)

// Stage is the workflow placement of a change. It is distinct from task progress.
// Workflow stage can be explicitly set via .specsync/metadata.json or derived
// from tasks/location.
type Stage string

const (
	StageBacklog   Stage = "backlog"      // not yet started; pre-discovery or deferred
	StageBlocked   Stage = "blocked"      // waiting on external blocker or decision
	StageActive    Stage = "active"       // in flight; has unchecked work
	StageInReview  Stage = "in-review"    // awaiting approval before proceeding
	StageComplete  Stage = "complete"     // all work done, not yet archived
	StageArchived  Stage = "archived"     // moved to changes/archive/ (immutable)
)

// ValidateStage checks if a stage value is canonical or matches the custom stage pattern.
func ValidateStage(stage Stage) error {
	if IsCanonicalStage(stage) {
		return nil
	}
	// Custom stages must match token pattern: lowercase alphanumeric + hyphens
	pattern := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	if !pattern.MatchString(string(stage)) {
		return fmt.Errorf("invalid stage %q\n"+
			"  Canonical: backlog, active, blocked, in-review, complete, archived\n"+
			"  Custom: lowercase letters/numbers/hyphens, 1-64 chars (e.g., awaiting-review)", stage)
	}
	return nil
}

// IsCanonicalStage reports whether stage is one of the six canonical values.
func IsCanonicalStage(stage Stage) bool {
	switch stage {
	case StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived:
		return true
	default:
		return false
	}
}

// CanonicalStageOrder returns the canonical stage ordering for sorting.
func CanonicalStageOrder() []Stage {
	return []Stage{
		StageBacklog, StageBlocked, StageActive, StageInReview, StageComplete, StageArchived,
	}
}

// ChangeMetadata holds shared workflow metadata from .specsync/metadata.json.
type ChangeMetadata struct {
	Version  int    `json:"version"`
	Stage    *Stage `json:"stage,omitempty"`
	Priority *int   `json:"priority,omitempty"`
}

// StageSource indicates how the current stage was derived.
type StageSource string

const (
	StageSourceDefault      StageSource = "default"       // no other source; assume active
	StageSourceTasks        StageSource = "tasks"         // derived from task completion (all done → complete)
	StageSourceMetadata     StageSource = "metadata"      // explicit .specsync/metadata.json stage field
	StageSourceLegacyStatus StageSource = "legacy-status" // read from .status file (backward compat)
	StageSourceFolder       StageSource = "folder"        // archived folder location (final, immutable)
)

// Change is a provider-agnostic view of one OpenSpec change folder. It is the
// only thing this package reads from disk and is fully self-contained.
type Change struct {
	Dir           string       // absolute path to the change folder
	Slug          string
	Title         string       // first H1 of proposal.md, falling back to Slug
	Body          string       // proposal.md contents
	TasksMarkdown string       // tasks.md contents, may be ""
	Links         []Ref        // resolved related issue refs from links.md
	Archived      bool
	Progress      TaskProgress // what the task checklist says
	Stage         Stage        // current workflow placement
	StageSource   StageSource  // how we arrived at Stage (default/tasks/metadata/legacy-status/folder)
	Priority      *int         // optional 1-100; nil if unset
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

	if err := refreshState(c); err != nil {
		return nil, fmt.Errorf("load state for %s: %w", slug, err)
	}

	// Optional related issues: links.md (human/agent/machine-writable).
	c.Links = parseLinksMD(dir, openspecDir)

	return c, nil
}

// LoadChangeBySlug finds a change by slug, checking active then archived dirs.
func LoadChangeBySlug(openspecDir, slug string) (*Change, error) {
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
// countCheckboxes returns (total, completed) checkbox counts from markdown.
func countCheckboxes(md string) (total, completed int) {
	if strings.TrimSpace(md) == "" {
		return 0, 0
	}
	for _, line := range strings.Split(md, "\n") {
		if _, checked, ok := parseTaskLine(line); ok {
			total++
			if checked {
				completed++
			}
		}
	}
	return total, completed
}

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

// refreshStage derives lifecycle from the change's current task state, then
// applies filesystem facts and the explicit .status override. Sync calls this
// again after inbound reconciliation so one invocation projects the resulting
// state rather than the state loaded before checkbox merging.
// refreshState derives task progress and workflow stage with clear precedence.
func refreshState(c *Change) error {
	// Step 1: Always derive progress from tasks
	c.Progress = deriveTaskProgress(c.TasksMarkdown)

	// Step 2: Archived folder is immutable and final
	if c.Archived {
		c.Stage = StageArchived
		c.StageSource = StageSourceFolder
		return nil
	}

	// Step 3: Try explicit metadata from .specsync/metadata.json
	metadata, err := LoadChangeMetadata(c.Dir)
	if err != nil {
		return err // malformed metadata blocks loading
	}

	// Load priority from metadata if present (can be independent of stage)
	if metadata != nil && metadata.Priority != nil {
		c.Priority = metadata.Priority
	}

	// Try explicit stage from metadata
	if metadata != nil && metadata.Stage != nil {
		c.Stage = *metadata.Stage
		c.StageSource = StageSourceMetadata
		return nil
	}

	// Step 4: Try legacy .status file for backward compat
	if legacyStage, ok := readLegacyStatus(c.Dir); ok {
		c.Stage = legacyStage
		c.StageSource = StageSourceLegacyStatus
		// Warn if both files exist and differ
		if metadata != nil && metadata.Stage != nil && *metadata.Stage != legacyStage {
			warnStageMismatch(c.Slug, *metadata.Stage, legacyStage)
		}
		return nil
	}

	// Step 5: Derive from task completion
	if c.Progress == TaskProgressComplete {
		c.Stage = StageComplete
		c.StageSource = StageSourceTasks
		return nil
	}

	// Step 6: Default to active
	c.Stage = StageActive
	c.StageSource = StageSourceDefault
	return nil
}

// deriveTaskProgress derives progress from task checklist state.
func deriveTaskProgress(tasksMarkdown string) TaskProgress {
	if tasksMarkdown == "" {
		return TaskProgressNoTasks
	}

	total, completed := countCheckboxes(tasksMarkdown)
	if total == 0 {
		return TaskProgressNoTasks
	}
	if completed == 0 {
		return TaskProgressNotStarted
	}
	if completed == total {
		return TaskProgressComplete
	}
	return TaskProgressInProgress
}

// LoadChangeMetadata loads and validates a change folder's
// .specsync/metadata.json, returning nil if the file is absent.
func LoadChangeMetadata(dir string) (*ChangeMetadata, error) {
	path := filepath.Join(dir, ".specsync", "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // file absent; no metadata
		}
		return nil, fmt.Errorf("read .specsync/metadata.json: %w", err)
	}

	var m ChangeMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse .specsync/metadata.json: %w", err)
	}

	if err := normalizeMetadata(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveChangeMetadata atomically writes a change folder's
// .specsync/metadata.json. Metadata that carries neither a stage nor a
// priority means "no manual overrides", so the file is removed instead of
// written — LoadChangeMetadata then reports nil and stage derivation falls
// back to tasks/legacy/default. This is the single write path for workflow
// metadata; the CLI's set-stage and set-priority both go through it so
// unsetting one field can never drop the other.
func SaveChangeMetadata(dir string, m ChangeMetadata) error {
	if err := normalizeMetadata(&m); err != nil {
		return err
	}

	metaDir := filepath.Join(dir, ".specsync")
	path := filepath.Join(metaDir, "metadata.json")

	if m.Stage == nil && m.Priority == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}

	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", metaDir, err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename metadata file: %w", err)
	}
	return nil
}

// normalizeMetadata validates metadata version and field values.
func normalizeMetadata(m *ChangeMetadata) error {
	if m.Version == 0 {
		m.Version = 1
	}

	if m.Version != 1 {
		return fmt.Errorf("unsupported .specsync/metadata.json version %d", m.Version)
	}

	if m.Stage != nil {
		if err := ValidateStage(*m.Stage); err != nil {
			return err
		}
	}

	if m.Priority != nil {
		if *m.Priority < 1 || *m.Priority > 100 {
			return fmt.Errorf("priority must be 1–100; got %d\n"+
				"  1-29   VERY_LOW  (docs, cleanup)\n"+
				"  30-49  LOW  (nice-to-have)\n"+
				"  50-69  NORMAL  (regular work)\n"+
				"  70-89  HIGH  (user-facing features)\n"+
				"  90-98  CRITICAL  (security, data loss prevention)\n"+
				"  99     FOCUS  (human priority)", *m.Priority)
		}
	}

	return nil
}

// readLegacyStatus reads .status file for backward compatibility.
func readLegacyStatus(dir string) (Stage, bool) {
	path := filepath.Join(dir, ".status")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	stage := Stage(strings.TrimSpace(string(data)))
	return stage, true
}

// warnStageMismatch emits a stderr warning when .specsync/metadata.json and
// legacy .status disagree.
func warnStageMismatch(slug string, metaStage, statusStage Stage) {
	fmt.Fprintf(os.Stderr,
		"warning: %s defines stage in both .specsync/metadata.json and legacy .status;\n"+
			"  using .specsync/metadata.json (%q)\n"+
			"  run `specsync set-stage %s auto` to migrate\n",
		slug, metaStage, slug,
	)
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
