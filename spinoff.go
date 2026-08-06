package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SpinoffOptions configures a spinoff run: spawning a new linked change from
// a discovery or task in an existing change.
type SpinoffOptions struct {
	OpenSpecDir string // path to the openspec/ directory
	Parent      string // slug of the parent change
	TaskIndex   int    // 1-based task index to spin off; 0 = free text mode
	Text        string // discovery text (free text mode)
	Slug        string // child change slug; derived from text when empty
	Repo        string // target repo as owner/name; "" = same as parent
	Kind        string // "bug", "followup", or "task" — maps to issue label
	DryRun      bool
}

// SpinoffResult reports what a spinoff produced (or would produce on a dry run).
type SpinoffResult struct {
	ParentSlug  string
	ParentURL   string
	ChildSlug   string
	ChildDir    string
	Proposal    string
	ParentTask  string // the task text that was moved
	ParentTaskN int    // the 1-based task index that was moved
	Linked      bool   // whether parent↔child links were recorded
	Label       string // the issue label derived from -kind
}

// Spinoff spawns a new linked change from a discovery or task in an existing
// change. It scaffolds a new change folder, marks the originating task as
// moved, and records a parent↔child link.
func Spinoff(ctx context.Context, opts SpinoffOptions) (SpinoffResult, error) {
	if strings.TrimSpace(opts.Parent) == "" {
		return SpinoffResult{}, fmt.Errorf("spinoff: parent change is required")
	}

	parent, err := LoadChangeBySlug(opts.OpenSpecDir, opts.Parent)
	if err != nil {
		return SpinoffResult{}, err
	}

	// Parse the parent's tasks.md to extract task text and build the task list.
	tasksPath := filepath.Join(parent.Dir, "tasks.md")
	tasksBytes, err := os.ReadFile(tasksPath)
	if err != nil {
		return SpinoffResult{}, fmt.Errorf("read parent tasks: %w", err)
	}

	taskLines := strings.Split(string(tasksBytes), "\n")

	var taskText string
	var taskN int

	if opts.TaskIndex > 0 {
		// Task mode: extract text from the specified task.
		taskN = opts.TaskIndex
		taskIndex := 0
		for _, line := range taskLines {
			text, _, ok := parseTaskState(line)
			if !ok {
				continue
			}
			taskIndex++
			if taskIndex == opts.TaskIndex {
				taskText = text
				break
			}
		}
		if taskText == "" {
			return SpinoffResult{}, fmt.Errorf("spinoff: task index %d not found", opts.TaskIndex)
		}
	} else {
		// Free text mode.
		taskText = opts.Text
	}

	if taskText == "" {
		return SpinoffResult{}, fmt.Errorf("spinoff: no text provided; use -task <n> or provide discovery text")
	}

	// Derive child slug from task text.
	childSlug := opts.Slug
	if childSlug == "" {
		childSlug = slugify(taskText)
	}
	if childSlug == "" {
		return SpinoffResult{}, fmt.Errorf("spinoff: could not derive a slug from the text; pass -change")
	}

	// Check that child doesn't already exist.
	childDir := filepath.Join(opts.OpenSpecDir, "changes", childSlug)
	if _, err := os.Stat(childDir); err == nil {
		return SpinoffResult{}, fmt.Errorf("spinoff: change %q already exists", childSlug)
	}

	// Build parent URL for provenance.
	var parentURL string
	if refs, err := loadRefs(parent.Dir); err == nil && len(refs) > 0 {
		_, ref := firstRef(refs)
		parentURL = ref.URL
	}

	// Seed the proposal.
	proposal := seedProposal(taskText, opts.Parent, parentURL)

	// Derive label from -kind.
	label := kindLabel(opts.Kind)

	res := SpinoffResult{
		ParentSlug:  opts.Parent,
		ParentURL:   parentURL,
		ChildSlug:   childSlug,
		ChildDir:    childDir,
		Proposal:    proposal,
		ParentTask:  taskText,
		ParentTaskN: taskN,
		Label:       label,
	}

	if opts.DryRun {
		return res, nil
	}

	// Create child change directory.
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		return SpinoffResult{}, fmt.Errorf("create child dir: %w", err)
	}

	// Write child proposal.md.
	if err := os.WriteFile(filepath.Join(childDir, "proposal.md"), []byte(proposal), 0o644); err != nil {
		return SpinoffResult{}, fmt.Errorf("write child proposal: %w", err)
	}

	// Write child tasks.md (empty with a single todo).
	emptyTasks := "- [ ] TODO\n"
	if err := os.WriteFile(filepath.Join(childDir, "tasks.md"), []byte(emptyTasks), 0o644); err != nil {
		return SpinoffResult{}, fmt.Errorf("write child tasks: %w", err)
	}

	// Mark the parent task as moved (if in task mode).
	if taskN > 0 {
		if err := markTaskMoved(tasksPath, taskN, childSlug); err != nil {
			return SpinoffResult{}, fmt.Errorf("mark task moved: %w", err)
		}
		res.Linked = true
	}

	// Record parent↔child link.
	if parentURL != "" {
		// Add the parent as a link in the child.
		parentRef := Ref{Provider: "github", ID: opts.Parent, URL: parentURL}
		if err := saveLinksToMD(childDir, opts.OpenSpecDir, []Ref{parentRef}); err != nil {
			// Non-fatal: link recording failed but spinoff succeeded.
		}
		res.Linked = true
	}

	return res, nil
}

// markTaskMoved rewrites the task at the given 1-based index as "[>] moved: <slug>".
func markTaskMoved(tasksPath string, taskN int, childSlug string) error {
	tasksBytes, err := os.ReadFile(tasksPath)
	if err != nil {
		return fmt.Errorf("read tasks: %w", err)
	}

	lines := strings.Split(string(tasksBytes), "\n")
	taskIndex := 0
	for i, line := range lines {
		_, _, ok := parseTaskState(line)
		if !ok {
			continue
		}
		taskIndex++
		if taskIndex == taskN {
			lines[i] = "- [>] moved: " + childSlug
			break
		}
	}

	return os.WriteFile(tasksPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// seedProposal creates a proposal.md for a spun-off change from discovery text
// and parent provenance.
func seedProposal(text, parentSlug, parentURL string) string {
	var sb strings.Builder
	sb.WriteString("# " + text + "\n\n")

	if parentURL != "" {
		sb.WriteString("Spun off from [" + parentSlug + "](" + parentURL + ")\n\n")
	} else {
		sb.WriteString("Spun off from `" + parentSlug + "`\n\n")
	}

	sb.WriteString("## What\n\n")
	sb.WriteString(text + "\n")
	return sb.String()
}

// kindLabel maps a -kind value to a GitHub issue label.
func kindLabel(kind string) string {
	switch strings.ToLower(kind) {
	case "bug":
		return "kind:bug"
	case "followup":
		return "kind:followup"
	case "task":
		return "kind:task"
	default:
		return ""
	}
}
