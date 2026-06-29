// Command specsync projects OpenSpec changes into a work tracker (GitHub today).
// It is a standalone, single-binary tool that works in any OpenSpec project,
// regardless of the project's own language.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

func main() {
	// Subcommands: "pull" reads an issue into a local change; "link" cross-links
	// two or more specs; the default (no subcommand, or "sync") projects changes
	// outward to issues.
	args := os.Args[1:]
	switch {
	case len(args) > 0 && args[0] == "pull":
		runPull(args[1:])
	case len(args) > 0 && args[0] == "link":
		runLink(args[1:])
	case len(args) > 0 && args[0] == "scan":
		runScan(args[1:])
	case len(args) > 0 && args[0] == "trace":
		runTrace(args[1:])
	case len(args) > 0 && args[0] == "release-plan":
		runReleasePlan(args[1:])
	case len(args) > 0 && args[0] == "sync":
		runSync(args[1:])
	default:
		runSync(args)
	}
}

// runSync projects every OpenSpec change into the tracker (spec -> issue).
func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	slug := fs.String("slug", "", "sync only this change (default: all changes)")
	repo := fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	dryRun := fs.Bool("dry-run", false, "print the gh commands and rendered issue body without executing")
	reconcile := fs.Bool("reconcile", true, "merge issue checkbox state back into tasks.md before pushing")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	provider := makeProvider(*repo, *dryRun)
	if *dryRun {
		fmt.Println("DRY RUN — no GitHub calls are made")
		if *repo != "" {
			fmt.Printf("target: %s\n", *repo)
		} else {
			fmt.Println("target: auto-detected from the current repo's git remote")
		}
		fmt.Println()
	}

	res, err := specsync.Sync(context.Background(), specsync.Options{
		OpenSpecDir: abs,
		Provider:    provider,
		Slug:        *slug,
		DryRun:      *dryRun,
		Reconcile:   *reconcile,
	})
	if err != nil {
		fail(err)
	}
	if *dryRun && *reconcile {
		fmt.Println("(reconcile applies on a real sync — dry-run makes no issue reads)")
	}
	fmt.Println()
	for _, it := range res.Items {
		verb := "updated"
		if it.Created {
			verb = "created"
		}
		fmt.Printf("  %-8s %s  (%s)\n", verb, it.URL, it.Slug)
		for _, f := range it.Flips {
			state := "unchecked"
			if f.Checked {
				state = "checked"
			}
			fmt.Printf("           ↳ reconciled from issue: %s → %s\n", f.Text, state)
		}
	}
	fmt.Printf("specsync: %d created, %d updated\n", res.Created, res.Updated)
}

// runPull reads an existing issue and materializes it as a local change
// (issue -> spec). A dry run reads the issue but writes nothing to disk.
func runPull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	issue := fs.String("issue", "", "issue number to pull into a local change (required)")
	slug := fs.String("slug", "", "change slug (default: derived from the issue title)")
	repo := fs.String("repo", "", "source repo as owner/name (default: auto-detect from git remote)")
	dryRun := fs.Bool("dry-run", false, "show what would be written without touching disk")
	_ = fs.Parse(args)

	if strings.TrimSpace(*issue) == "" {
		fail(fmt.Errorf("pull: -issue is required"))
	}
	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	res, err := specsync.Pull(context.Background(), specsync.PullOptions{
		OpenSpecDir: abs,
		Provider:    makeProvider(*repo, false),
		IssueID:     *issue,
		Slug:        *slug,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	dest := filepath.Join("openspec", "changes", res.Slug)
	if *dryRun {
		fmt.Printf("DRY RUN — would write %s from issue %s\n\n", dest, *issue)
		printPreview("proposal.md", res.Proposal)
		if res.Tasks != "" {
			printPreview("tasks.md", res.Tasks)
		}
		return
	}
	fmt.Printf("specsync: pulled issue %s -> %s\n", *issue, dest)
	fmt.Println("  + proposal.md")
	if res.Tasks != "" {
		fmt.Println("  + tasks.md")
	}
}

// runLink writes links.md for each slug (recording the other's issue URL) and
// then syncs each spec so the "## Related" section appears in both GitHub issues.
//
// Usage: specsync link [flags] <slug1> <slug2> [slug3...]
func runLink(args []string) {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	openspec := fs.String("openspec", "openspec", "path to the openspec/ directory")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing files or calling GitHub")
	_ = fs.Parse(args)

	slugs := fs.Args()
	if len(slugs) < 2 {
		fail(fmt.Errorf("link: at least 2 slugs required\nusage: specsync link <slug1> <slug2> [slug3...]"))
	}

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	pairs, err := specsync.Link(specsync.LinkOptions{
		OpenSpecDir: abs,
		Slugs:       slugs,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}

	if *dryRun {
		fmt.Println("DRY RUN — no files or GitHub calls will be modified")
		fmt.Println()
		for i, p := range pairs {
			fmt.Printf("  %s/links.md would contain:\n", p.Slug)
			for j, other := range pairs {
				if j != i {
					fmt.Printf("    - %s\n", other.Ref.URL)
				}
			}
			// Render the Related section preview by loading the change and
			// injecting the would-be links directly, bypassing disk.
			c, err := specsync.LoadChange(p.Dir, false, abs)
			if err == nil && c != nil {
				c.Links = nil
				for j, other := range pairs {
					if j != i {
						c.Links = append(c.Links, other.Ref)
					}
				}
				item := specsync.WorkItemFor(*c)
				if idx := strings.Index(item.Body, "\n\n## Related\n\n"); idx >= 0 {
					fmt.Printf("\n  Related section in %s issue:\n", p.Slug)
					for _, line := range strings.Split(item.Body[idx+2:], "\n") {
						fmt.Println("    " + line)
					}
				}
			}
			fmt.Println()
		}
		fmt.Printf("specsync link: would cross-link %d specs\n", len(pairs))
		return
	}

	// Real run: sync each spec with the provider matching its repo.
	for _, p := range pairs {
		provider := makeProvider(p.Repo, false)
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
	fmt.Printf("specsync link: %d specs cross-linked\n", len(pairs))
}

// makeProvider returns a dry-runner for dry runs, or the real GitHub provider
// targeting repo (auto-detect when empty).
func makeProvider(repo string, dryRun bool) specsync.WorkProvider {
	if dryRun {
		return specsync.NewGitHubProviderFuncWithRepo(repo, dryRunner)
	}
	if repo != "" {
		return specsync.NewGitHubProviderWithRepo(repo)
	}
	return specsync.NewGitHubProvider()
}

func printPreview(name, content string) {
	fmt.Println("  " + name)
	fmt.Println("    ┌───────────────────────────")
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		fmt.Println("    │ " + line)
	}
	fmt.Println("    └───────────────────────────")
}

// dryRunner prints what would run instead of calling gh, returning canned output
// so the orchestration proceeds through the create path.
func dryRunner(_ context.Context, args ...string) (string, error) {
	var inline []string
	var body string
	for i := 0; i < len(args); i++ {
		if args[i] == "--body" && i+1 < len(args) {
			body = args[i+1]
			inline = append(inline, "--body", "«see below»")
			i++
			continue
		}
		inline = append(inline, args[i])
	}
	fmt.Println("  $ gh " + shellJoin(inline))
	if body != "" {
		fmt.Println("    ┌─ issue body ──────────────")
		for _, line := range strings.Split(body, "\n") {
			fmt.Println("    │ " + line)
		}
		fmt.Println("    └───────────────────────────")
	}

	switch {
	case len(args) >= 2 && args[0] == "issue" && args[1] == "list":
		return "[]", nil // pretend no existing issue
	case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
		return "https://github.com/<owner>/<repo>/issues/0", nil
	case len(args) >= 2 && args[0] == "issue" && args[1] == "view":
		return `{"labels":[]}`, nil
	default:
		return "", nil
	}
}

func shellJoin(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if a == "" || strings.ContainsAny(a, " \t\n\"'") {
			b.WriteString(fmt.Sprintf("%q", a))
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "specsync:", err)
	os.Exit(1)
}
