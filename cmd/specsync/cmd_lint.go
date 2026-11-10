package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/androidand/specsync"
)

// lintCmd checks titles and text against the repository's text standard. It
// reports and suggests; it never edits anything.
type lintCmd struct {
	openspec  *string
	storeFlag *string
	change    *string
	title     *string
	kind      *string
	bodyFile  *string
	pr        *string
	repo      *string
	fail      *bool
	asJSON    *bool
}

func init() { Register(&lintCmd{}) }

func (*lintCmd) Spec() Spec {
	return Spec{
		Name:     "lint",
		Group:    GroupReview,
		Summary:  "Check titles and issue or PR text against the short-text standard.",
		Intents:  []string{"keep issues short", "check my PR title", "conventional commits"},
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"pr-body"}},
		Safety: []string{
			"Advisory: exits 0 unless -fail is given; never edits text",
			"The standard is the default or the text: section of openspec/specsync.yml",
		},
		Examples: []string{
			"specsync lint -change my-change",
			"specsync lint -title 'fix(sync): keep links out of sections' -fail",
			"specsync lint -pr 42 -fail",
			"specsync lint -body-file body.md",
		},
	}
}

func (cm *lintCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "lint this change's title and proposal as an issue")
	cm.title = fs.String("title", "", "lint this title")
	cm.kind = fs.String("kind", "", "what -title and -body-file are: pr, commit or issue (default: pr)")
	cm.bodyFile = fs.String("body-file", "", "lint the text in this file")
	cm.pr = fs.String("pr", "", "lint the title and body of this pull request number")
	cm.repo = fs.String("repo", "", "with -pr: repo as owner/name (default: auto-detect from git remote)")
	cm.fail = fs.Bool("fail", false, "exit non-zero when anything is reported, for CI")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

type lintReport struct {
	Source string `json:"source"`
	specsync.LintFinding
}

var ghPRText = func(number, repo string) (title, body string, err error) {
	args := []string{"pr", "view", number, "--json", "title,body"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return "", "", fmt.Errorf("gh pr view %s: %w", number, err)
	}
	var v struct{ Title, Body string }
	if err := json.Unmarshal(out, &v); err != nil {
		return "", "", err
	}
	return v.Title, v.Body, nil
}

func (cm *lintCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	if *cm.change == "" && *cm.title == "" && *cm.bodyFile == "" && *cm.pr == "" {
		fail(fmt.Errorf("lint: pass -change <slug>, -title <text>, -body-file <file> or -pr <number>"))
	}
	kind := *cm.kind
	if kind != "" && kind != "pr" && kind != "commit" && kind != "issue" {
		fail(fmt.Errorf("lint: -kind must be pr, commit or issue"))
	}
	if kind == "" {
		kind = "pr"
	}

	repoRoot := "."
	var change *specsync.Change
	if *cm.change != "" {
		resolved := resolveRoot(fs, cm.openspec, cm.storeFlag)
		c, err := specsync.LoadChangeBySlug(resolved.Dir, *cm.change)
		if err != nil {
			fail(err)
		}
		change = c
		repoRoot = filepath.Dir(resolved.Dir)
	}
	cfg, issues := specsync.LoadWays(repoRoot)
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "specsync: %s has problems (%s); run specsync doctor config\n", specsync.SpecSyncConfigPath, issues[0])
	}
	text := cfg.Text

	var reports []lintReport
	add := func(source string, fs []specsync.LintFinding) {
		for _, f := range fs {
			reports = append(reports, lintReport{Source: source, LintFinding: f})
		}
	}
	if change != nil {
		add("title", specsync.LintTitle("issue", change.Title, text))
		add(filepath.Join(change.Slug, "proposal.md"), specsync.LintDocument("issue", change.Body, text))
	}
	if *cm.title != "" {
		add("title", specsync.LintTitle(kind, *cm.title, text))
	}
	if *cm.bodyFile != "" {
		b, err := os.ReadFile(*cm.bodyFile)
		if err != nil {
			fail(err)
		}
		docKind := "pr"
		if kind == "issue" {
			docKind = "issue"
		}
		add(*cm.bodyFile, specsync.LintDocument(docKind, string(b), text))
	}
	if *cm.pr != "" {
		title, body, err := ghPRText(*cm.pr, *cm.repo)
		if err != nil {
			fail(err)
		}
		add("pr #"+*cm.pr+" title", specsync.LintTitle("pr", title, text))
		add("pr #"+*cm.pr+" body", specsync.LintDocument("pr", body, text))
	}

	if *cm.asJSON {
		if reports == nil {
			reports = []lintReport{}
		}
		emitJSON(reports)
	} else {
		printLint(reports)
	}
	if *cm.fail && len(reports) > 0 {
		os.Exit(1)
	}
	return nil
}

func printLint(reports []lintReport) {
	if len(reports) == 0 {
		fmt.Println("lint: the text follows the standard")
		return
	}
	for _, r := range reports {
		fmt.Printf("%s:%d  %s  %s\n", r.Source, r.Line, r.Rule, r.Message)
		if r.Suggestion != "" {
			fmt.Printf("    suggestion: %s\n", strings.TrimSpace(r.Suggestion))
		}
	}
	fmt.Printf("lint: %d finding(s); advisory unless -fail is given\n", len(reports))
}
