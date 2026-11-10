package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/androidand/specsync"
)

// verifyCmd checks open PRs for traceability gaps: a PR whose head branch
// matches a change slug but whose body has no reference to that change's issue.
type verifyCmd struct {
	openspec  *string
	store     *string
	repo      *string
	change    *string
	asJSON    *bool
	checklist *bool
}

func init() { Register(&verifyCmd{}) }

func (*verifyCmd) Spec() Spec {
	return Spec{
		Name:     "verify",
		Group:    GroupReview,
		Summary:  "Verify PR references to their change issues.",
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"pr-body"}},
		Safety: []string{
			"Run before merging a PR",
			"All open PRs should have issue references",
		},
		Examples: []string{
			"specsync verify",
		},
	}
}

func (cm *verifyCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.store = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.change = fs.String("change", "", "verify this change against its own spec deltas (OpenSpec's Verify phase)")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.checklist = fs.Bool("checklist", false, "print only the acceptance checklist, for pasting into a PR")
}

func (cm *verifyCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *verifyCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, store, repo, change, asJSON, checklist := cm.openspec, cm.store, cm.repo, cm.change, cm.asJSON, cm.checklist
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	root := resolveRoot(fs, openspec, store)

	// -change verifies one change against its behaviour contract; without it,
	// verify sweeps the release for change -> issue -> PR traceability.
	if *change != "" || *checklist {
		if *change == "" {
			fail(fmt.Errorf("-checklist needs -change <slug>"))
		}
		runVerifySpec(root, *change, *asJSON, *checklist)
		return
	}

	abs := root.Dir

	// Load all changes.
	changes, err := specsync.LoadChanges(abs)
	if err != nil {
		fail(err)
	}

	// Build a map: change slug -> synced issue number.
	type changeIssue struct {
		slug     string
		issueNum string
		url      string
	}
	var changeIssues []changeIssue
	for _, c := range changes {
		// Skip archived changes — they don't need PR traceability.
		if c.Archived {
			continue
		}
		refs, err := specsync.LoadRefs(c.Dir)
		if err != nil {
			continue
		}
		// Find the first github ref.
		var ref *specsync.Ref
		for key, r := range refs {
			if strings.HasPrefix(key, "github") || key == "github" {
				ref = &r
				break
			}
		}
		if ref == nil || ref.ID == "" {
			continue
		}
		changeIssues = append(changeIssues, changeIssue{slug: c.Slug, issueNum: ref.ID, url: ref.URL})
	}

	if len(changeIssues) == 0 {
		fmt.Println("verify: no synced changes found — nothing to verify")
		return
	}

	// List open PRs.
	var prov specsync.WorkProvider = specsync.NewGitHubProvider()
	if *repo != "" {
		prov = specsync.NewGitHubProviderWithRepo(*repo)
	}

	gp, ok := prov.(*specsync.GitHubProvider)
	if !ok {
		fail(fmt.Errorf("verify: only GitHub provider is supported"))
	}

	prs, err := gp.ListOpenPRs(context.Background())
	if err != nil {
		fail(err)
	}

	if len(prs) == 0 {
		fmt.Println("verify: no open PRs found")
		return
	}

	warnings := 0
	for _, pr := range prs {
		// Check if the PR's head branch matches any change slug.
		// Branch naming convention: feat/<issue>-<change-slug> or just <change-slug>.
		branch := pr.HeadRefName
		for _, ci := range changeIssues {
			if branchMatchesChange(branch, ci.slug) {
				// This PR belongs to this change. Check if the body references the issue.
				if !prBodyReferencesIssue(pr.Body, ci.issueNum) {
					fmt.Printf("  WARNING: %s (branch %s) — no reference to #%s in PR body\n", pr.URL, branch, ci.issueNum)
					warnings++
				}
			}
		}
	}

	if warnings == 0 {
		fmt.Println("verify: all open PRs on change branches have issue references")
	} else {
		fmt.Printf("\nverify: %d warning(s) found\n", warnings)
	}
}
