package specsync

import (
	"context"
	"fmt"
	"strings"
)

// UpdatePlan is what `specsync update` would do. Advice is nil when the
// branch is already current.
type UpdatePlan struct {
	Branch    string      `json:"branch"`
	Advice    *SyncAdvice `json:"advice,omitempty"`
	Conflicts []string    `json:"conflicts,omitempty"` // predicted, merge only
	Shared    bool        `json:"shared,omitempty"`
}

// UpdateResult reports an applied plan.
type UpdateResult struct {
	Applied   bool     `json:"applied"`
	Pushed    bool     `json:"pushed"`
	Conflicts []string `json:"conflicts,omitempty"`
}

// PlanUpdate reads the branch's freshness and applies the sync table and the
// repo's sync: config. It changes nothing.
func PlanUpdate(ctx context.Context, dir string, ways Ways, underReview bool) (UpdatePlan, error) {
	ls, err := CollectLocalState(ctx, dir, LocalStateOptions{})
	if err != nil {
		return UpdatePlan{}, err
	}
	branch, _ := gitIn(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	plan := UpdatePlan{Branch: branch, Conflicts: ls.Conflicts, Shared: ls.SharedBranch}
	plan.Advice = SuggestSync(*ls, SyncInput{UnderReview: underReview}, ways.Sync)
	return plan, nil
}

func gitStep(cmd string) ([]string, bool) {
	f := strings.Fields(cmd)
	if len(f) < 2 || f[0] != "git" {
		return nil, false
	}
	switch f[1] {
	case "merge", "rebase":
		return f[1:], true
	}
	return nil, false
}

// ApplyUpdate runs the plan. It refuses a dirty tree, runs only the merge or
// rebase the plan names, stops at a conflict leaving git's own state for the
// user to resolve, and pushes only when push is set: with a lease after a
// rebase of a pushed branch, plainly otherwise, and never a force on a shared
// branch.
func ApplyUpdate(ctx context.Context, dir string, plan UpdatePlan, push bool) (UpdateResult, error) {
	var res UpdateResult
	a := plan.Advice
	switch {
	case a == nil:
		return res, nil
	case a.Action == "commit-or-stash":
		return res, fmt.Errorf("the working tree has changes: commit or stash them first (specsync save), then update")
	}
	st, err := gitIn(ctx, dir, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return res, fmt.Errorf("could not read the working tree state, so nothing was changed: %w", err)
	}
	if st != "" {
		return res, fmt.Errorf("the working tree has changes: commit or stash them first (specsync save), then update")
	}
	for _, c := range a.Commands {
		args, ok := gitStep(c)
		if !ok {
			continue
		}
		if _, err := gitIn(ctx, dir, args...); err != nil {
			files, _ := gitIn(ctx, dir, "diff", "--name-only", "--diff-filter=U")
			for _, f := range strings.Split(files, "\n") {
				if f != "" {
					res.Conflicts = append(res.Conflicts, f)
				}
			}
			if len(res.Conflicts) == 0 {
				return res, err
			}
			abort := "git " + args[0] + " --abort"
			return res, fmt.Errorf("stopped at a conflict in %s; resolve it and continue (git %s --continue), or back out with %s", strings.Join(res.Conflicts, ", "), args[0], abort)
		}
	}
	res.Applied = true
	if !push {
		return res, nil
	}
	pushArgs := []string{"push"}
	if a.ForcePush {
		if plan.Shared {
			return res, fmt.Errorf("not pushed: the branch is shared, so it is never force-pushed")
		}
		pushArgs = append(pushArgs, "--force-with-lease")
	}
	if _, err := gitIn(ctx, dir, pushArgs...); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
}
