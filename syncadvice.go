package specsync

// SyncAdvice is a suggestion for bringing a branch up to date. It is text:
// specsync never runs it. Applying it is the job of `specsync update`.
type SyncAdvice struct {
	Action    string   `json:"action"`
	Summary   string   `json:"summary"`
	Commands  []string `json:"commands,omitempty"`
	ForcePush bool     `json:"forcePush"`
}

// SyncInput is the provider fact the advice needs beyond local git state.
type SyncInput struct {
	ReviewStarted bool // the request has any review or comment
}

// SyncConfig is the `sync:` section of openspec/specsync.yml. Zero values mean
// the documented defaults: strategy rebase, after_review merge, force with-lease.
type SyncConfig struct {
	Strategy    string // "rebase" (default) or "merge"
	AfterReview string // "merge" (default) or "rebase"
	Force       string // "with-lease" (default) or "never"
}

// SuggestSync applies the documented table over the freshness facts, first
// match wins. It returns nil when there is nothing to do or no base to name.
//
//	dirty tree                    commit or stash first; nothing else
//	only behind upstream          fast-forward
//	diverged from upstream        rebase local commits onto upstream; no force
//	behind base, shared branch    merge the base; never force
//	behind base, review started   after_review (default merge)
//	behind base, otherwise        strategy (default rebase); force with lease if pushed
func SuggestSync(ls LocalState, in SyncInput, cfg SyncConfig) *SyncAdvice {
	switch {
	case ls.Modified+ls.Untracked > 0:
		return &SyncAdvice{Action: "commit-or-stash", Summary: "commit or stash your changes first"}
	case ls.UpstreamBehind > 0 && ls.Ahead == 0:
		return &SyncAdvice{Action: "fast-forward", Summary: "fast-forward to the upstream",
			Commands: []string{"git merge --ff-only @{u}"}}
	case ls.UpstreamBehind > 0 && ls.Ahead > 0:
		return &SyncAdvice{Action: "rebase-onto-upstream", Summary: "rebase your commits onto the upstream; no force push needed",
			Commands: []string{"git rebase @{u}"}}
	case ls.BehindBase > 0 && ls.Base != "":
		return baseAdvice(ls, in, cfg)
	}
	return nil
}

func baseAdvice(ls LocalState, in SyncInput, cfg SyncConfig) *SyncAdvice {
	merge := &SyncAdvice{Action: "merge-base", Commands: []string{"git merge " + ls.Base}}
	switch {
	case ls.SharedBranch:
		merge.Summary = "merge " + ls.Base + " (others share this branch); no force push needed"
		return merge
	case in.ReviewStarted:
		if cfg.AfterReview != "rebase" {
			merge.Summary = "merge " + ls.Base + " (review has started); no force push needed"
			return merge
		}
	default:
		if cfg.Strategy == "merge" {
			merge.Summary = "merge " + ls.Base + "; no force push needed"
			return merge
		}
	}
	// Rebasing. A pushed branch then needs a force push, which the config may forbid.
	pushed := ls.Upstream != ""
	if pushed && cfg.Force == "never" {
		merge.Summary = "merge " + ls.Base + " (force pushes are disabled); no force push needed"
		return merge
	}
	rebase := &SyncAdvice{Action: "rebase-onto-base", Commands: []string{"git rebase " + ls.Base}}
	if pushed {
		rebase.Commands = append(rebase.Commands, "git push --force-with-lease")
		rebase.ForcePush = true
		rebase.Summary = "rebase onto " + ls.Base + "; a force push with lease is needed because the branch is pushed"
	} else {
		rebase.Summary = "rebase onto " + ls.Base + "; no force push needed, the branch was never pushed"
	}
	return rebase
}
