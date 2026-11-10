package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/androidand/specsync"
)

// runVerifySize prints the size findings for one branch. It is advisory: it
// exits 0 unless failOnOversized is set and a limit is exceeded.
func runVerifySize(branch, base, budgetFlag string, exclude []string, failOnOversized bool) {
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
	budget, err := specsync.ResolveBudget(budgetFlag, "", os.Getenv(specsync.BudgetEnv))
	if err != nil {
		fail(err)
	}
	size, scopes, err := specsync.AnalyzeBranch(ctx, dir, base, head, exclude)
	if err != nil {
		fail(fmt.Errorf("measure %s against %s: %w", head, base, err))
	}
	exceeded := budget.Exceeded(size)

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
