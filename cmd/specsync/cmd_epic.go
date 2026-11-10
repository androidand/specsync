package main

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"github.com/androidand/specsync"
)

// epicCmd implements `specsync epic <title> [--repo owner/name] [--child
// <slug|owner/repo#N|url>]...`: mints (or converges onto) a `type:epic`
// coordination issue and wires every child — a local change slug or an
// existing issue reference, possibly cross-repo — to it. See
// openspec/changes/epic-scaffold-command for the full design.
type epicCmd struct {
	openspec  *string
	storeFlag *string
	repo      *string
	children  stringSlice
	dryRun    *bool
}

func init() { Register(&epicCmd{}) }

func (*epicCmd) Spec() Spec {
	return Spec{
		Name:     "epic",
		Group:    GroupPlan,
		Summary:  "Create a coordination issue and wire children to it.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "plan", RelatedBefore: []string{"changes"}, RelatedAfter: []string{"sync"}},
		Safety: []string{
			"Use -dry-run to preview before creating or editing issues",
			"Re-running with the same title converges onto the existing epic instead of duplicating it",
		},
		Examples: []string{
			"specsync epic 'Q3 auth rework' --child my-change --child other-org/other-repo#42 --dry-run",
			"specsync epic 'Q3 auth rework' --child my-change",
		},
	}
}

func (cm *epicCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "target repo for the epic issue, as owner/name (default: auto-detect from git remote)")
	fs.Var(&cm.children, "child", "a child to attach: local change slug, owner/repo#N, bare #N (resolved against --repo), or issue URL (repeatable)")
	cm.dryRun = fs.Bool("dry-run", false, "print what would happen without creating or editing any issue")
}

func (cm *epicCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *epicCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, repo, dryRun := cm.openspec, cm.storeFlag, cm.repo, cm.dryRun
	_ = fs.Parse(args)
	children := cm.children
	checkArgs(fs, args, -1) // the title is a free-form positional

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

	abs := resolveRoot(fs, openspec, storeFlag).Dir

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
