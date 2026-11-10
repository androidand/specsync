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

type saveFlags struct {
	openspec, storeFlag *string
	message, change     *string
	add, allow          stringSlice
	push, pushToRequest *bool
	syncSpec, yes       *bool
	dryRun, asJSON      *bool
}

func (sf *saveFlags) define(fs *flag.FlagSet, withSync bool) {
	sf.openspec, sf.storeFlag = addRootFlags(fs)
	sf.message = fs.String("m", "", "commit message (required)")
	sf.change = fs.String("change", "", "the change this work belongs to (default: found from the branch name)")
	fs.Var(&sf.allow, "allow", "a file whose secret scan result you have reviewed as a false positive (repeatable); reported when used")
	fs.Var(&sf.add, "add", "an untracked file to include (repeatable); untracked files are never included otherwise")
	sf.push = fs.Bool("push", false, "push the branch after committing (never forces)")
	sf.pushToRequest = fs.Bool("push-to-request", false, "allow pushing a branch that has an open pull request")
	sf.syncSpec, sf.yes = new(bool), new(bool)
	if withSync {
		sf.syncSpec = fs.Bool("sync", false, "after committing, preview then sync the change to its tracker issue (specsync -change <slug>); a change with no issue yet gets one only with -yes")
		sf.yes = fs.Bool("yes", false, "with -sync, allow creating a new tracker issue for a change that has none")
	}
	sf.dryRun = fs.Bool("dry-run", false, "show what would be committed and pushed and change nothing")
	sf.asJSON = fs.Bool("json", false, "output as JSON")
}

// configRoot is where the repo's openspec/specsync.yml lives for this run: the
// directory of an explicit -openspec, else the repository top of the current
// directory (or the main checkout's when the worktree has none).
func configRoot(ctx context.Context, fs *flag.FlagSet, cwd string) string {
	if f := fs.Lookup("openspec"); f != nil && flagVisited(fs, "openspec") {
		abs, _ := filepath.Abs(f.Value.String())
		return filepath.Dir(abs)
	}
	return specsync.RepoConfigRoot(ctx, cwd)
}

// openRequestFrom reads the open request for branch out of a listing. A
// truncated listing with no match may simply be missing it, so the direct
// query decides; if that fails too the state is unknown.
func openRequestFrom(ctx context.Context, set specsync.ChangeRequestSet, branch string, direct func(context.Context, string) (int, error)) (int, error) {
	for _, r := range set.Requests {
		if r.Branch == branch && r.State == "open" && !r.Fork {
			return r.Number, nil
		}
	}
	if set.Truncated {
		return direct(ctx, branch)
	}
	return 0, nil
}

func directOpenRequest(repo string) func(context.Context, string) (int, error) {
	return func(ctx context.Context, branch string) (int, error) {
		args := []string{"pr", "list", "--head", branch, "--state", "open", "--json", "number", "--limit", "5"}
		if repo != "" {
			args = append(args, "--repo", repo)
		}
		var out, errb bytes.Buffer
		cmd := exec.CommandContext(ctx, "gh", args...)
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return 0, fmt.Errorf("gh pr list: %s", strings.TrimSpace(errb.String()))
		}
		var rows []struct {
			Number int `json:"number"`
		}
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			return 0, err
		}
		if len(rows) > 0 {
			return rows[0].Number, nil
		}
		return 0, nil
	}
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
		return openRequestFrom(ctx, set, branch, directOpenRequest(repo))
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
			"Scans only the lines a change adds (whole new files), the commit message too; a file it cannot scan is refused unless named with -allow, which is reported",
			"Refuses a detached HEAD, and pushes only with -push, by explicit refspec, never forcing: it refuses the default branch and a branch with an open or unknown pull request unless -push-to-request is given",
			"Every refusal happens before anything is committed",
			"-dry-run lists what would be committed and changes nothing",
		},
		Examples: []string{
			"specsync save -m 'fix: handle empty input (#42)' -dry-run",
			"specsync save -m 'fix: handle empty input (#42)'",
			"specsync save -m 'add the new fixture' -add testdata/new.json -push",
		},
	}
}

func (cm *saveCmd) Flags(fs *flag.FlagSet) { cm.define(fs, true) }

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
	ways := loadWaysOrFail(configRoot(ctx, fs, cwd))
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
		Dir: cwd, Message: *cm.message, Add: cm.add, Allow: cm.allow, Ways: ways,
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
	for _, a := range res.Allowed {
		fmt.Fprintf(os.Stderr, "specsync: allowed %s (secret scan overridden with -allow)\n", a)
	}
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
	cm.define(fs, false)
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
	ways := loadWaysOrFail(configRoot(ctx, fs, cwd))
	var change *specsync.Change
	if *cm.change != "" {
		change = mutableChange(resolveRoot(fs, cm.openspec, cm.storeFlag).Dir, *cm.change, false)
	} else {
		change = matchedChange(ctx, fs, cm.openspec, cm.storeFlag, cwd, "HEAD")
	}
	if change == nil {
		return fmt.Errorf("park could not tell which change this branch belongs to; pass -change <slug>")
	}
	noteLine := "- parked: " + strings.TrimSpace(*cm.message) + "\n"
	if specsync.ScanSecrets(noteLine) {
		return fmt.Errorf("the message (and so the note) contains what looks like a key or token; nothing was written")
	}
	opts := specsync.SaveOptions{Dir: cwd, Message: "park: " + *cm.message, Ways: ways, Push: *cm.push,
		PushToRequest: *cm.pushToRequest, OpenRequest: openRequestFor(""), DryRun: true, Add: cm.add, Allow: cm.allow}
	if _, err := specsync.Save(ctx, opts); err != nil && !strings.Contains(err.Error(), "nothing to commit") {
		return err
	}
	if *cm.dryRun {
		fmt.Printf("DRY RUN — would set %s to %s, note %q, then save\n", change.Slug, *cm.stage, *cm.message)
		return nil
	}
	top, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return err
	}
	topDir := strings.TrimSpace(string(top))
	metaPath := filepath.Join(change.Dir, ".specsync", "metadata.json")
	notePath := filepath.Join(change.Dir, "discoveries.md")
	restore := snapshotFiles(metaPath, notePath)

	meta := changeMetadata(change)
	st := specsync.Stage(*cm.stage)
	meta.Stage = &st
	existing, _ := os.ReadFile(notePath)
	text := strings.TrimSpace(string(existing))
	if text != "" {
		text += "\n"
	}
	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		restore()
		return err
	}
	if err := os.WriteFile(notePath, []byte(text+noteLine), 0o644); err != nil {
		restore()
		return err
	}
	for _, written := range []string{metaPath, notePath} {
		rel, err := filepath.Rel(resolvedPath(topDir), resolvedPath(written))
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		tracked := exec.Command("git", "-C", topDir, "ls-files", "--error-unmatch", "--", rel).Run() == nil
		ignored := exec.Command("git", "-C", topDir, "check-ignore", "-q", "--", rel).Run() == nil
		if !tracked && !ignored {
			opts.AddFromTop = append(opts.AddFromTop, filepath.ToSlash(rel))
		}
	}
	opts.DryRun = false
	res, err := specsync.Save(ctx, opts)
	if err != nil && !res.Committed {
		if strings.Contains(err.Error(), "nothing to commit") {
			fmt.Printf("specsync: %s set to %s, note recorded (nothing to commit: the spec folder is not tracked here)\n", change.Slug, *cm.stage)
			return nil
		}
		restore()
		return err
	}
	fmt.Printf("specsync: %s set to %s, note recorded\n", change.Slug, *cm.stage)
	if res.Committed {
		printSave(res, false, false, *cm.push)
	}
	return err
}

// snapshotFiles records the current content of each file (or that it did not
// exist) and returns a function that puts them back.
func snapshotFiles(paths ...string) func() {
	type snap struct {
		path   string
		data   []byte
		exists bool
	}
	var snaps []snap
	for _, p := range paths {
		d, err := os.ReadFile(p)
		snaps = append(snaps, snap{p, d, err == nil})
	}
	return func() {
		for _, s := range snaps {
			if s.exists {
				_ = os.WriteFile(s.path, s.data, 0o644)
			} else {
				_ = os.Remove(s.path)
			}
		}
	}
}

func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
