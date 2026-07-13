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

// runChangelog emits the spec-driven changelog for a revision range: one entry
// per shipped OpenSpec change (release note or title, Keep a Changelog
// categories), with loose feat/fix commits as honest fallbacks. Read-only by
// default; -apply mutates CHANGELOG.md and defers to a changelog-owning release
// tool unless -force.
func runChangelog(args []string) {
	fs := flag.NewFlagSet("changelog", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	since := fs.String("since", "", "range start (default: latest tag)")
	until := fs.String("until", "", "range end (default: HEAD)")
	versionFlag := fs.String("version", "", "version label for the section (default: latest tag + advisory bump)")
	asJSON := fs.Bool("json", false, "emit JSON")
	releaseNotes := fs.Bool("release-notes", false, "emit the bare section body (for goreleaser --release-notes)")
	apply := fs.Bool("apply", false, "write the section into CHANGELOG.md (idempotent per version)")
	force := fs.Bool("force", false, "apply even when another release tool owns the changelog")
	changelogPath := fs.String("changelog", "CHANGELOG.md", "path of the changelog file -apply writes")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	scope := specsync.Scope{Since: *since, Until: *until}

	in, err := specsync.GatherTrace(ctx, abs, specsync.NewGitCommitSource(), scope)
	if err != nil {
		fail(err)
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

func ownsChangelog(tool specsync.ReleaseTool) bool {
	for _, o := range tool.Owns {
		if o == "changelog" {
			return true
		}
	}
	return false
}

// suggestedVersion is the advisory next version: latest tag bumped by the
// inferred impact. It returns "" — Unreleased semantics — when there is no tag
// to bump from, or when the impact is none: the current tag's section is
// already released history and must never be rewritten by a default.
func suggestedVersion(in specsync.TraceInput, deltas map[string][]specsync.OpenSpecDelta) string {
	tag := latestTag()
	if tag == "" {
		return ""
	}
	v, err := specsync.ParseVersion(tag)
	if err != nil {
		return ""
	}
	var all []specsync.OpenSpecDelta
	for _, d := range deltas {
		all = append(all, d...)
	}
	hasBaseline, _ := specsync.NewOpenSpecCLI().HasBaseline(context.Background())
	impact := specsync.InferImpact(in.Commits, all, hasBaseline, nil)
	next := v.Bump(impact.Impact)
	if next.String() == v.String() {
		return ""
	}
	return next.String()
}
