package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/specsync"
)

// unlinkedCommitsError reports entries that reached the raw fallback:
// conventional commits with no change bound (Slug == "") that carry a hash
// (Hash != ""). These are exactly the entries looseEntry produces for a
// commit whose message has no recognized issue reference — see extractRefs
// in commit.go. A properly-linked change entry has Slug set and no Hash; a
// silently-omitted non-conventional loose commit never becomes an entry at
// all, so it can't appear here and isn't this check's concern.
func unlinkedCommitsError(cl specsync.Changelog, failOnUnlinkedCommits bool) error {
	if !failOnUnlinkedCommits {
		return nil
	}
	var offenders []string
	for _, e := range cl.Entries {
		if e.Hash != "" && e.Slug == "" {
			offenders = append(offenders, fmt.Sprintf("%s (%s)", e.Text, e.Hash))
		}
	}
	if len(offenders) == 0 {
		return nil
	}
	return fmt.Errorf("changelog: %d commit(s) not linked to a change's issue: %s — each needs the issue number of the change it ships, in one of two forms. Either reference it in the commit message (e.g. \"(#42)\", \"Closes #42\") so this renders as an authored release note instead of a raw commit line, or — when the commit message is not yours to edit, as with a squash-merge — add a \"## Release note\" section to the change's proposal.md, which is the only remedy available in that case. Note that a \"(#N)\" in a squash-merge header is often the PR number rather than the change's issue; that is why 131-style refs above can look present and still be unlinked. If the number is the change's issue, \"(#42)\" does bind and nothing else is needed",
		len(offenders), strings.Join(offenders, ", "))
}

func ownsChangelog(tool specsync.ReleaseTool) bool {
	for _, o := range tool.Owns {
		if o == "changelog" {
			return true
		}
	}
	return false
}

// suggestedVersion is the advisory next version: latest tag bumped by the
// inferred impact. It returns "" — Unreleased semantics — when there is no tag
// to bump from, or when the impact is none: the current tag's section is
// already released history and must never be rewritten by a default.
func suggestedVersion(in specsync.TraceInput, deltas map[string][]specsync.OpenSpecDelta) string {
	tag := latestTag()
	if tag == "" {
		return ""
	}
	v, err := specsync.ParseVersion(tag)
	if err != nil {
		return ""
	}
	var all []specsync.OpenSpecDelta
	for _, d := range deltas {
		all = append(all, d...)
	}
	hasBaseline, _ := specsync.NewOpenSpecCLI().HasBaseline(context.Background())
	impact := specsync.InferImpact(in.Commits, all, hasBaseline, nil)
	next := v.Bump(impact.Impact)
	if next.String() == v.String() {
		return ""
	}
	return next.String()
}
