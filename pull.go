package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PullOptions configures an issue-first pull: reading an existing tracker issue
// and materializing it as a local OpenSpec change.
type PullOptions struct {
	OpenSpecDir string       // path to the openspec/ directory
	Provider    WorkProvider // must implement IssueReader
	IssueID     string       // provider id of the issue to pull (e.g. "42")
	Slug        string       // change slug; derived from the issue when empty
	DryRun      bool         // when true, render but never touch disk
}

// PullResult reports what a pull produced (or would produce on a dry run).
type PullResult struct {
	Slug     string
	Dir      string
	IssueURL string
	Proposal string
	Tasks    string
}

// Pull materializes a local OpenSpec change from an existing issue. The change
// is linked to the source issue (via a cached ref) so a later push updates that
// same issue instead of creating a duplicate. The provider must implement
// IssueReader.
func Pull(ctx context.Context, opts PullOptions) (PullResult, error) {
	if opts.Provider == nil {
		return PullResult{}, fmt.Errorf("provider is required")
	}
	reader, ok := opts.Provider.(IssueReader)
	if !ok {
		return PullResult{}, fmt.Errorf("provider %q cannot read issues", opts.Provider.Name())
	}
	if strings.TrimSpace(opts.IssueID) == "" {
		return PullResult{}, fmt.Errorf("issue id is required")
	}

	item, err := reader.Get(ctx, opts.IssueID)
	if err != nil {
		return PullResult{}, err
	}

	slug := opts.Slug
	if slug == "" {
		slug = slugFromMarker(item.Body)
	}
	if slug == "" {
		slug = slugify(item.Title)
	}
	if slug == "" {
		return PullResult{}, fmt.Errorf("could not derive a slug from issue %s; pass -slug", opts.IssueID)
	}

	proposal, tasks := splitBody(item.Body, item.Title)
	res := PullResult{
		Slug:     slug,
		Dir:      filepath.Join(opts.OpenSpecDir, "changes", slug),
		IssueURL: item.URL,
		Proposal: proposal,
		Tasks:    tasks,
	}

	if opts.DryRun {
		return res, nil
	}

	if err := os.MkdirAll(res.Dir, 0o755); err != nil {
		return PullResult{}, fmt.Errorf("create change dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(res.Dir, "proposal.md"), []byte(proposal), 0o644); err != nil {
		return PullResult{}, fmt.Errorf("write proposal: %w", err)
	}
	if tasks != "" {
		if err := os.WriteFile(filepath.Join(res.Dir, "tasks.md"), []byte(tasks), 0o644); err != nil {
			return PullResult{}, fmt.Errorf("write tasks: %w", err)
		}
	}
	// Link the change to the source issue so the next push updates it.
	ref := Ref{Provider: opts.Provider.Name(), ID: item.ID, URL: item.URL}
	if err := saveRef(res.Dir, opts.Provider.Name(), ref); err != nil {
		return PullResult{}, err
	}
	return res, nil
}

// splitBody separates an issue body into proposal and tasks markdown. It drops
// the specsync identity marker and the "## Tasks" heading that push inserts, and
// guarantees the proposal opens with an H1 derived from the issue title. This is
// the inverse of the body rendering in workItemFor + GitHubProvider.renderBody.
func splitBody(body, title string) (proposal, tasks string) {
	var prop, tsk []string
	inTasks := false
	inRelated := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- specsync:change=") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !inTasks && !inRelated && trimmed == "## Tasks" {
			inTasks = true
			continue
		}
		// "## Related" is a managed section — strip it on pull; links.json owns it.
		if !inTasks && !inRelated && trimmed == "## Related" {
			inRelated = true
			continue
		}
		// A new H2 ends the managed sections and returns to proposal content.
		if (inTasks || inRelated) && strings.HasPrefix(trimmed, "## ") {
			inTasks = false
			inRelated = false
			prop = append(prop, line)
			continue
		}
		if inTasks {
			tsk = append(tsk, line)
		} else if !inRelated {
			prop = append(prop, line)
		}
	}

	proposal = strings.TrimSpace(strings.Join(prop, "\n"))
	if !startsWithH1(proposal) {
		h1 := "# " + strings.TrimSpace(title)
		if proposal == "" {
			proposal = h1
		} else {
			proposal = h1 + "\n\n" + proposal
		}
	}
	proposal += "\n"

	tasks = strings.TrimSpace(strings.Join(tsk, "\n"))
	if tasks != "" {
		tasks += "\n"
	}
	return proposal, tasks
}

// startsWithH1 reports whether the first non-blank line is a markdown H1.
func startsWithH1(md string) bool {
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return strings.HasPrefix(t, "# ")
	}
	return false
}

// slugFromMarker returns the slug encoded in a specsync identity marker, or "".
func slugFromMarker(body string) string {
	const open = "<!-- specsync:change="
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "-->")
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

// slugify turns a title into a kebab-case slug: lowercase, with each run of
// non-alphanumeric characters collapsed to a single hyphen and trimmed.
func slugify(s string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		default:
			pendingHyphen = true
		}
	}
	return b.String()
}
