package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/androidand/specsync"
)

// linkCmd writes links.md for each change (recording the other's issue URL) and
// then syncs each spec so the "## Related" section appears in both GitHub issues.
// Issue references (#N, owner/repo#N, URL) are also accepted — they are linked
// directly without requiring a local spec.
//
// Usage: specsync link [flags] <change1> <change2> [<change3>...]
type linkCmd struct {
	openspec  *string
	storeFlag *string
	dryRun    *bool
	repo      *string
}

func init() { Register(&linkCmd{}) }

func (*linkCmd) Spec() Spec {
	return Spec{
		Name:     "link",
		Group:    GroupPlan,
		Summary:  "Cross-link two or more OpenSpec changes.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "plan", RelatedBefore: []string{"scan", "changes"}, RelatedAfter: []string{"sync"}},
		Safety: []string{
			"At least 2 changes required",
			"Use -dry-run to preview",
			"Links are append-only (existing links not removed)",
		},
		Examples: []string{
			"specsync link my-change other-change -dry-run",
			"specsync link my-change other-change",
			"specsync link change-a change-b change-c",
		},
	}
}

func (cm *linkCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.dryRun = fs.Bool("dry-run", false, "show what would change without writing files or calling GitHub")
	cm.repo = fs.String("repo", "", "repo (owner/name) for bare issue refs")
}

func (cm *linkCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *linkCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, dryRun, repo := cm.openspec, cm.storeFlag, cm.dryRun, cm.repo
	_ = fs.Parse(args)
	checkArgs(fs, args, -1)

	args = fs.Args()
	if len(args) < 2 {
		fail(fmt.Errorf("link: at least 2 arguments required\nusage: specsync link <change1> <change2> [<change3>...]"))
	}

	abs := resolveRoot(fs, openspec, storeFlag).Dir

	result, err := specsync.Link(context.Background(), specsync.LinkOptions{
		OpenSpecDir: abs,
		Args:        args,
		Repo:        *repo,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		for i, p := range result.Pairs {
			fmt.Printf("  %s/links.md would contain:\n", p.Slug)
			for j, other := range result.Pairs {
				if j != i {
					fmt.Printf("    - %s\n", other.Ref.URL)
				}
			}
			for _, lr := range result.Refs {
				fmt.Printf("    - %s\n", lr.Ref.URL)
			}
			// Render the Related section preview by loading the change and
			// injecting the would-be links directly, bypassing disk.
			c, err := specsync.LoadChange(p.Dir, false, abs)
			if err == nil && c != nil {
				c.Links = nil
				for j, other := range result.Pairs {
					if j != i {
						c.Links = append(c.Links, other.Ref)
					}
				}
				for _, lr := range result.Refs {
					c.Links = append(c.Links, lr.Ref)
				}
				item := specsync.WorkItemFor(*c, false)
				if idx := strings.Index(item.Body, "\n\n## Related\n\n"); idx >= 0 {
					fmt.Printf("\n  Related section in %s issue:\n", p.Slug)
					for _, line := range strings.Split(item.Body[idx+2:], "\n") {
						fmt.Println("    " + line)
					}
				}
			}
			fmt.Println()
		}
		// Reference path: show what each issue would be edited to contain.
		for _, lr := range result.Refs {
			var others []specsync.Ref
			for _, p := range result.Pairs {
				others = append(others, p.Ref)
			}
			for _, other := range result.Refs {
				if other.ID != lr.ID || other.Repo != lr.Repo {
					others = append(others, other.Ref)
				}
			}
			relatedBody := specsync.UpsertRelatedSection("(existing issue body)", others)
			fmt.Printf("  %s would be edited:\n", lr.Ref.URL)
			fmt.Printf("    ## Related section:\n")
			for _, line := range strings.Split(relatedBody, "\n") {
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "## Related") {
					fmt.Println("      " + line)
				}
			}
			fmt.Println()
		}

		fmt.Printf("specsync link: would cross-link %d specs and %d issue refs\n", len(result.Pairs), len(result.Refs))
		return
	}

	// Real run: sync each spec with the provider matching its repo.
	for _, p := range result.Pairs {
		provider := makeProvider(p.Repo, false, "github", "")
		_, err := specsync.Sync(context.Background(), specsync.Options{
			OpenSpecDir: abs,
			Provider:    provider,
			Slug:        p.Slug,
		})
		if err != nil {
			fail(fmt.Errorf("sync %s after link: %w", p.Slug, err))
		}
		fmt.Printf("  linked  %s  <->  %s\n", p.Slug, p.Ref.URL)
	}

	// Reference path: fetch each issue, upsert "## Related", push edited body.
	for _, lr := range result.Refs {
		// Collect "other" refs (all refs except this one).
		var others []specsync.Ref
		for _, p := range result.Pairs {
			others = append(others, p.Ref)
		}
		for _, other := range result.Refs {
			if other.ID != lr.ID || other.Repo != lr.Repo {
				others = append(others, other.Ref)
			}
		}

		provider := makeProvider(lr.Repo, false, "github", "")
		if _, err := specsync.PushRelatedEdit(context.Background(), provider, lr.Ref, "", others); err != nil {
			fail(fmt.Errorf("push edited body for %s: %w", lr.Ref.URL, err))
		}
		fmt.Printf("  linked  %s\n", lr.Ref.URL)
	}

	fmt.Printf("specsync link: %d specs and %d refs cross-linked\n", len(result.Pairs), len(result.Refs))
}
