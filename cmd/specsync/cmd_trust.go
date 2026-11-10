package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	specsync "github.com/androidand/specsync"
)

// trustCmd records that the user has read the commands in this repo's
// openspec/specsync.yml and allows specsync to run them.
type trustCmd struct {
	openspec *string
	dryRun   *bool
	revoke   *bool
}

func init() { Register(&trustCmd{}) }

func (*trustCmd) Spec() Spec {
	return Spec{
		Name:     "trust",
		Group:    GroupHousekeeping,
		Summary:  "Allow specsync to run the commands in this repo's specsync.yml.",
		Intents:  []string{"trust a repo's gates", "allow gate commands to run"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "setup", RelatedAfter: []string{"doctor"}},
		Safety: []string{
			"Writes only a per-user record (a hash of the config) outside the repo; read the printed commands first",
			"A changed config needs trust again; specsync never runs repository-defined commands without it",
		},
		Examples: []string{
			"specsync trust -dry-run",
			"specsync trust",
			"specsync trust -revoke",
		},
	}
}

func (cm *trustCmd) Flags(fs *flag.FlagSet) {
	cm.openspec = fs.String("openspec", "openspec", "path to the openspec/ directory")
	cm.dryRun = fs.Bool("dry-run", false, "show the commands that would be trusted without recording trust")
	cm.revoke = fs.Bool("revoke", false, "remove the trust record for this repository")
}

func (cm *trustCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	abs, err := filepath.Abs(*cm.openspec)
	if err != nil {
		return err
	}
	root := filepath.Dir(abs)

	if *cm.revoke {
		if *cm.dryRun {
			fmt.Println("DRY RUN — would remove the trust record for", root)
			return nil
		}
		if err := specsync.Untrust(root); err != nil {
			return err
		}
		fmt.Println("specsync: trust removed for", root)
		return nil
	}

	ways, issues := specsync.LoadWays(root)
	if len(issues) > 0 {
		return fmt.Errorf("fix the config first (specsync doctor config): %s", issues[0])
	}
	if specsync.ConfigHash(root) == "" {
		return fmt.Errorf("no %s to trust", specsync.SpecSyncConfigPath)
	}
	for _, c := range ways.GateCommands(true, nil) {
		fmt.Printf("  $ %s\n", c)
	}
	if *cm.dryRun {
		fmt.Println("DRY RUN — would trust the commands above for", root)
		return nil
	}
	if _, err := specsync.Trust(root); err != nil {
		return err
	}
	fmt.Println("specsync: trusted", specsync.SpecSyncConfigPath, "for", root)
	return nil
}
