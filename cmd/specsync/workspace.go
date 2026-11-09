package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/androidand/specsync"
)

// runWorkspace implements `specsync workspace setup|archive`, meant to run
// from an orchestrator's workspace hooks (Orca's orca.yaml scripts.setup and
// scripts.archive).
func runWorkspace(args []string) {
	if len(args) == 0 || (args[0] != "setup" && args[0] != "archive") {
		fail(fmt.Errorf("usage: specsync workspace setup|archive [flags]"))
	}
	action := args[0]
	fs := flag.NewFlagSet("workspace "+action, flag.ExitOnError)
	openspec, storeFlag := addRootFlags(fs)
	repo := fs.String("repo", "", "repo as owner/name for reading the linked issue (default: auto-detect from git remote)")
	doSync := fs.Bool("sync", false, "sync the change to the tracker after updating its stage")
	dryRun := fs.Bool("dry-run", false, "report the change and what would be done without writing anything")
	if err := fs.Parse(args[1:]); err != nil {
		fail(err)
	}
	checkArgs(fs, args[1:], 0)
	root := resolveRoot(fs, openspec, storeFlag).Dir

	branch, def := specsync.CurrentBranches()
	linkedIssue := func() string {
		return orcaLinkedIssue(orcaQuery{
			run: func(name string, args ...string) ([]byte, error) {
				return exec.CommandContext(context.Background(), name, args...).Output()
			},
			lookPath: exec.LookPath,
			getenv:   os.Getenv,
			goos:     runtime.GOOS,
		})
	}

	slug, err := workspaceAction(os.Stdout, action, specsync.WorkspaceChangeOptions{
		OpenSpecDir:   root,
		Branch:        branch,
		DefaultBranch: def,
		LinkedIssueFn: linkedIssue,
		Provider:      makeProvider(*repo, false, "github", ""),
		PullMissing:   action == "setup",
		DryRun:        *dryRun,
	})
	if err != nil {
		fail(err)
	}
	if slug == "" || !*doSync {
		return
	}
	if *dryRun {
		fmt.Printf("  would sync: specsync -change %s\n", slug)
		return
	}
	runSync([]string{"-openspec", root, "-change", slug})
}

// workspaceAction finds the workspace's change and sets its stage: active on
// setup, auto (derived from tasks) on archive. It returns the change, or ""
// when the workspace names none — which is not an error, so a hook never
// blocks an unrelated workspace.
func workspaceAction(out io.Writer, action string, opts specsync.WorkspaceChangeOptions) (string, error) {
	res, err := specsync.WorkspaceChange(context.Background(), opts)
	if err != nil {
		return "", err
	}
	if res.WouldPull {
		fmt.Fprintf(out, "workspace %s: would pull issue #%s into a new change and set it active\n", action, res.Issue)
		return "", nil
	}
	if res.Slug == "" {
		fmt.Fprintf(out, "workspace %s: this workspace names no change; nothing to do\n", action)
		return "", nil
	}
	how := map[string]string{
		"branch":       "from branch " + opts.Branch,
		"ref-cache":    "linked to issue #" + res.Issue,
		"issue-marker": "named by issue #" + res.Issue,
		"pulled":       "pulled from issue #" + res.Issue,
	}[res.Source]
	stage := "active"
	if action == "archive" {
		stage = "auto"
	}
	if opts.DryRun {
		fmt.Fprintf(out, "workspace %s: change %s (%s); would set stage %s\n", action, res.Slug, how, stage)
		return res.Slug, nil
	}
	change, err := specsync.LoadChangeBySlug(opts.OpenSpecDir, res.Slug)
	if err != nil {
		return "", err
	}
	meta := changeMetadata(change)
	if stage == "auto" {
		meta.Stage = nil
	} else {
		s := specsync.Stage(stage)
		meta.Stage = &s
	}
	if err := specsync.SaveChangeMetadata(change.Dir, meta); err != nil {
		return "", err
	}
	fmt.Fprintf(out, "workspace %s: change %s (%s) → %s\n", action, res.Slug, how, stage)
	return res.Slug, nil
}

type orcaQuery struct {
	run      func(name string, args ...string) ([]byte, error)
	lookPath func(string) (string, error)
	getenv   func(string) string
	goos     string
}

// orcaLinkedIssue asks Orca for the current workspace's linked GitHub issue.
// It returns "" when Orca is not installed, the directory is not an Orca
// workspace, or no issue is linked.
func orcaLinkedIssue(q orcaQuery) string {
	orca := resolveOrca(q.getenv, q.lookPath, q.goos)
	if orca == "" {
		return ""
	}
	out, err := q.run(orca, "worktree", "current", "--json")
	if err != nil {
		return ""
	}
	return parseOrcaLinkedIssue(out)
}

// parseOrcaLinkedIssue reads result.worktree.linkedIssue (a GitHub issue
// number, or null) from `orca worktree current --json`.
func parseOrcaLinkedIssue(out []byte) string {
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			Worktree struct {
				LinkedIssue *int `json:"linkedIssue"`
			} `json:"worktree"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil || !resp.OK {
		return ""
	}
	if n := resp.Result.Worktree.LinkedIssue; n != nil && *n > 0 {
		return strconv.Itoa(*n)
	}
	return ""
}
