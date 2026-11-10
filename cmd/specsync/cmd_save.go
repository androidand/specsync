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

type saveFlags struct {
	openspec, storeFlag *string
	message, change     *string
	add                 stringSlice
	push, pushToRequest *bool
	syncSpec, yes       *bool
	dryRun, asJSON      *bool
}

func (sf *saveFlags) define(fs *flag.FlagSet) {
	sf.openspec, sf.storeFlag = addRootFlags(fs)
	sf.message = fs.String("m", "", "commit message (required)")
	sf.change = fs.String("change", "", "the change this work belongs to (default: found from the branch name)")
	fs.Var(&sf.add, "add", "an untracked file to include (repeatable); untracked files are never included otherwise")
	sf.push = fs.Bool("push", false, "push the branch after committing (never forces)")
	sf.pushToRequest = fs.Bool("push-to-request", false, "allow pushing a branch that has an open pull request")
	sf.syncSpec = fs.Bool("sync", false, "after committing, preview then sync the change to its tracker issue (specsync -change <slug>); a change with no issue yet gets one only with -yes")
	sf.yes = fs.Bool("yes", false, "with -sync, allow creating a new tracker issue for a change that has none")
	sf.dryRun = fs.Bool("dry-run", false, "show what would be committed and pushed and change nothing")
	sf.asJSON = fs.Bool("json", false, "output as JSON")
}

func openRequestFor(repo string) func(ctx context.Context, branch string) (int, error) {
	return func(ctx context.Context, branch string) (int, error) {
		src, ok := makeProvider(repo, false, "github", "").(specsync.ChangeRequestSource)
		if !ok {
			return 0, fmt.Errorf("the provider cannot list pull requests")
		}
		set, err := src.ChangeRequests(ctx)
		if err != nil {
			return 0, err
		}
		for _, r := range set.Requests {
			if r.Branch == branch && r.State == "open" && !r.Fork {
				return r.Number, nil
			}
		}
		return 0, nil
	}
}

func loadWaysOrFail(root string) specsync.Ways {
	ways, issues := specsync.LoadWays(root)
	if len(issues) > 0 {
		fail(fmt.Errorf("fix the config first (specsync doctor config): %s", issues[0]))
	}
	return ways
}

func printSave(res specsync.SaveResult, dryRun, asJSON bool, pushRequested bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return
	}
	prefix := "specsync: "
	if dryRun {
		prefix = "DRY RUN — would "
	}
	verb := "committed"
	if dryRun {
		verb = "commit"
	}
	fmt.Printf("%s%s %d file(s) on %s\n", prefix, verb, len(res.Files), res.Branch)
	for _, f := range res.Files {
		fmt.Println("  " + f)
	}
	switch {
	case res.Pushed:
		fmt.Println("specsync: pushed", res.Branch)
	case dryRun && pushRequested:
		fmt.Println("DRY RUN — would push", res.Branch, "unless a pull request is open")
	case res.Committed:
		fmt.Println("specsync: not pushed (add -push to push)")
	}
}

type saveCmd struct{ saveFlags }

func init() { Register(&saveCmd{}) }

func (*saveCmd) Spec() Spec {
	return Spec{
		Name:     "save",
		Group:    GroupPlan,
		Summary:  "Commit your work safely: refuses secrets and forbidden trailers, pushes only when asked.",
		Intents:  []string{"commit my work", "save progress"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"next"}, RelatedAfter: []string{"gates", "update"}},
		Safety: []string{
			"Commits tracked changes only; an untracked file is included only when named with -add",
			"Refuses files that look like secrets (by name or content) and any save.forbid_files, and trailers listed in save.forbid_trailers",
			"Pushes only with -push, never forces, and refuses a branch that has an open pull request unless -push-to-request is given",
			"-dry-run lists what would be committed and changes nothing",
		},
		Examples: []string{
			"specsync save -m 'fix: handle empty input (#42)' -dry-run",
			"specsync save -m 'fix: handle empty input (#42)'",
			"specsync save -m 'add the new fixture' -add testdata/new.json -push",
		},
	}
}

func (cm *saveCmd) Flags(fs *flag.FlagSet) { cm.define(fs) }

func (cm *saveCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*cm.openspec)
	ways := loadWaysOrFail(filepath.Dir(abs))
	slug := ""
	var syncChange *specsync.Change
	if *cm.syncSpec {
		c := matchedChange(ctx, fs, cm.openspec, cm.storeFlag, cwd, "HEAD")
		if *cm.change != "" {
			c, _ = specsync.LoadChangeBySlug(resolveRoot(fs, cm.openspec, cm.storeFlag).Dir, *cm.change)
		}
		if c == nil {
			return fmt.Errorf("-sync could not tell which change this branch belongs to; pass -change <slug>")
		}
		slug, syncChange = c.Slug, c
	}
	res, err := specsync.Save(ctx, specsync.SaveOptions{
		Dir: cwd, Message: *cm.message, Add: cm.add, Ways: ways,
		Push: *cm.push, PushToRequest: *cm.pushToRequest, OpenRequest: openRequestFor(""), DryRun: *cm.dryRun,
	})
	if err != nil && !res.Committed {
		if *cm.asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]string{"error": err.Error()})
			os.Exit(1)
		}
		return err
	}
	printSave(res, *cm.dryRun, *cm.asJSON, *cm.push)
	if err != nil || slug == "" || *cm.dryRun {
		if slug != "" && *cm.dryRun {
			fmt.Printf("DRY RUN — would then sync %s to its issue\n", slug)
		}
		return err
	}
	exe, exeErr := os.Executable()
	if exeErr != nil {
		return exeErr
	}
	syncArgs := []string{"-change", slug, "-openspec", *cm.openspec}
	if *cm.storeFlag != "" {
		syncArgs = append(syncArgs, "-store", *cm.storeFlag)
	}
	preview := exec.CommandContext(ctx, exe, append(syncArgs, "-dry-run")...)
	preview.Stdout, preview.Stderr = os.Stdout, os.Stderr
	if err := preview.Run(); err != nil {
		return fmt.Errorf("the sync preview failed, so nothing was synced: %w", err)
	}
	if !changeHasIssue(syncChange) && !*cm.yes {
		fmt.Printf("specsync: not synced: %s has no tracker issue yet, so syncing would create one; review the preview above and run again with -sync -yes\n", slug)
		return nil
	}
	sync := exec.CommandContext(ctx, exe, syncArgs...)
	sync.Stdout, sync.Stderr = os.Stdout, os.Stderr
	return sync.Run()
}

func changeHasIssue(c *specsync.Change) bool {
	if c == nil {
		return false
	}
	refs, err := specsync.LoadRefs(c.Dir)
	if err != nil {
		return false
	}
	for _, r := range refs {
		if r.ID != "" {
			return true
		}
	}
	return false
}

type parkCmd struct {
	saveFlags
	stage *string
}

func init() { Register(&parkCmd{}) }

func (*parkCmd) Spec() Spec {
	return Spec{
		Name:     "park",
		Group:    GroupPlan,
		Summary:  "Stop work so another session can resume: save, set the stage and leave a note.",
		Intents:  []string{"stop for the day", "pause work"},
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "implementation", RelatedBefore: []string{"save"}, RelatedAfter: []string{"status"}},
		Safety: []string{
			"Runs the same guards as save before anything is written; nothing is discarded, reset or deleted",
			"Sets the change's stage (default backlog) and appends a `parked:` line to its discoveries.md, then commits",
			"Pushes only with -push, with the same rules as save",
		},
		Examples: []string{
			"specsync park -m 'waiting on the API change' -dry-run",
			"specsync park -m 'waiting on the API change' -stage blocked",
		},
	}
}

func (cm *parkCmd) Flags(fs *flag.FlagSet) {
	cm.define(fs)
	cm.stage = fs.String("stage", "backlog", "the stage to set on the change")
}

func (cm *parkCmd) Run(ctx context.Context, args []string) error {
	fs := flagSetFor(cm)
	if err := fs.Parse(args); err != nil {
		return err
	}
	checkArgs(fs, args, 0)
	if strings.TrimSpace(*cm.message) == "" {
		return fmt.Errorf("park needs a note for whoever resumes: -m <message>")
	}
	if err := specsync.ValidateStage(specsync.Stage(*cm.stage)); err != nil {
		return err
	}
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
	if change == nil {
		return fmt.Errorf("park could not tell which change this branch belongs to; pass -change <slug>")
	}
	opts := specsync.SaveOptions{Dir: cwd, Message: "park: " + *cm.message, Ways: ways, Push: *cm.push,
		PushToRequest: *cm.pushToRequest, OpenRequest: openRequestFor(""), DryRun: true, Add: cm.add}
	if _, err := specsync.Save(ctx, opts); err != nil && !strings.Contains(err.Error(), "nothing to commit") {
		return err
	}
	if *cm.dryRun {
		fmt.Printf("DRY RUN — would set %s to %s, note %q, then save\n", change.Slug, *cm.stage, *cm.message)
		return nil
	}
	meta := changeMetadata(change)
	s := specsync.Stage(*cm.stage)
	meta.Stage = &s
	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		return err
	}
	notePath := filepath.Join(change.Dir, "discoveries.md")
	existing, _ := os.ReadFile(notePath)
	text := strings.TrimSpace(string(existing))
	if text != "" {
		text += "\n"
	}
	if err := os.WriteFile(notePath, []byte(text+"- parked: "+strings.TrimSpace(*cm.message)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("specsync: %s set to %s, note recorded\n", change.Slug, *cm.stage)
	for _, written := range []string{filepath.Join(change.Dir, ".specsync", "metadata.json"), notePath} {
		rel, err := filepath.Rel(cwd, written)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		tracked := exec.Command("git", "-C", cwd, "ls-files", "--error-unmatch", "--", rel).Run() == nil
		ignored := exec.Command("git", "-C", cwd, "check-ignore", "-q", "--", rel).Run() == nil
		if !tracked && !ignored {
			opts.Add = append(opts.Add, rel)
		}
	}
	opts.DryRun = false
	res, err := specsync.Save(ctx, opts)
	if err != nil && strings.Contains(err.Error(), "nothing to commit") {
		fmt.Println("specsync: nothing to commit (the spec folder is not tracked here)")
		return nil
	}
	if res.Committed {
		printSave(res, false, false, *cm.push)
	}
	return err
}
