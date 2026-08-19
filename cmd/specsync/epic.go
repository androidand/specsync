package main

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// runEpic implements `specsync epic <title> [--repo owner/name] [--child
// <slug|owner/repo#N|url>]...`: mints (or converges onto) a `type:epic`
// coordination issue and wires every child — a local change slug or an
// existing issue reference, possibly cross-repo — to it. See
// openspec/changes/epic-scaffold-command for the full design.
func runEpic(args []string) {
	fs := flag.NewFlagSet("epic", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	repo := fs.String("repo", "", "target repo for the epic issue, as owner/name (default: auto-detect from git remote)")
	var children stringSlice
	fs.Var(&children, "child", "a child to attach: local change slug, owner/repo#N, bare #N (resolved against --repo), or issue URL (repeatable)")
	dryRun := fs.Bool("dry-run", false, "print what would happen without creating or editing any issue")
	_ = fs.Parse(args)

	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		fail(fmt.Errorf("epic: usage: specsync epic <title> [--repo owner/name] [--child <slug|owner/repo#N|url>]..."))
	}

	ctx := context.Background()

	// Resolve --repo: explicit flag → gh's own auto-detect, same convention
	// relate's --repo already uses.
	targetRepo := *repo
	if targetRepo == "" {
		if out, err := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner").CombinedOutput(); err == nil {
			targetRepo = strings.TrimSpace(string(out))
		}
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	epicProvider := makeProvider(targetRepo, *dryRun, "github", "")

	// providerCache avoids re-resolving (and re-shelling to gh) the same
	// repo's provider once per child.
	providerCache := map[string]specsync.WorkProvider{}
	childProviderFor := func(childRepo string) specsync.WorkProvider {
		if p, ok := providerCache[childRepo]; ok {
			return p
		}
		p := makeProvider(childRepo, *dryRun, "github", "")
		providerCache[childRepo] = p
		return p
	}

	res, err := specsync.Epic(ctx, specsync.EpicOptions{
		OpenSpecDir:      abs,
		Title:            title,
		Repo:             targetRepo,
		Children:         children,
		DryRun:           *dryRun,
		EpicProvider:     epicProvider,
		ChildProviderFor: childProviderFor,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no issues were created or edited")
		fmt.Println()
	}

	verb := "updated"
	if res.Created {
		verb = "created"
	}
	fmt.Printf("epic     %-8s %s\n", verb, res.Ref.URL)
	for _, c := range res.Children {
		note := ""
		if c.Synced {
			note = " (synced)"
		}
		fmt.Printf("  child  %s%s\n", c.Ref.URL, note)
	}
	fmt.Printf("specsync epic: %d children wired\n", len(res.Children))
}
