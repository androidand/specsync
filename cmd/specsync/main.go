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
	// Subcommands: "pull" reads an issue into a local change; the default (no
	// subcommand, or "sync") projects changes outward to issues.
	args := os.Args[1:]
	switch {
	case len(args) > 0 && args[0] == "pull":
		runPull(args[1:])
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
	dryRun := fs.Bool("dry-run", false, "print the gh commands and rendered issue body without executing")
	_ = fs.Parse(args)

	abs, err := filepath.Abs(*openspec)
	if err != nil {
		fail(err)
	}

	provider := specsync.NewGitHubProvider()
	if *dryRun {
		provider = specsync.NewGitHubProviderFunc(dryRunner)
		fmt.Println("DRY RUN — no GitHub calls are made")
		fmt.Println()
	}

	res, err := specsync.Sync(context.Background(), specsync.Options{
		OpenSpecDir: abs,
		Provider:    provider,
		Slug:        *slug,
		DryRun:      *dryRun,
	})
	if err != nil {
		fail(err)
	}
	fmt.Println()
	for _, it := range res.Items {
		verb := "updated"
		if it.Created {
			verb = "created"
		}
		fmt.Printf("  %-8s %s  (%s)\n", verb, it.URL, it.Slug)
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
		Provider:    specsync.NewGitHubProvider(),
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
