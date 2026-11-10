package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/androidand/specsync"
)

// runVerifySize prints the size findings for one branch. It is advisory: it
// exits 0 unless failOnOversized is set and a limit is exceeded.
func runVerifySize(fs *flag.FlagSet, openspec, store *string, branch, base, budgetFlag string, exclude []string, failOnOversized bool) {
	ctx := context.Background()
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	head := branch
	if head == "" {
		head = "HEAD"
	}
	if base == "" {
		base, err = specsync.DefaultBase(ctx, dir)
		if err != nil {
			fail(fmt.Errorf("no base to measure against: %w; pass -base <ref>", err))
		}
	}
	sliceLine, from := changeBudgetLine(ctx, fs, openspec, store, dir, head)
	budget, err := specsync.ResolveBudget(budgetFlag, sliceLine, os.Getenv(specsync.BudgetEnv))
	if err != nil && sliceLine != "" {
		fmt.Fprintf(os.Stderr, "specsync: budget line of change %s ignored: %v\n", from, err)
		budget, err = specsync.ResolveBudget(budgetFlag, "", os.Getenv(specsync.BudgetEnv))
	}
	if err != nil {
		fail(fmt.Errorf("size budget: %w", err))
	}
	size, scopes, err := specsync.AnalyzeBranch(ctx, dir, base, head, exclude)
	if err != nil {
		fail(fmt.Errorf("measure %s against %s: %w", head, base, err))
	}
	exceeded := budget.Exceeded(size)
	if sliceLine != "" {
		fmt.Printf("budget: from the budget line of change %s unless -budget overrides it\n", from)
	}

	fmt.Printf("size: %d files, +%d -%d (%d lines), %d commits, %s against %s\n",
		size.Files, size.Additions, size.Deletions, size.Additions+size.Deletions, size.Commits, head, base)
	if len(exceeded) > 0 {
		fmt.Printf("oversized: %s\n", strings.Join(exceeded, ", "))
	}
	if len(scopes) > 1 {
		fmt.Printf("mixed-scopes: %s\n", strings.Join(scopes, ", "))
	}
	if len(exceeded) == 0 && len(scopes) < 2 {
		fmt.Println("within budget, no mixed scopes")
	}
	if failOnOversized && len(exceeded) > 0 {
		os.Exit(1)
	}
}

var branchIssue = regexp.MustCompile(`^(?:[^/]+/)?(\d+)-`)

// changeBudgetLine returns the budget line of the one change the branch
// belongs to, or "" when there is none.
func changeBudgetLine(ctx context.Context, fs *flag.FlagSet, openspec, store *string, dir, head string) (line, slug string) {
	c := matchedChange(ctx, fs, openspec, store, dir, head)
	if c == nil {
		return "", ""
	}
	return specsync.BudgetLineForBranch(c.Dir, branchName(ctx, dir, head)), c.Slug
}

// branchName resolves head to a branch name; "HEAD" means the checked-out one.
func branchName(ctx context.Context, dir, head string) string {
	if head != "HEAD" {
		return head
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// matchedChange finds the one change a branch belongs to, by slug or by issue
// number in its name, as status does. With no specs, or no single match, it
// returns nil.
func matchedChange(ctx context.Context, fs *flag.FlagSet, openspec, store *string, dir, head string) *specsync.Change {
	root := resolveRootUnchecked(fs, openspec, store)
	if root.Validate() != nil {
		return nil
	}
	changes, err := specsync.LoadChanges(root.Dir)
	if err != nil {
		return nil
	}
	name := branchName(ctx, dir, head)
	if name == "" {
		return nil
	}
	issue := ""
	if m := branchIssue.FindStringSubmatch(name); m != nil {
		issue = m[1]
	}
	var matches []specsync.Change
	for _, c := range changes {
		if c.Archived {
			continue
		}
		hit := branchMatchesChange(name, c.Slug)
		if !hit && issue != "" {
			if refs, err := specsync.LoadRefs(c.Dir); err == nil {
				for _, r := range refs {
					hit = hit || r.ID == issue
				}
			}
		}
		if hit {
			matches = append(matches, c)
		}
	}
	if len(matches) != 1 {
		return nil
	}
	return &matches[0]
}
