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

	specsync "github.com/androidand/specsync"
)

type updateCmd struct {
	openspec      *string
	apply         *bool
	push          *bool
	pushToRequest *bool
	fetch         *bool
	asJSON        *bool
}

func init() { Register(&updateCmd{}) }

func (*updateCmd) Spec() Spec {
	return Spec{
		Name:     "update",
		Group:    GroupPlan,
		Summary:  "Bring your branch up to date with its base or upstream: a plan first, then -apply.",
		Intents:  []string{"rebase my branch", "merge main into my branch", "get current"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"status"}, RelatedAfter: []string{"gates"}},
		Safety: []string{
			"Without -apply it prints the plan and changes nothing",
			"Refuses a dirty working tree; never force-pushes a shared branch (those are merged)",
			"Pushes only with -push, by explicit refspec to the branch's own name, and after a rebase of a pushed branch only with an explicit lease on the sha it saw",
			"An unknown or open pull request state counts as under review: the plan merges, and -push then needs -push-to-request",
			"Stops at a conflict, names the files and leaves git's own merge or rebase state for you to resolve or abort",
			"The plan uses the sync: strategy from openspec/specsync.yml; run with -fetch to refresh remote branches first",
		},
		Examples: []string{
			"specsync update",
			"specsync update -fetch -apply",
			"specsync update -apply -push",
		},
	}
}

func (cm *updateCmd) Flags(fs *flag.FlagSet) {
	cm.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	cm.apply = fs.Bool("apply", false, "run the plan (without it nothing changes)")
	cm.push = fs.Bool("push", false, "with -apply, push afterwards (with a lease after a rebase)")
	cm.pushToRequest = fs.Bool("push-to-request", false, "with -push, allow pushing when the branch has an open pull request or that could not be checked")
	cm.fetch = fs.Bool("fetch", false, "run git fetch first so the plan sees current remote branches")
	cm.asJSON = fs.Bool("json", false, "output the plan as JSON")
}

func (cm *updateCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	if *cm.push && !*cm.apply {
		return fmt.Errorf("-push needs -apply")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*cm.openspec)
	ways := loadWaysOrFail(filepath.Dir(abs))
	if *cm.fetch {
		if out, err := exec.CommandContext(ctx, "git", "-C", cwd, "fetch", "--quiet").CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	branch, _ := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD").Output()
	openN, lookupErr := openRequestFor("")(ctx, strings.TrimSpace(string(branch)))
	underReview := lookupErr != nil || openN > 0
	plan, err := specsync.PlanUpdate(ctx, cwd, ways, underReview)
	if err != nil {
		return err
	}
	if *cm.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(plan); err != nil {
			return err
		}
	} else {
		printUpdatePlan(plan, *cm.apply)
	}
	if !*cm.apply {
		return nil
	}
	if *cm.push && !*cm.pushToRequest {
		switch {
		case lookupErr != nil:
			return fmt.Errorf("not applied: cannot tell whether a pull request is open for %s (%v); pass -push-to-request to push anyway, or update without -push", plan.Branch, lookupErr)
		case openN > 0:
			return fmt.Errorf("not applied: %s has an open pull request (#%d), and a push notifies its reviewers; pass -push-to-request to push anyway, or update without -push", plan.Branch, openN)
		}
	}
	res, err := specsync.ApplyUpdate(ctx, cwd, plan, *cm.push)
	if err != nil {
		return err
	}
	switch {
	case plan.Advice == nil:
	case res.Pushed:
		fmt.Println("specsync: updated and pushed", plan.Branch)
	case res.Applied:
		fmt.Println("specsync: updated", plan.Branch, "(not pushed)")
	}
	return nil
}

func printUpdatePlan(plan specsync.UpdatePlan, apply bool) {
	if plan.Advice == nil {
		fmt.Println("specsync:", plan.Branch, "is up to date; nothing to do")
		return
	}
	head := "plan"
	if !apply {
		head = "plan only, nothing changed"
	}
	fmt.Printf("%s (%s): %s\n", plan.Advice.Summary, head, plan.Branch)
	for _, c := range plan.Advice.Commands {
		note := ""
		if strings.HasPrefix(c, "git push") {
			note = "   (only with -push)"
		}
		fmt.Printf("  $ %s%s\n", c, note)
	}
	if len(plan.Conflicts) > 0 {
		fmt.Printf("predicted conflicts: %s (-apply will stop there for you to resolve)\n", strings.Join(plan.Conflicts, ", "))
	}
	if !apply {
		fmt.Println("add -apply to run it")
	}
}
