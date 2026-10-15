package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/androidand/specsync"
)

// changelogCmd emits the spec-driven changelog for a revision range: one entry
// per shipped OpenSpec change (release note or title, Keep a Changelog
// categories), with loose feat/fix commits as honest fallbacks. Read-only by
// default; -apply mutates CHANGELOG.md and defers to a changelog-owning release
// tool unless -force.
type changelogCmd struct {
	openspec              *string
	storeFlag             *string
	since                 *string
	until                 *string
	versionFlag           *string
	asJSON                *bool
	releaseNotes          *bool
	apply                 *bool
	force                 *bool
	changelogPath         *string
	resolveRefs           *bool
	repo                  *string
	providerName          *string
	failOnUnlinkedCommits *bool
}

func init() { Register(&changelogCmd{}) }

func (*changelogCmd) Spec() Spec {
	return Spec{
		Name:     "changelog",
		Group:    GroupReview,
		Summary:  "Generate release notes from OpenSpec changes and commits.",
		Workflow: AgentCommandWorkflow{Position: "release", RelatedBefore: []string{"release-plan"}},
		Safety: []string{
			"Read-only operation",
			"Requires git history",
		},
		Examples: []string{
			"specsync changelog",
			"specsync changelog -resolve-refs",
		},
	}
}

func (cm *changelogCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.since = fs.String("since", "", "range start (default: latest tag)")
	cm.until = fs.String("until", "", "range end (default: HEAD)")
	cm.versionFlag = fs.String("version", "", "version label for the section (default: latest tag + advisory bump)")
	cm.asJSON = fs.Bool("json", false, "emit JSON")
	cm.releaseNotes = fs.Bool("release-notes", false, "emit the bare section body (for goreleaser --release-notes)")
	cm.apply = fs.Bool("apply", false, "write the section into CHANGELOG.md (idempotent per version)")
	cm.force = fs.Bool("force", false, "apply even when another release tool owns the changelog")
	cm.changelogPath = fs.String("changelog", "CHANGELOG.md", "path of the changelog file -apply writes")
	cm.resolveRefs = fs.Bool("resolve-refs", false, "look up a change's issue live (read-only) when its ref cache is missing, e.g. on a fresh CI checkout")
	cm.repo = fs.String("repo", "", "repo as owner/name for -resolve-refs (default: auto-detect from git remote)")
	cm.providerName = fs.String("provider", "github", "work provider for -resolve-refs: github (default) or beads")
	cm.failOnUnlinkedCommits = fs.Bool("fail-on-unlinked-commits", false, "exit non-zero when a conventional commit in range isn't linked to its change's issue (would otherwise render as a raw title+hash fallback)")
}

func (cm *changelogCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *changelogCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, since, until, versionFlag, asJSON, releaseNotes, apply, force, changelogPath, resolveRefs, repo, providerName, failOnUnlinkedCommits := cm.openspec, cm.storeFlag, cm.since, cm.until, cm.versionFlag, cm.asJSON, cm.releaseNotes, cm.apply, cm.force, cm.changelogPath, cm.resolveRefs, cm.repo, cm.providerName, cm.failOnUnlinkedCommits
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir
	ctx := context.Background()
	scope := specsync.Scope{Since: *since, Until: *until}

	in, err := specsync.GatherTrace(ctx, abs, specsync.NewGitCommitSource(), scope)
	if err != nil {
		fail(err)
	}
	if *resolveRefs {
		provider := makeProvider(*repo, false, *providerName, "")
		if err := specsync.ResolveLiveRefs(ctx, &in, provider); err != nil {
			fail(fmt.Errorf("resolve-refs: %w", err))
		}
	}

	// Requirement deltas, best-effort and only for changes that actually
	// shipped in the range (mirrors release-plan): a pending change's deltas
	// must not color this release's categories or advisory version.
	os2 := specsync.NewOpenSpecCLI()
	shipped := nodesOfKind(specsync.ResolveTrace(in, scope), specsync.NodeChange)
	deltas := map[string][]specsync.OpenSpecDelta{}
	for _, n := range shipped {
		slug := strings.TrimPrefix(n.ID, "change:")
		if d, err := os2.Deltas(ctx, slug); err == nil && len(d) > 0 {
			deltas[slug] = d
		}
	}

	cl := specsync.BuildChangelog(in, deltas)

	if err := unlinkedCommitsError(cl, *failOnUnlinkedCommits); err != nil {
		fail(err)
	}

	version := strings.TrimPrefix(strings.TrimSpace(*versionFlag), "v")
	if version == "" {
		version = suggestedVersion(in, deltas)
	}
	date := time.Now().Format("2006-01-02")
	section := specsync.RenderChangelogSection(cl, version, date)

	switch {
	case *asJSON:
		emitJSON(map[string]any{
			"range": rangeLabel(*since, *until), "version": version, "date": date,
			"entries": cl.Entries, "omittedCommits": cl.OmittedCommits,
			"section": section,
		})
	case *releaseNotes:
		// Bare body: strip the version heading so goreleaser owns the title.
		body := section
		if i := strings.Index(body, "\n"); i >= 0 {
			body = strings.TrimLeft(body[i+1:], "\n")
		}
		fmt.Print(body)
	default:
		fmt.Printf("Changelog  (%s)\n\n", rangeLabel(*since, *until))
		fmt.Println(section)
	}

	if !*apply {
		return
	}
	tool := specsync.DetectReleaseTool(filepath.Dir(abs))
	if ownsChangelog(tool) && !*force {
		fail(fmt.Errorf("changelog: %s owns the changelog (evidence: %s); it stays in charge — re-run with -force to override",
			tool.Name, strings.Join(tool.Evidence, ", ")))
	}
	if err := specsync.ApplyChangelog(*changelogPath, version, section); err != nil {
		fail(err)
	}
	label := version
	if label == "" {
		label = "Unreleased"
	}
	// Status goes to stderr so -json stdout stays a single valid document.
	fmt.Fprintf(os.Stderr, "specsync: wrote [%s] section to %s\n", label, *changelogPath)
}
