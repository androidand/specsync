package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PullOptions configures an issue-first pull: reading an existing tracker issue
// and materializing it as a local change.
type PullOptions struct {
	OpenSpecDir string       // path to the spec root (openspec/, beads/, etc.)
	Provider    WorkProvider // must implement IssueReader
	IssueID     string       // provider id of the issue to pull (e.g. "42"); when empty, resolved from branch name
	Slug        string       // change slug; derived from the issue when empty
	DryRun      bool         // when true, render but never touch disk
	Project     BoardTarget  // optional GitHub Projects board; unset = no board operations
}

// PullResult reports what a pull produced (or would produce on a dry run).
type PullResult struct {
	Slug        string
	Dir         string
	IssueID     string // resolved issue ID (from -issue flag or branch name)
	IssueURL    string
	Proposal    string
	Tasks       string
	Links       []string // URLs from the ## Related section, for dry-run display
	OriginalAsk string   // from ## Original ask section of the issue
	Discoveries string   // from ## Discoveries section of the issue
	Marker      string   // identity marker upserted into the source issue
	// MarkerPresent reports whether the source issue already carried the marker,
	// i.e. no write was (or would be) needed. Drives the dry-run preview.
	MarkerPresent bool
	// TitleSuggestion is a tighter variant of the issue title, set only when
	// shortenTitle would modify it. Pull writes the title verbatim — rewriting
	// someone else's issue title is not specsync's call — but surfaces the
	// suggestion so the author can tighten the proposal H1 after pulling.
	TitleSuggestion string
	// Board reports the board projection; BoardConfigured is false when no target
	// project was configured (Board is zero and no board calls ran).
	BoardConfigured bool
	Board           BoardPlan
}

// Pull materializes a local change from an existing issue. The change
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

	issueID := strings.TrimSpace(opts.IssueID)
	if issueID == "" {
		if opts.Provider.Name() != "github" && !strings.HasPrefix(opts.Provider.Name(), "github:") {
			return PullResult{}, fmt.Errorf("issue id is required for non-github providers")
		}
		// Resolve issue from branch name (e.g., feat/42-change → #42).
		repo := opts.Provider.Name()
		if strings.HasPrefix(repo, "github:") {
			repo = strings.TrimPrefix(repo, "github:")
		}
		br := NewBranchResolver(repo)
		result, err := br.Resolve(ctx, "")
		if err != nil {
			return PullResult{}, fmt.Errorf("resolve issue from branch: %w", err)
		}
		if result == nil || result.Ref == nil {
			return PullResult{}, fmt.Errorf("could not resolve issue from branch name; specify -issue or use an issue-linked branch (e.g., feat/42-change-name)")
		}
		issueID = result.Ref.ID
	}

	item, err := reader.Get(ctx, issueID)
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
		return PullResult{}, fmt.Errorf("could not derive a change name from issue %s; pass -change", opts.IssueID)
	}

	proposal, tasks, relatedURLs, origAsk, disc := splitBody(item.Body, item.Title)
	res := PullResult{
		Slug:            slug,
		Dir:             filepath.Join(opts.OpenSpecDir, "changes", slug),
		IssueID:         issueID,
		IssueURL:        item.URL,
		Proposal:        proposal,
		Tasks:           tasks,
		Links:           relatedURLs,
		OriginalAsk:     origAsk,
		Discoveries:     disc,
		Marker:          marker(slug),
		MarkerPresent:   strings.Contains(item.Body, marker(slug)),
		TitleSuggestion: titleSuggestion(item.Title),
	}

	// Project onto the board when a target is configured and the provider supports
	// it. Done for both dry and real runs (the projector makes no mutation on a
	// dry run); skipped entirely when unconfigured.
	res.BoardConfigured = opts.Project.Configured()
	if opts.Project.Configured() {
		if bp, ok := opts.Provider.(BoardProjector); ok {
			pulledItem := WorkItem{Slug: slug, Title: item.Title, Stage: StageActive}
			ref := Ref{Provider: opts.Provider.Name(), ID: item.ID, URL: item.URL}
			plan, perr := bp.ProjectOntoBoard(ctx, opts.Project, ref, pulledItem, opts.DryRun)
			if perr != nil {
				return PullResult{}, perr
			}
			res.Board = plan
		}
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
		// Save baseline task count for tracking added tasks later.
		tc := countTaskStates(tasks)
		baseline := tc.Total()
		meta, _ := LoadChangeMetadata(res.Dir)
		if meta == nil {
			meta = &ChangeMetadata{}
		}
		meta.BaselineTaskCount = &baseline
		if err := SaveChangeMetadata(res.Dir, *meta); err != nil {
			return PullResult{}, fmt.Errorf("save baseline task count: %w", err)
		}
	}
	// Preserve Related links as links.md so the next push renders them.
	if len(relatedURLs) > 0 {
		var refs []Ref
		for _, u := range relatedURLs {
			if r := refFromURL(u); r != nil {
				refs = append(refs, *r)
			}
		}
		if err := saveLinksToMD(res.Dir, opts.OpenSpecDir, refs, nil, nil); err != nil {
			return PullResult{}, fmt.Errorf("write links.md: %w", err)
		}
	}
	// Save original ask from the first pull (never overwritten on re-pull).
	// On first pull the issue won't have ## Original ask yet, so we seed it
	// from the proposal. On re-pull we strip it but never overwrite the file.
	askPath := filepath.Join(res.Dir, "original-ask.md")
	if _, err := os.Stat(askPath); os.IsNotExist(err) {
		// Use the rendered original ask if available, otherwise the proposal.
		askContent := res.OriginalAsk
		if askContent == "" {
			askContent = res.Proposal
		}
		if err := os.WriteFile(askPath, []byte(askContent), 0o644); err != nil {
			return PullResult{}, fmt.Errorf("write original-ask: %w", err)
		}
	}
	// Save discoveries from the issue (appended, not overwritten, to preserve local notes).
	if res.Discoveries != "" {
		discPath := filepath.Join(res.Dir, "discoveries.md")
		existing, _ := os.ReadFile(discPath)
		if len(existing) == 0 {
			if err := os.WriteFile(discPath, []byte(res.Discoveries), 0o644); err != nil {
				return PullResult{}, fmt.Errorf("write discoveries: %w", err)
			}
		}
	}
	// Link the change to the source issue so the next push updates it.
	ref := Ref{Provider: opts.Provider.Name(), ID: item.ID, URL: item.URL}
	if err := saveRef(res.Dir, opts.Provider.Name(), ref); err != nil {
		return PullResult{}, err
	}
	// Persist the identity marker into the source issue so the link is durable:
	// even if the ref cache is deleted, a later sync rediscovers it via Find.
	if mw, ok := opts.Provider.(IssueMarkerWriter); ok {
		if _, err := mw.EnsureMarker(ctx, item.ID, slug, item.Body); err != nil {
			return PullResult{}, fmt.Errorf("persist identity marker: %w", err)
		}
	}
	return res, nil
}

// splitBody separates an issue body into proposal, tasks, related-issue URLs,
// original ask, and discoveries. It drops the specsync identity marker and the
// managed sections (## Tasks, ## Related, ## Original ask, ## Discoveries,
// ## Plan changes), and guarantees the proposal opens with an H1 derived from
// the issue title. This is the inverse of WorkItemFor rendering.
func splitBody(body, title string) (proposal, tasks string, relatedURLs []string, originalAsk, discoveries string) {
	var prop, tsk, ask, disc []string
	inTasks := false
	inRelated := false
	inOriginalAsk := false
	inDiscoveries := false
	inPlanChanges := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- specsync:change=") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		// Check managed section headers first — they can appear even while
		// inside another managed section (e.g. ## Tasks after ## Original ask).
		switch trimmed {
		case "## Tasks":
			inTasks = true
			inRelated = false
			inOriginalAsk = false
			inDiscoveries = false
			inPlanChanges = false
			continue
		case "## Related":
			inTasks = false
			inRelated = true
			inOriginalAsk = false
			inDiscoveries = false
			inPlanChanges = false
			continue
		case "## Original ask":
			inTasks = false
			inRelated = false
			inOriginalAsk = true
			inDiscoveries = false
			inPlanChanges = false
			continue
		case "## Discoveries":
			inTasks = false
			inRelated = false
			inOriginalAsk = false
			inDiscoveries = true
			inPlanChanges = false
			continue
		case "## Plan changes":
			inTasks = false
			inRelated = false
			inOriginalAsk = false
			inDiscoveries = false
			inPlanChanges = true
			continue
		}
		// A new (non-managed) H2 ends the current managed section and returns
		// to proposal content.
		if strings.HasPrefix(trimmed, "## ") {
			inTasks = false
			inRelated = false
			inOriginalAsk = false
			inDiscoveries = false
			inPlanChanges = false
			prop = append(prop, line)
			continue
		}
		switch {
		case inTasks:
			tsk = append(tsk, line)
		case inRelated:
			// Collect URLs from "- [label](url)" or "- url" entries.
			if strings.HasPrefix(trimmed, "- ") {
				entry := strings.TrimSpace(trimmed[2:])
				if u := extractURL(entry); u != "" {
					relatedURLs = append(relatedURLs, u)
				}
			}
		case inOriginalAsk:
			ask = append(ask, line)
		case inDiscoveries:
			disc = append(disc, line)
		case inPlanChanges:
			// discard plan changes footer content
		default:
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
	originalAsk = strings.TrimSpace(strings.Join(ask, "\n"))
	if originalAsk != "" {
		originalAsk += "\n"
	}
	discoveries = strings.TrimSpace(strings.Join(disc, "\n"))
	if discoveries != "" {
		discoveries += "\n"
	}
	return proposal, tasks, relatedURLs, originalAsk, discoveries
}

// extractURL pulls the href out of "[label](url)" or returns the string as-is
// if it looks like a bare URL.
func extractURL(s string) string {
	if strings.HasPrefix(s, "[") {
		if i := strings.Index(s, "]("); i >= 0 {
			rest := s[i+2:]
			if j := strings.Index(rest, ")"); j >= 0 {
				return rest[:j]
			}
		}
	}
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return s
	}
	return ""
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
