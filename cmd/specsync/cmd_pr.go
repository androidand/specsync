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
	"slices"
	"strings"

	specsync "github.com/androidand/specsync"
)

type prCmd struct {
	openspec, storeFlag *string
	change, budget      *string
	exclude             stringSlice
	confirm             stringSlice
	title, bodyFile     *string
	base                *string
	draft, dryRun       *bool
	asJSON              *bool
}

func init() { Register(&prCmd{}) }

func (*prCmd) Spec() Spec {
	return Spec{
		Name:     "pr",
		Group:    GroupReview,
		Summary:  "Check a branch is ready for a pull request (prep) and open one with a short generated body (open).",
		Intents:  []string{"open a pull request", "is my branch ready for review"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"gates", "update"}, RelatedAfter: []string{"status"}},
		Args:     []Arg{{Name: "verb", Description: "prep (check, read-only) or open (check, then create the pull request)", Required: true}},
		Safety: []string{
			"prep reads and changes nothing; it checks a clean tree, tasks done, current with the base, gates stamped for HEAD and the size budget",
			"open refuses unless prep passes, the branch is pushed and every pr.confirm entry from openspec/specsync.yml is given with -confirm <name>",
			"-confirm oversized acknowledges only a failed size check; no other failed check can be waived",
			"open never pushes; -dry-run prints the title, body and command and creates nothing",
		},
		Examples: []string{
			"specsync pr prep",
			"specsync pr open -dry-run",
			"specsync pr open -confirm tests-run -confirm manual-test",
		},
	}
}

func (cm *prCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.change = fs.String("change", "", "the change this branch belongs to (default: found from the branch name)")
	cm.budget = fs.String("budget", "", "size budget override, e.g. files=15,lines=400,commits=30")
	fs.Var(&cm.exclude, "exclude", "glob of paths left out of the size counts (repeatable)")
	fs.Var(&cm.confirm, "confirm", "open: a confirmation named in pr.confirm (repeatable), or oversized")
	cm.title = fs.String("title", "", "open: the title (default: the change's title)")
	cm.bodyFile = fs.String("body-file", "", "open: a body file to use instead of the generated one")
	cm.base = fs.String("base", "", "open: the base branch (default: the repository's default branch)")
	cm.draft = fs.Bool("draft", false, "open: create the pull request as a draft")
	cm.dryRun = fs.Bool("dry-run", false, "open: show what would be created and create nothing")
	cm.asJSON = fs.Bool("json", false, "prep: output as JSON")
}

func (cm *prCmd) Run(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "prep" && args[0] != "open") {
		return fmt.Errorf("usage: specsync pr prep or open (prep or open)")
	}
	verb := args[0]
	fs := flagSetFor(cm)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	checkArgs(fs, args[1:], 0)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*cm.openspec)
	ways := loadWaysOrFail(filepath.Dir(abs))
	var change *specsync.Change
	if *cm.change != "" {
		change = mutableChange(resolveRoot(fs, cm.openspec, cm.storeFlag).Dir, *cm.change, false)
	} else {
		change = matchedChange(ctx, fs, cm.openspec, cm.storeFlag, cwd, "HEAD")
	}
	line := ""
	if change != nil {
		line = specsync.BudgetLineForBranch(change.Dir, branchName(ctx, cwd, "HEAD"))
	}
	rep, err := specsync.PreparePR(ctx, specsync.PrepOptions{Dir: cwd, Ways: ways, Change: change, BudgetFlag: *cm.budget, BudgetLine: line, Exclude: cm.exclude})
	if err != nil {
		return err
	}
	if verb == "prep" {
		if *cm.asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(rep)
		} else {
			printPrep(rep)
		}
		if !rep.Passed {
			os.Exit(1)
		}
		return nil
	}
	return cm.open(ctx, cwd, ways, change, rep)
}

func printPrep(rep specsync.PrepReport) {
	for _, c := range rep.Checks {
		fmt.Printf("  %-8s %-8s %s\n", c.Status, c.Name, c.Detail)
	}
	if rep.Passed {
		fmt.Println("specsync: ready for a pull request")
	} else {
		fmt.Println("specsync: not ready for a pull request")
	}
}

func (cm *prCmd) open(ctx context.Context, cwd string, ways specsync.Ways, change *specsync.Change, rep specsync.PrepReport) error {
	var blocked []string
	for _, c := range rep.Checks {
		if c.Status != "fail" {
			continue
		}
		if c.Name == "size" && slices.Contains(cm.confirm, "oversized") {
			continue
		}
		blocked = append(blocked, c.Name+": "+c.Detail)
	}
	if len(blocked) > 0 {
		printPrep(rep)
		return fmt.Errorf("not opening a pull request: %s", strings.Join(blocked, "; "))
	}
	var missing []string
	for _, want := range ways.PR.Confirm {
		if !slices.Contains(cm.confirm, want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing confirmation: %s (pass -confirm <name> for each once it is true)", strings.Join(missing, ", "))
	}
	ls, err := specsync.CollectLocalState(ctx, cwd, specsync.LocalStateOptions{})
	if err != nil {
		return err
	}
	branch := branchName(ctx, cwd, "HEAD")
	if ls.Upstream == "" || ls.Ahead > 0 || ls.UpstreamGone {
		return fmt.Errorf("%s is not pushed: push it first (specsync save -push), then open", branch)
	}
	base := *cm.base
	if base == "" {
		b, err := specsync.DefaultBase(ctx, cwd)
		if err != nil {
			return fmt.Errorf("cannot tell the base branch (%v); pass -base", err)
		}
		base = strings.TrimPrefix(b, "origin/")
	}
	title, body, err := cm.titleAndBody(ctx, change)
	if err != nil {
		return err
	}
	args := []string{"pr", "create", "--title", title, "--body-file", "-", "--base", base, "--head", branch}
	if *cm.draft {
		args = append(args, "--draft")
	}
	if *cm.dryRun {
		fmt.Printf("DRY RUN — would run: gh %s\ntitle: %s\n--- body ---\n%s\n", strings.Join(args, " "), title, body)
		return nil
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(body)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh pr create: %v: %s", err, strings.TrimSpace(errb.String()))
	}
	fmt.Print(out.String())
	return nil
}

func (cm *prCmd) titleAndBody(ctx context.Context, change *specsync.Change) (title, body string, err error) {
	title = *cm.title
	if title == "" && change != nil {
		title = change.Title
	}
	if title == "" {
		return "", "", fmt.Errorf("no title: pass -title, or run from a branch that matches a change")
	}
	if *cm.bodyFile != "" {
		b, err := os.ReadFile(*cm.bodyFile)
		return title, string(b), err
	}
	if change == nil {
		return "", "", fmt.Errorf("no change matched this branch: pass -body-file")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	gen := exec.CommandContext(ctx, exe, "pr-body", "-change", change.Slug, "-template", "-openspec", *cm.openspec)
	var out, errb bytes.Buffer
	gen.Stdout, gen.Stderr = &out, &errb
	if err := gen.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "specsync: the body has no closing reference (%s)\n", strings.TrimSpace(errb.String()))
		abs, _ := filepath.Abs(*cm.openspec)
		repoRoot := filepath.Dir(abs)
		ways, _ := specsync.LoadWays(repoRoot)
		_, tpl, terr := specsync.FindPRTemplate(repoRoot, ways.Text)
		if terr != nil {
			return "", "", terr
		}
		return title, specsync.RenderPRBody(specsync.PRBodyInput{Change: *change, Template: tpl, Size: branchSizeLine(ctx, repoTop(repoRoot)), Text: ways.Text}), nil
	}
	return title, out.String(), nil
}
