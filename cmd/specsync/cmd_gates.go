package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	specsync "github.com/androidand/specsync"
)

type gatesCmd struct {
	openspec *string
	only     *string
	with     *string
	cont     *bool
	dryRun   *bool
	asJSON   *bool
}

func init() { Register(&gatesCmd{}) }

func (*gatesCmd) Spec() Spec {
	return Spec{
		Name:     "gates",
		Group:    GroupReview,
		Summary:  "Run the repo's configured gates in order and stamp the commit when they pass.",
		Intents:  []string{"run the checks", "run lint and tests before a PR"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"next"}, RelatedAfter: []string{"pr"}},
		Safety: []string{
			"Runs commands from openspec/specsync.yml, so nothing runs until `specsync trust -yes` has been given for that file",
			"The stamp is written only for a full passing run on a clean tree; a new commit, a failed run or a partial run leaves no valid stamp",
			"-dry-run lists the gates that would run and runs nothing",
		},
		Examples: []string{
			"specsync gates -dry-run",
			"specsync gates",
			"specsync gates -with e2e",
			"specsync gates -only lint,types -continue",
		},
	}
}

func (cm *gatesCmd) Flags(fs *flag.FlagSet) {
	cm.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	cm.only = fs.String("only", "", "run only these gates (comma-separated names); a partial run never stamps")
	cm.with = fs.String("with", "", "also run these optional gates (comma-separated names)")
	cm.cont = fs.Bool("continue", false, "keep running after a gate fails")
	cm.dryRun = fs.Bool("dry-run", false, "list the gates that would run and run nothing")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (cm *gatesCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := configRoot(ctx, fs, cwd)
	ways, issues := specsync.LoadWays(root)
	if len(issues) > 0 {
		return fmt.Errorf("fix the config first (specsync doctor config): %s", issues[0])
	}
	opts := specsync.GateOptions{Dir: cwd, Root: root, Ways: ways, Only: splitList(*cm.only), With: splitList(*cm.with), Continue: *cm.cont, Out: os.Stdout}
	if *cm.asJSON {
		opts.Out = os.Stderr
	}
	if *cm.dryRun {
		names, err := specsync.PlanGates(opts)
		if err != nil {
			return err
		}
		fmt.Println("DRY RUN — would run, in order:")
		for _, n := range names {
			fmt.Println("  " + n)
		}
		if !specsync.IsTrusted(root) {
			fmt.Println("note: the config is not trusted yet, so a real run would stop; read the commands and run: specsync trust -yes")
		}
		return nil
	}
	rep, err := specsync.RunGates(ctx, opts)
	if err != nil {
		return err
	}
	if *cm.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		for _, r := range rep.Results {
			fmt.Printf("  %-8s %s\n", r.Status, r.Name)
		}
		switch {
		case !rep.Passed:
			fmt.Println("specsync: gates failed; no stamp")
		case rep.Stamped:
			fmt.Println("specsync: gates passed; stamped", rep.Head[:min(8, len(rep.Head))])
		default:
			fmt.Println("specsync: gates passed;", rep.Note)
		}
	}
	if !rep.Passed {
		os.Exit(1)
	}
	return nil
}
