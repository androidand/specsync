package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	specsync "github.com/androidand/specsync"
)

type newCmd struct {
	openspec, storeFlag *string
	repo                *string
	anyway, yes, dryRun *bool
}

func init() { Register(&newCmd{}) }

func (*newCmd) Spec() Spec {
	return Spec{
		Name:     "new",
		Group:    GroupPlan,
		Summary:  "Create a change from a short template, after showing likely duplicates; the issue is created only on -yes.",
		Intents:  []string{"start a new piece of work", "create a change", "write a new issue"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "issue-first-start", RelatedAfter: []string{"start", "pull"}},
		Args:     []Arg{{Name: "title", Description: "what the change delivers, as one short sentence", Required: true}},
		Safety: []string{
			"Searches local changes and open issues for the title's key words first and stops when it finds likely duplicates, unless -anyway is given",
			"Writes only openspec/changes/<slug>/ (proposal and tasks); it never creates the openspec folder: run specsync init first in a repo with none",
			"Creates nothing in the tracker until you have seen the text and pass -yes; -dry-run creates nothing at all",
		},
		Examples: []string{
			"specsync new 'Show a clean summary in status' -dry-run",
			"specsync new 'Show a clean summary in status'",
			"specsync new 'Show a clean summary in status' -yes",
		},
	}
}

func (cm *newCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "repository to search for duplicate issues as owner/name (default: the current repository)")
	cm.anyway = fs.Bool("anyway", false, "create the change even though likely duplicates were found")
	cm.yes = fs.Bool("yes", false, "after showing the text, create the tracker issue (specsync -change <slug>)")
	cm.dryRun = fs.Bool("dry-run", false, "show the duplicates and the text and create nothing")
}

func (cm *newCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	args = reorderKnownFlags(fs, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, -1)
	title := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if title == "" {
		return fmt.Errorf("usage: specsync new <title>")
	}
	root := resolveRoot(fs, cm.openspec, cm.storeFlag)
	changes, err := specsync.LoadChanges(root.Dir)
	if err != nil {
		return err
	}
	var cands []specsync.DuplicateCandidate
	for _, c := range changes {
		if !c.Archived {
			cands = append(cands, specsync.DuplicateCandidate{Kind: "change", Ref: c.Slug, Title: c.Title})
		}
	}
	issues, searchErr := searchIssues(ctx, *cm.repo, title)
	cands = append(cands, issues...)
	dups := specsync.FindDuplicates(title, cands)
	if searchErr != nil {
		fmt.Fprintf(os.Stderr, "specsync: could not search open issues (%v); only local changes were compared\n", searchErr)
	}
	if len(dups) > 0 {
		fmt.Println("Likely duplicates:")
		for _, d := range dups {
			fmt.Printf("  %-6s %s  %s %s\n", d.Kind, d.Ref, d.Title, d.URL)
		}
		if !*cm.anyway {
			return fmt.Errorf("nothing created: look at the duplicates above; pass -anyway if this is a different change")
		}
	}
	slug := specsync.Slugify(title)
	if *cm.dryRun {
		fmt.Printf("DRY RUN — would create openspec/changes/%s/ (proposal.md, tasks.md)\n", slug)
		return nil
	}
	slug, err = specsync.ScaffoldChange(root.Dir, title)
	if err != nil {
		return err
	}
	fmt.Printf("specsync: created openspec/changes/%s/\n", slug)
	proposal, _ := os.ReadFile(filepath.Join(root.Dir, "changes", slug, "proposal.md"))
	fmt.Printf("--- issue text (from proposal.md) ---\n%s---\n", proposal)
	if !*cm.yes {
		fmt.Printf("The tracker issue was not created. Fill in the text, then: specsync -dry-run -change %s, and specsync -change %s (or run this again with -yes once the text is right)\n", slug, slug)
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	sync := exec.CommandContext(ctx, exe, "-change", slug, "-openspec", *cm.openspec)
	sync.Stdout, sync.Stderr = os.Stdout, os.Stderr
	return sync.Run()
}

func searchIssues(ctx context.Context, repo, title string) ([]specsync.DuplicateCandidate, error) {
	words := specsync.KeyWords(title)
	if len(words) == 0 {
		return nil, nil
	}
	args := []string{"issue", "list", "--state", "open", "--search", strings.Join(words, " "), "--json", "number,title,url", "--limit", "10"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh issue list: %s", strings.TrimSpace(errb.String()))
	}
	var rows []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		return nil, err
	}
	var cands []specsync.DuplicateCandidate
	for _, r := range rows {
		cands = append(cands, specsync.DuplicateCandidate{Kind: "issue", Ref: fmt.Sprintf("#%d", r.Number), Title: r.Title, URL: r.URL})
	}
	return cands, nil
}
