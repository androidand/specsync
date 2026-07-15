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
	asJSON := fs.Bool("json", false, "emit JSON for a planning agent")
	_ = fs.Parse(args)

	paths, topic := splitArea(fs.Args())
	if len(paths) == 0 && topic == "" {
		fail(fmt.Errorf("scan: give an area — one or more paths and/or a topic\nusage: specsync scan <path...> [topic]"))
	}
	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	scope := specsync.Scope{Paths: paths, Topic: topic}
	in, err := specsync.GatherTrace(ctx, abs, specsync.NewGitCommitSource(), scope)
	if err != nil {
		fail(err)
	}
	tr := specsync.ResolveTrace(in, scope)
	looseIssues, issuesNote := openIssuesInArea(ctx, in, topic)

	if *asJSON {
		emitJSON(map[string]any{"trace": tr, "openIssuesNoChange": looseIssues})
		return
	}
	label := strings.Join(append(append([]string{}, paths...), quoteIf(topic)...), "  +  ")
	fmt.Printf("Scan  %s\n\n", strings.TrimSpace(label))
	changes := nodesOfKind(tr, specsync.NodeChange)
	if len(changes) == 0 && len(looseIssues) == 0 {
		fmt.Println("Nothing exists here yet.")
		return
	}
	if len(changes) > 0 {
		fmt.Println("Related changes")
		for _, n := range changes {
			fmt.Printf("  %-32s %s\n", strings.TrimPrefix(n.ID, "change:"), n.Label)
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

	if *apply {
		fmt.Println("\n--apply: spec-archive execution is not yet wired; archive completed changes with:")
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
