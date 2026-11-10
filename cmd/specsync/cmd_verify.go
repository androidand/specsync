package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	refs      *bool
	failStale *bool
}

func init() { Register(&verifyCmd{}) }

func (*verifyCmd) Spec() Spec {
	return Spec{
		Name:     "verify",
		Group:    GroupReview,
		Summary:  "Verify PR references to their change issues, or with -refs the commit and request references in change documents.",
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"pr-body"}},
		Args:     []Arg{{Name: "path", Description: "with -refs: extra markdown files or folders to scan, besides the change folders"}},
		Safety: []string{
			"Run before merging a PR",
			"All open PRs should have issue references",
			"-refs is read-only and advisory: it never rewrites a document or suggests a replacement for a stale reference",
		},
		Examples: []string{
			"specsync verify",
			"specsync verify -refs",
			"specsync verify -refs -fail-on-stale docs/runbook.md",
		},
	}
}

func (cm *verifyCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.store = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "target repo as owner/name (default: auto-detect from git remote)")
	cm.change = fs.String("change", "", "verify this change against its own spec deltas (OpenSpec's Verify phase)")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.checklist = fs.Bool("checklist", false, "print only the acceptance checklist, for pasting into a PR")
	cm.refs = fs.Bool("refs", false, "report commit hashes and request references in change documents (and any paths given) that no longer resolve")
	cm.failStale = fs.Bool("fail-on-stale", false, "with -refs: exit non-zero when a commit reference is stale or orphaned")
}

func (cm *verifyCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *verifyCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, store, repo, change, asJSON, checklist := cm.openspec, cm.store, cm.repo, cm.change, cm.asJSON, cm.checklist
	_ = fs.Parse(args)
	if *cm.refs {
		checkArgs(fs, args, -1)
	} else {
		checkArgs(fs, args, 0)
		if *cm.failStale {
			fail(fmt.Errorf("-fail-on-stale needs -refs"))
		}
	}

	root := resolveRoot(fs, openspec, store)

	if *cm.refs {
		if *change != "" || *checklist || *asJSON {
			fail(fmt.Errorf("-refs cannot be combined with -change, -checklist or -json"))
		}
		repoDir, err := os.Getwd()
		if err != nil {
			fail(err)
		}
		prov := makeProvider(*repo, false, "github", "")
		key := "github"
		if gp, ok := prov.(*specsync.GitHubProvider); ok {
			key = gp.Name()
		}
		stale, err := verifyRefs(context.Background(), os.Stdout, verifyRefsOptions{
			OpenSpecDir: root.Dir, RepoDir: repoDir, Paths: fs.Args(), Provider: prov, DefaultKey: key,
		})
		if err != nil {
			fail(err)
		}
		if stale > 0 && *cm.failStale {
			os.Exit(1)
		}
		return
	}

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
				if !specsync.PRBodyReferencesIssue(pr.Body, ci.issueNum) {
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
