package specsync

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// PrepOptions describes one `specsync pr prep`.
type PrepOptions struct {
	Dir  string
	Ways Ways
	// Change is the change the branch belongs to, nil when none matched.
	Change *Change
	// BudgetFlag and BudgetLine are the flag and the change or slice budget
	// line; ResolveBudget layers them with SPECSYNC_BUDGET.
	BudgetFlag, BudgetLine string
	Exclude                []string
	// analyze measures the branch; nil means AnalyzeBranch.
	analyze func(ctx context.Context, dir, base, head string, exclude []string) (*BranchSize, []string, error)
}

// PrepCheck is one readiness check. Status is pass, fail or skipped.
type PrepCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	// Waivable is true only for a real over-budget result, which the caller may
	// acknowledge; analysis and configuration errors are never waivable.
	Waivable bool `json:"waivable,omitempty"`
}

// PrepReport is the outcome of the checks. Passed is true when none failed.
type PrepReport struct {
	Checks []PrepCheck `json:"checks"`
	Passed bool        `json:"passed"`
	Head   string      `json:"head,omitempty"`
}

// PreparePR checks that the branch is ready for a pull request: a clean tree,
// tasks done, current with its base, gates stamped for HEAD, and within the
// size budget. It reads and changes nothing.
func PreparePR(ctx context.Context, o PrepOptions) (PrepReport, error) {
	var rep PrepReport
	head, err := gitIn(ctx, o.Dir, "rev-parse", "HEAD")
	if err != nil {
		return rep, err
	}
	rep.Head = head
	add := func(name, status, detail string) {
		rep.Checks = append(rep.Checks, PrepCheck{Name: name, Status: status, Detail: detail})
	}
	analyze := o.analyze
	if analyze == nil {
		analyze = AnalyzeBranch
	}

	if st, err := gitIn(ctx, o.Dir, "status", "--porcelain", "--untracked-files=normal"); err != nil {
		add("tree", "fail", "could not read the working tree: "+err.Error())
	} else if st != "" {
		add("tree", "fail", "uncommitted changes: commit them with specsync save")
	} else {
		add("tree", "pass", "clean")
	}

	switch {
	case o.Change == nil:
		add("tasks", "skipped", "no change matched this branch")
	default:
		total, done := CountCheckboxes(o.Change.TasksMarkdown)
		switch {
		case total == 0:
			add("tasks", "fail", "no tasks listed in "+o.Change.Slug+"/tasks.md")
		case done < total:
			add("tasks", "fail", fmt.Sprintf("%d of %d tasks done in %s", done, total, o.Change.Slug))
		default:
			add("tasks", "pass", fmt.Sprintf("%d of %d tasks done", done, total))
		}
	}

	var size *BranchSize
	var sizeErr error
	ls, err := CollectLocalState(ctx, o.Dir, LocalStateOptions{})
	switch {
	case err != nil:
		add("fresh", "fail", err.Error())
	case ls.Base == "":
		add("fresh", "skipped", "no base branch to compare with")
	case ls.BehindBase > 0:
		add("fresh", "fail", fmt.Sprintf("%d commits behind %s: bring it current with specsync update", ls.BehindBase, ls.Base))
	case len(ls.Conflicts) > 0:
		add("fresh", "fail", "would conflict with "+ls.Base+" in "+strings.Join(ls.Conflicts, ", "))
	default:
		add("fresh", "pass", "current with "+ls.Base)
	}

	switch {
	case len(o.Ways.Gates) == 0:
		add("gates", "skipped", "no gates configured")
	case GatesPassedAt(ctx, o.Dir, head):
		add("gates", "pass", "gates passed for this commit")
	default:
		add("gates", "fail", "gates not run for HEAD: run specsync gates")
	}

	if ls != nil && ls.Base != "" {
		size, _, sizeErr = analyze(ctx, o.Dir, ls.Base, "HEAD", o.Exclude)
	}
	budget, berr := ResolveBudget(o.BudgetFlag, o.BudgetLine, os.Getenv(BudgetEnv))
	switch {
	case berr != nil:
		add("size", "fail", berr.Error())
	case sizeErr != nil:
		add("size", "fail", "could not measure: "+sizeErr.Error())
	case size == nil:
		add("size", "skipped", "no base branch to measure against")
	default:
		if over := budget.Exceeded(size); len(over) > 0 {
			add("size", "fail", "over budget ("+strings.Join(over, ", ")+"): split it, see specsync slices")
			rep.Checks[len(rep.Checks)-1].Waivable = true
		} else {
			add("size", "pass", fmt.Sprintf("%d files, +%d -%d, %d commits", size.Files, size.Additions, size.Deletions, size.Commits))
		}
	}

	rep.Passed = true
	for _, c := range rep.Checks {
		if c.Status == "fail" {
			rep.Passed = false
		}
	}
	return rep, nil
}
