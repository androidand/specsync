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
	if branch == "HEAD" {
		return UpdatePlan{}, fmt.Errorf("HEAD is detached: check out a branch before updating")
	}
	plan := UpdatePlan{Branch: branch, Conflicts: ls.Conflicts, Shared: ls.SharedBranch}
	plan.Advice = SuggestSync(*ls, SyncInput{UnderReview: underReview}, ways.Sync)
	if plan.Advice != nil && plan.Advice.ForcePush {
		a := *plan.Advice
		a.Summary += "; this rewrites pushed history: pass -force-with-lease (with -push) to publish it"
		plan.Advice = &a
	}
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

// UpdateApply says how far ApplyUpdate may go.
type UpdateApply struct {
	// Push pushes afterwards.
	Push bool
	// ForceWithLease allows publishing a rebase of a pushed branch, with a
	// lease on the remote sha.
	ForceWithLease bool
	// PushToRequest allows pushing the default branch, or a branch with an open
	// or unknown pull request.
	PushToRequest bool
	OpenRequest   func(ctx context.Context, branch string) (int, error)
}

// ApplyUpdate runs the plan. It refuses a detached HEAD and a dirty tree, runs
// only the merge or rebase the plan names, stops at a conflict leaving git's
// own state for the user to resolve, and pushes only when asked. Every push
// refusal happens before anything is changed. A rebase of a pushed branch is
// published only with ForceWithLease, by a lease on the remote sha that is
// fetched first and must already be contained in HEAD, so commits nobody
// integrated are never overwritten; a shared branch is never force-pushed.
func ApplyUpdate(ctx context.Context, dir string, plan UpdatePlan, o UpdateApply) (UpdateResult, error) {
	var res UpdateResult
	a := plan.Advice
	switch {
	case a == nil:
		return res, nil
	case plan.Branch == "" || plan.Branch == "HEAD":
		return res, fmt.Errorf("HEAD is detached: check out a branch before updating")
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
	remote, expected := "", ""
	if o.Push {
		if err := PushGuards(ctx, dir, plan.Branch, o.PushToRequest, o.OpenRequest); err != nil {
			return res, err
		}
		if remote, err = pushRemote(ctx, dir, plan.Branch); err != nil {
			return res, err
		}
		if a.ForcePush {
			if plan.Shared {
				return res, fmt.Errorf("not pushed: the branch is shared, so it is never force-pushed")
			}
			if !o.ForceWithLease {
				return res, fmt.Errorf("nothing changed: this update rewrites pushed history; pass -force-with-lease to allow publishing it")
			}
			if _, err := gitIn(ctx, dir, "fetch", "--quiet", remote, plan.Branch); err != nil {
				return res, fmt.Errorf("nothing changed: could not fetch %s/%s to lease against: %w", remote, plan.Branch, err)
			}
			expected, _ = gitIn(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/remotes/"+remote+"/"+plan.Branch)
			if expected == "" {
				return res, fmt.Errorf("nothing changed: there is no remote-tracking sha for %s/%s to lease against", remote, plan.Branch)
			}
			if _, err := gitIn(ctx, dir, "merge-base", "--is-ancestor", expected, "HEAD"); err != nil {
				return res, fmt.Errorf("nothing changed: %s/%s has commits you do not have; run update again to plan with them", remote, plan.Branch)
			}
		}
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
	if !o.Push {
		return res, nil
	}
	pushArgs := []string{"push"}
	if expected != "" {
		pushArgs = append(pushArgs, "--force-with-lease=refs/heads/"+plan.Branch+":"+expected)
	}
	pushArgs = append(pushArgs, "-u", remote, plan.Branch+":refs/heads/"+plan.Branch)
	if _, err := gitIn(ctx, dir, pushArgs...); err != nil {
		return res, err
	}
	res.Pushed = true
	return res, nil
}
