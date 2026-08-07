package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// runTrace prints the raw resolved trace graph for a scope (debugging/scripting).
func runTrace(args []string) {
	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	change := fs.String("change", "", "scope to a single change slug")
	since := fs.String("since", "", "range start (default: latest tag)")
	until := fs.String("until", "", "range end (default: HEAD)")
	asJSON := fs.Bool("json", false, "emit JSON")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	scope := specsync.Scope{Change: *change, Since: *since, Until: *until}
	tr := resolve(abs, scope)

	if *asJSON {
		emitJSON(tr)
		return
	}
	printTrace(tr)
}

// runScan answers "what already exists here?" for an area before planning.
func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	openspecs := fs.String("openspecs", "", "comma-separated list of openspec directories for cross-repo scanning")
	asJSON := fs.Bool("json", false, "emit JSON for a planning agent")
	_ = fs.Parse(args)

	paths, topic := splitArea(fs.Args())
	if len(paths) == 0 && topic == "" {
		fail(fmt.Errorf("scan: give an area — one or more paths and/or a topic\nusage: specsync scan <path...> [topic]"))
	}

	// Determine if this is a cross-repo scan
	var abs []string
	if *openspecs != "" {
		// Cross-repo scan mode
		dirs := strings.Split(*openspecs, ",")
		for _, dir := range dirs {
			d, err := filepath.Abs(strings.TrimSpace(dir))
			if err != nil {
				fail(err)
			}
			abs = append(abs, d)
		}
	} else {
		// Single-repo scan mode
		d, err := filepath.Abs(*openspec)
		if err != nil {
			fail(err)
		}
		abs = append(abs, d)
	}

	ctx := context.Background()
	scope := specsync.Scope{Paths: paths, Topic: topic}

	var in specsync.TraceInput
	if len(abs) == 1 {
		// Single-repo scan (original behavior)
		var err error
		in, err = specsync.GatherTrace(ctx, abs[0], specsync.NewGitCommitSource(), scope)
		if err != nil {
			fail(err)
		}
	} else {
		// Cross-repo scan (new behavior)
		var err error
		in, err = specsync.GatherTraceMulti(ctx, abs, specsync.NewGitCommitSource(), scope)
		if err != nil {
			fail(err)
		}
	}

	tr := specsync.ResolveTrace(in, scope)
	looseIssues, issuesNote := openIssuesInArea(ctx, in, topic)

	// Compute cross-repo relationships for multi-repo scans
	var crossRepo map[string][]specsync.ChangeRelationship
	if len(abs) > 1 {
		crossRepo = specsync.CrossRepoCorrelation(in.Changes, scope)
	}

	if *asJSON {
		emitJSON(map[string]any{"trace": tr, "openIssuesNoChange": looseIssues, "crossRepo": crossRepo})
		return
	}
	label := strings.Join(append(append([]string{}, paths...), quoteIf(topic)...), "  +  ")
	fmt.Printf("Scan  %s\n\n", strings.TrimSpace(label))
	changes := nodesOfKind(tr, specsync.NodeChange)
	if len(changes) == 0 && len(looseIssues) == 0 && len(crossRepo) == 0 {
		fmt.Println("Nothing exists here yet.")
		return
	}
	if len(changes) > 0 {
		fmt.Println("Related changes")
		for _, n := range changes {
			fmt.Printf("  %-32s %s\n", strings.TrimPrefix(n.ID, "change:"), n.Label)
		}
	}

	// Print cross-repo relationships
	if len(crossRepo) > 0 {
		fmt.Println("\nCross-repo relationships")
		for slug, rels := range crossRepo {
			for _, rel := range rels {
				fmt.Printf("  %-32s --[%s]--> %s (%s)\n", slug, rel.Provenance, rel.RelatedChange.Slug, rel.RelatedChange.Title)
			}
		}
	}

	fmt.Println("\nOpen issues here (no linked change)")
	if issuesNote != "" {
		fmt.Printf("  (%s)\n", issuesNote)
	} else if len(looseIssues) == 0 {
		fmt.Println("  (none)")
	}
	for _, it := range looseIssues {
		fmt.Printf("  #%-6s %s\n", it.ID, it.Title)
	}
	if commits := nodesOfKind(tr, specsync.NodeCommit); len(commits) > 0 {
		fmt.Println("\nRecent commits here")
		for _, n := range commits {
			fmt.Printf("  %s\n", n.Label)
		}
	}
}

// openIssuesInArea finds open issues matching the topic that link to no change —
// neither carrying a specsync:change= marker nor bound to a change via the ref
// cache. Returns a note when issues could not be read (no topic, or gh absent),
// so scan degrades visibly rather than silently narrowing.
func openIssuesInArea(ctx context.Context, in specsync.TraceInput, topic string) (loose []specsync.FetchedItem, note string) {
	if topic == "" {
		return nil, "topic needed to search issues; path-only issue scan not yet supported"
	}
	var searcher specsync.IssueSearcher = specsync.NewGitHubProvider()
	found, err := searcher.SearchOpenIssues(ctx, topic)
	if err != nil {
		return nil, "issues not read: gh unavailable"
	}
	linked := map[string]bool{}
	for _, cr := range in.Changes {
		for _, id := range cr.IssueIDs {
			linked[id] = true
		}
	}
	for _, it := range found {
		if linked[it.ID] || strings.Contains(it.Body, "specsync:change=") {
			continue // already linked to a change
		}
		loose = append(loose, it)
	}
	return loose, ""
}

// runReleasePlan prints the read-only follow-up report for a revision range.
func runReleasePlan(args []string) {
	fs := flag.NewFlagSet("release-plan", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	since := fs.String("since", "", "range start (default: latest tag)")
	until := fs.String("until", "", "range end (default: HEAD)")
	asJSON := fs.Bool("json", false, "emit JSON")
	failOnArchiveCandidates := fs.Bool("fail-on-archive-candidates", false, "exit non-zero when shipped completed changes remain unarchived")
	archiveCompleted := fs.Bool("archive-completed", false, "move shipped completed changes from openspec/changes/ to openspec/changes/archive/")
	apply := fs.Bool("apply", false, "perform suggested spec actions (archive completed changes)")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	scope := specsync.Scope{Since: *since, Until: *until}

	src := specsync.NewGitCommitSource()
	in, err := specsync.GatherTrace(ctx, abs, src, scope)
	if err != nil {
		fail(err)
	}
	tr := specsync.ResolveTrace(in, scope)

	// Advisory bump: commit signals × OpenSpec requirement deltas.
	os2 := specsync.NewOpenSpecCLI()
	hasBaseline, _ := os2.HasBaseline(ctx)
	var deltas []specsync.OpenSpecDelta
	shipped := nodesOfKind(tr, specsync.NodeChange)
	statusBySlug := openSpecStatus(ctx, os2)
	for _, n := range shipped {
		slug := strings.TrimPrefix(n.ID, "change:")
		if d, err := os2.Deltas(ctx, slug); err == nil {
			deltas = append(deltas, d...)
		}
	}
	impact := specsync.InferImpact(in.Commits, deltas, hasBaseline, nil)
	archiveCandidates := completedShipped(shipped, statusBySlug)
	if err := archiveHygieneError(archiveCandidates, *failOnArchiveCandidates); err != nil {
		fail(err)
	}

	tool := specsync.DetectReleaseTool(filepath.Dir(abs))
	rng := rangeLabel(*since, *until)

	if *asJSON {
		emitJSON(map[string]any{
			"range": rng, "trace": tr, "bump": impact.Impact.String(),
			"reasons": impact.Reasons, "releaseTool": tool,
			"archiveCandidates": archiveCandidates,
		})
		return
	}

	fmt.Printf("Follow-up  (%s)\n\n", rng)
	fmt.Println("Shipped")
	for _, n := range shipped {
		fmt.Printf("  %-32s %s\n", strings.TrimPrefix(n.ID, "change:"), n.Label)
	}
	if len(shipped) == 0 {
		fmt.Println("  (no linked changes in range)")
	}

	if len(tr.Gaps) > 0 {
		fmt.Println("\nLoose ends")
		for _, g := range tr.Gaps {
			fmt.Printf("  %s  %s (%s)\n", g.Kind, g.Subject, g.Reason)
		}
	}

	if len(archiveCandidates) > 0 {
		fmt.Println("\nArchive candidates  (all tasks done)")
		for _, slug := range archiveCandidates {
			fmt.Printf("  %s\n", slug)
		}
	}

	fmt.Printf("\nAdvisory bump   %s%s\n", impact.Impact.String(), nextVersionSuffix(impact))
	if len(impact.Reasons) > 0 {
		fmt.Println("Why")
		for _, r := range impact.Reasons {
			fmt.Printf("  %s\n", r)
		}
	}

	fmt.Println("\nRelease path (detected)")
	fmt.Printf("  tool: %s", tool.Name)
	if len(tool.Owns) > 0 {
		fmt.Printf("  → owns %s", strings.Join(tool.Owns, ", "))
	}
	fmt.Println("\n  specsync defers to it; the bump above is advisory only")

	if *archiveCompleted {
		archived, err := archiveCompletedChanges(abs, archiveCandidates)
		if err != nil {
			fail(err)
		}
		if len(archived) == 0 {
			fmt.Println("\narchive-completed: no changes archived")
		} else {
			fmt.Println("\nArchived changes")
			for _, slug := range archived {
				fmt.Printf("  %s\n", slug)
			}
		}
	}

	if *apply && !*archiveCompleted {
		fmt.Println("\n--apply preview: archive candidates (execution requires -archive-completed)")
		for _, slug := range archiveCandidates {
			fmt.Printf("  openspec archive %s\n", slug)
		}
	}
}

// resolve gathers and resolves a trace, failing on error.
func resolve(openspecDir string, scope specsync.Scope) specsync.Trace {
	in, err := specsync.GatherTrace(context.Background(), openspecDir, specsync.NewGitCommitSource(), scope)
	if err != nil {
		fail(err)
	}
	return specsync.ResolveTrace(in, scope)
}

func printTrace(tr specsync.Trace) {
	fmt.Println("Nodes")
	for _, n := range tr.Nodes {
		fmt.Printf("  [%s] %s — %s\n", n.Kind, n.ID, n.Label)
	}
	fmt.Println("Links")
	for _, l := range tr.Links {
		fmt.Printf("  %s → %s  (%s)\n", l.From, l.To, l.Provenance)
	}
	if len(tr.Gaps) > 0 {
		fmt.Println("Gaps")
		for _, g := range tr.Gaps {
			fmt.Printf("  %s: %s (%s)\n", g.Kind, g.Subject, g.Reason)
		}
	}
}

func emitJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(b))
}

func nodesOfKind(tr specsync.Trace, kind specsync.NodeKind) []specsync.TraceNode {
	var out []specsync.TraceNode
	for _, n := range tr.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// splitArea sorts scan args into path globs and a topic string. An arg is a path
// when it contains a separator or glob char or names an existing file; the rest
// join into the topic.
func splitArea(args []string) (paths []string, topic string) {
	var topicWords []string
	for _, a := range args {
		if looksLikePath(a) {
			paths = append(paths, a)
		} else {
			topicWords = append(topicWords, a)
		}
	}
	return paths, strings.TrimSpace(strings.Join(topicWords, " "))
}

func looksLikePath(a string) bool {
	if strings.ContainsAny(a, "/*?[") || strings.HasPrefix(a, ".") {
		return true
	}
	if _, err := os.Stat(a); err == nil {
		return true
	}
	return false
}

func quoteIf(topic string) []string {
	if topic == "" {
		return nil
	}
	return []string{fmt.Sprintf("%q", topic)}
}

func rangeLabel(since, until string) string {
	if until == "" {
		until = "HEAD"
	}
	if since == "" {
		since = latestTag()
		if since == "" {
			since = "start"
		}
	}
	return since + ".." + until
}

func latestTag() string {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func nextVersionSuffix(impact specsync.ImpactResult) string {
	tag := latestTag()
	if tag == "" {
		return ""
	}
	v, err := specsync.ParseVersion(tag)
	if err != nil {
		return ""
	}
	next := v.Bump(impact.Impact)
	if next.String() == v.String() {
		return ""
	}
	return fmt.Sprintf("   (%s → %s)", tag, "v"+next.String())
}

// openSpecStatus maps change slug → OpenSpec status, best-effort.
func openSpecStatus(ctx context.Context, o *specsync.OpenSpecCLI) map[string]specsync.OpenSpecChange {
	m := map[string]specsync.OpenSpecChange{}
	changes, err := o.Changes(ctx)
	if err != nil {
		return m
	}
	for _, c := range changes {
		m[c.Name] = c
	}
	return m
}

func completedShipped(shipped []specsync.TraceNode, status map[string]specsync.OpenSpecChange) []string {
	var out []string
	for _, n := range shipped {
		slug := strings.TrimPrefix(n.ID, "change:")
		if c, ok := status[slug]; ok && c.TotalTasks > 0 && c.CompletedTasks == c.TotalTasks {
			out = append(out, slug)
		}
	}
	return out
}

func archiveHygieneError(archiveCandidates []string, failOnArchiveCandidates bool) error {
	if !failOnArchiveCandidates || len(archiveCandidates) == 0 {
		return nil
	}
	return fmt.Errorf("release-plan: %d archive candidate(s) remain unarchived: %s", len(archiveCandidates), strings.Join(archiveCandidates, ", "))
}

func archiveCompletedChanges(openspecDir string, candidates []string) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	archiveRoot := filepath.Join(openspecDir, "changes", "archive")
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return nil, fmt.Errorf("archive-completed: create archive dir: %w", err)
	}
	archived := make([]string, 0, len(candidates))
	for _, slug := range candidates {
		src := filepath.Join(openspecDir, "changes", slug)
		dst := filepath.Join(archiveRoot, slug)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return archived, fmt.Errorf("archive-completed: stat %s: %w", slug, err)
		}
		if _, err := os.Stat(dst); err == nil {
			return archived, fmt.Errorf("archive-completed: destination already exists for %s", slug)
		} else if !os.IsNotExist(err) {
			return archived, fmt.Errorf("archive-completed: check destination for %s: %w", slug, err)
		}
		if err := os.Rename(src, dst); err != nil {
			return archived, fmt.Errorf("archive-completed: move %s to archive: %w", slug, err)
		}
		archived = append(archived, slug)
	}
	return archived, nil
}

// runPRBody emits a PR-body fragment for a change: the provider-specific
// reference line (Part of #N / Closes #N), optionally merged with a user-supplied
// body file. Idempotent — re-running does not stack duplicate lines.
func runPRBody(args []string) {
	fs := flag.NewFlagSet("pr-body", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	change := fs.String("change", "", "change slug (required)")
	bodyFile := fs.String("body-file", "", "file whose contents are merged after the reference line")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	_ = fs.Parse(args)

	if *change == "" {
		fail(fmt.Errorf("pr-body: -change <slug> is required"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	c, err := specsync.LoadChangeBySlug(abs, *change)
	if err != nil {
		fail(err)
	}

	// Build a GitHub provider to resolve the repo and read the ref.
	var prov specsync.WorkProvider = specsync.NewGitHubProvider()
	if *repo != "" {
		prov = specsync.NewGitHubProviderWithRepo(*repo)
	}

	// Resolve the synced ref for this change.
	ctx := context.Background()
	ref, err := resolvePRBodyRef(ctx, prov, c)
	if err != nil {
		fail(err)
	}
	if ref == nil {
		fail(fmt.Errorf("pr-body: change %s has no synced tracker item — run `specsync sync -change %s` first", *change, *change))
	}

	// Determine whether all tasks are complete (same predicate as -close-completed).
	allComplete := specsync.TasksComplete(c.TasksMarkdown)

	// Emit the reference line.
	if ref.ID != "" {
		keyword := "Part of"
		if allComplete {
			keyword = "Closes"
		}
		fmt.Printf("%s #%s\n", keyword, ref.ID)
	}

	// Merge with user body if provided.
	if *bodyFile != "" {
		bodyBytes, err := os.ReadFile(*bodyFile)
		if err != nil {
			fail(err)
		}
		userBody := strings.TrimSpace(string(bodyBytes))
		if userBody != "" {
			fmt.Println()
			fmt.Println(ensureNoDuplicateReference(userBody, allComplete, ref.ID))
		}
	}
}

// resolvePRBodyRef finds the synced ref for a change. It checks the ref cache
// first, then falls back to the provider's Find.
func resolvePRBodyRef(ctx context.Context, prov specsync.WorkProvider, c *specsync.Change) (*specsync.Ref, error) {
	// Try ref cache first.
	refs, err := specsync.LoadRefs(c.Dir)
	if err == nil && len(refs) > 0 {
		key := prov.Name()
		if ref, ok := refs[key]; ok {
			return &ref, nil
		}
		// Legacy bare-"github" key.
		if ref, ok := refs["github"]; ok {
			return &ref, nil
		}
	}

	// Fall back to provider Find.
	found, err := prov.Find(ctx, c.Slug)
	if err != nil {
		return nil, err
	}
	return found, nil
}

// ensureNoDuplicateReference strips any existing Part of / Closes line from body
// so that running pr-body twice never stacks duplicates. It returns the cleaned
// body ready for the reference line to be prepended.
func ensureNoDuplicateReference(body string, allComplete bool, issueID string) string {
	lines := strings.Split(body, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if allComplete {
			// Skip existing "Closes #N" for the same issue.
			if strings.HasPrefix(trimmed, "Closes #"+issueID) ||
				strings.HasPrefix(trimmed, "- [x] Closes #"+issueID) {
				continue
			}
		} else {
			// Skip existing "Part of #N" for the same issue.
			if strings.HasPrefix(trimmed, "Part of #"+issueID) ||
				strings.HasPrefix(trimmed, "- [x] Part of #"+issueID) {
				continue
			}
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// runVerify checks open PRs for traceability gaps: a PR whose head branch
// matches a change slug but whose body has no reference to that change's issue.
func runVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	// Load all changes.
	changes, err := specsync.LoadChanges(abs)
	if err != nil {
		fail(err)
	}

	// Build a map: change slug -> synced issue number.
	type changeIssue struct {
		slug string
		issueNum string
		url  string
	}
	var changeIssues []changeIssue
	for _, c := range changes {
		// Skip archived changes — they don't need PR traceability.
		if c.Archived {
			continue
		}
		refs, err := specsync.LoadRefs(c.Dir)
		if err != nil {
			continue
		}
		// Find the first github ref.
		var ref *specsync.Ref
		for key, r := range refs {
			if strings.HasPrefix(key, "github") || key == "github" {
				ref = &r
				break
			}
		}
		if ref == nil || ref.ID == "" {
			continue
		}
		changeIssues = append(changeIssues, changeIssue{slug: c.Slug, issueNum: ref.ID, url: ref.URL})
	}

	if len(changeIssues) == 0 {
		fmt.Println("verify: no synced changes found — nothing to verify")
		return
	}

	// List open PRs.
	var prov specsync.WorkProvider = specsync.NewGitHubProvider()
	if *repo != "" {
		prov = specsync.NewGitHubProviderWithRepo(*repo)
	}

	gp, ok := prov.(*specsync.GitHubProvider)
	if !ok {
		fail(fmt.Errorf("verify: only GitHub provider is supported"))
	}

	prs, err := gp.ListOpenPRs(context.Background())
	if err != nil {
		fail(err)
	}

	if len(prs) == 0 {
		fmt.Println("verify: no open PRs found")
		return
	}

	warnings := 0
	for _, pr := range prs {
		// Check if the PR's head branch matches any change slug.
		// Branch naming convention: feat/<issue>-<change-slug> or just <change-slug>.
		branch := pr.HeadRefName
		for _, ci := range changeIssues {
			if branchMatchesChange(branch, ci.slug) {
				// This PR belongs to this change. Check if the body references the issue.
				if !prBodyReferencesIssue(pr.Body, ci.issueNum) {
					fmt.Printf("  WARNING: %s (branch %s) — no reference to #%s in PR body\n", pr.URL, branch, ci.issueNum)
					warnings++
				}
			}
		}
	}

	if warnings == 0 {
		fmt.Println("verify: all open PRs on change branches have issue references")
	} else {
		fmt.Printf("\nverify: %d warning(s) found\n", warnings)
	}
}

// branchMatchesChange reports whether a PR head branch name belongs to a change.
// Matches exact slug or common prefixes like "feat/<slug>", "feature/<slug>".
func branchMatchesChange(branch, slug string) bool {
	if branch == slug {
		return true
	}
	// Strip common prefixes.
	for _, prefix := range []string{"feat/", "feature/", "fix/", "chore/", "refactor/"} {
		if strings.HasPrefix(branch, prefix) {
			rest := strings.TrimPrefix(branch, prefix)
			// Slug might be the first component before a slash.
			if idx := strings.Index(rest, "/"); idx >= 0 {
				rest = rest[:idx]
			}
			if rest == slug {
				return true
			}
		}
	}
	return false
}

// prBodyReferencesIssue reports whether body contains a reference to issue number.
// Matches patterns like "#42", "Closes #42", "Part of #42", "Refs #42", etc.
func prBodyReferencesIssue(body, issueNum string) bool {
	// Look for #N pattern where N matches the issue number.
	refPattern := "#" + issueNum
	return strings.Contains(body, refPattern)
}
