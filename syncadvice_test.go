package specsync

import (
	"reflect"
	"testing"
)

func TestSuggestSync_Table(t *testing.T) {
	pushed := LocalState{Upstream: "origin/feat", Base: "origin/main"}
	with := func(f func(*LocalState)) LocalState {
		ls := pushed
		f(&ls)
		return ls
	}
	cases := []struct {
		name       string
		ls         LocalState
		in         SyncInput
		wantAction string
		wantCmds   []string
		wantForce  bool
	}{
		{"dirty tree wins over everything", with(func(l *LocalState) { l.Modified = 1; l.BehindBase = 3; l.UpstreamBehind = 1 }),
			SyncInput{}, "commit-or-stash", nil, false},
		{"untracked files count as dirty", with(func(l *LocalState) { l.Untracked = 1; l.BehindBase = 3 }),
			SyncInput{}, "commit-or-stash", nil, false},
		{"only behind upstream", with(func(l *LocalState) { l.UpstreamBehind = 2 }),
			SyncInput{}, "fast-forward", []string{"git merge --ff-only @{u}"}, false},
		{"diverged from upstream", with(func(l *LocalState) { l.UpstreamBehind = 1; l.Ahead = 1 }),
			SyncInput{}, "rebase-onto-upstream", []string{"git rebase @{u}"}, false},
		{"base moved, shared branch: merge, never force", with(func(l *LocalState) { l.BehindBase = 2; l.SharedBranch = true }),
			SyncInput{}, "merge-base", []string{"git merge origin/main"}, false},
		{"base moved, review started: merge keeps comments attached", with(func(l *LocalState) { l.BehindBase = 2 }),
			SyncInput{ReviewStarted: true}, "merge-base", []string{"git merge origin/main"}, false},
		{"base moved, private and pushed: rebase with lease", with(func(l *LocalState) { l.BehindBase = 2 }),
			SyncInput{}, "rebase-onto-base", []string{"git rebase origin/main", "git push --force-with-lease"}, true},
		{"base moved, private and never pushed: rebase, no force", LocalState{Base: "origin/main", BehindBase: 2},
			SyncInput{}, "rebase-onto-base", []string{"git rebase origin/main"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SuggestSync(c.ls, c.in, SyncConfig{})
			if got == nil {
				t.Fatal("expected a suggestion")
			}
			if got.Action != c.wantAction || !reflect.DeepEqual(got.Commands, c.wantCmds) || got.ForcePush != c.wantForce {
				t.Fatalf("got %+v, want action=%s commands=%v force=%v", got, c.wantAction, c.wantCmds, c.wantForce)
			}
			if got.Summary == "" {
				t.Fatal("a suggestion needs a summary")
			}
		})
	}
}

func TestSuggestSync_UpToDateSuggestsNothing(t *testing.T) {
	if got := SuggestSync(LocalState{Upstream: "origin/f", Base: "origin/main"}, SyncInput{}, SyncConfig{}); got != nil {
		t.Fatalf("up to date should suggest nothing, got %+v", got)
	}
	if got := SuggestSync(LocalState{}, SyncInput{}, SyncConfig{}); got != nil {
		t.Fatalf("no facts should suggest nothing, got %+v", got)
	}
}

func TestSuggestSync_SharedBranchIsNeverForcePushedWhateverTheConfig(t *testing.T) {
	ls := LocalState{Upstream: "origin/f", Base: "origin/main", BehindBase: 1, SharedBranch: true}
	for _, cfg := range []SyncConfig{
		{Strategy: "rebase", AfterReview: "rebase", Force: "with-lease"},
		{Strategy: "rebase"},
		{Strategy: "merge"},
	} {
		got := SuggestSync(ls, SyncInput{ReviewStarted: true}, cfg)
		if got.ForcePush || got.Action != "merge-base" {
			t.Fatalf("shared branch with %+v: %+v", cfg, got)
		}
	}
}

func TestSuggestSync_ConfigChangesTheStrategy(t *testing.T) {
	ls := LocalState{Upstream: "origin/f", Base: "origin/main", BehindBase: 1}

	got := SuggestSync(ls, SyncInput{}, SyncConfig{Strategy: "merge"})
	if got.Action != "merge-base" || got.ForcePush {
		t.Fatalf("strategy merge: %+v", got)
	}

	got = SuggestSync(ls, SyncInput{ReviewStarted: true}, SyncConfig{AfterReview: "rebase"})
	if got.Action != "rebase-onto-base" || !got.ForcePush {
		t.Fatalf("after_review rebase: %+v", got)
	}

	got = SuggestSync(ls, SyncInput{}, SyncConfig{Force: "never"})
	if got.Action != "merge-base" || got.ForcePush {
		t.Fatalf("force never must fall back to merge, not suggest a force push: %+v", got)
	}
}

func TestSuggestSync_NoBaseNoBaseAdvice(t *testing.T) {
	if got := SuggestSync(LocalState{Upstream: "origin/f", BehindBase: 3}, SyncInput{}, SyncConfig{}); got != nil {
		t.Fatalf("without a base name there is no command to give: %+v", got)
	}
}
