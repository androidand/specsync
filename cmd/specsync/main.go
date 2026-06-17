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
	openspec := flag.String("openspec", "openspec", "path to the openspec/ directory")
	slug := flag.String("slug", "", "sync only this change (default: all changes)")
	dryRun := flag.Bool("dry-run", false, "print the gh commands and rendered issue body without executing")
	flag.Parse()

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
