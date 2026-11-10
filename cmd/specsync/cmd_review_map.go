package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/androidand/specsync"
)

// reviewMapCmd turns a branch, range or change request into a review checklist,
// or proves a rewritten branch holds the same content.
type reviewMapCmd struct {
	openspec   *string
	store      *string
	change     *string
	base       *string
	asJSON     *bool
	equivalent *bool
}

func init() { Register(&reviewMapCmd{}) }

func (*reviewMapCmd) Spec() Spec {
	return Spec{
		Name:     "review-map",
		Group:    GroupReview,
		Summary:  "Turn a branch into a review checklist, or prove a rewritten branch has the same content.",
		Intents:  []string{"review a large pull request", "what to read first", "check a squash kept the content"},
		Workflow: AgentCommandWorkflow{Position: "pre-merge", RelatedBefore: []string{"verify"}},
		Args: []Arg{
			{Name: "target", Description: "a branch, a range base..head, or a pull request number or URL (default: the checked-out branch against origin/HEAD); with -equivalent: <old> <new>"},
		},
		Safety: []string{
			"Read-only: needs git; the provider only resolves a pull request number or URL",
			"Marks files as needing judgement only from the change's `## Review focus` section; it marks nothing as safe and is not a reviewer",
			"-equivalent compares the content of two revisions, not their history; it exits 1 when they differ and 2 on an error",
		},
		Examples: []string{
			"specsync review-map",
			"specsync review-map origin/main..feat/42-thing -json",
			"specsync review-map -change my-change 123",
			"specsync review-map -equivalent feat/42-thing-old feat/42-thing",
		},
	}
}

func (cm *reviewMapCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.store = addRootFlags(fs)
	cm.change = fs.String("change", "", "the change whose `## Review focus` and tasks apply (default: the one the branch belongs to)")
	cm.base = fs.String("base", "", "base to compare against (default: origin/HEAD)")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
	cm.equivalent = fs.Bool("equivalent", false, "compare the trees of <old> and <new>; exit non-zero when they differ")
}

func (cm *reviewMapCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	args = reorderKnownFlags(fs, args)
	_ = fs.Parse(args)
	checkArgs(fs, args, -1)
	ctx := context.Background()
	dir, err := os.Getwd()
	if err != nil {
		fail(err)
	}

	if *cm.equivalent {
		if fs.NArg() != 2 || *cm.asJSON || *cm.change != "" || *cm.base != "" {
			fail(fmt.Errorf("usage: specsync review-map -equivalent <old> <new>"))
		}
		same, paths, err := specsync.TreesEqual(ctx, dir, fs.Arg(0), fs.Arg(1))
		if err != nil {
			fmt.Fprintf(os.Stderr, "specsync: %v\n", err)
			os.Exit(2)
		}
		if same {
			fmt.Printf("identical: %s and %s have the same tree. This compares content, not history.\n", fs.Arg(0), fs.Arg(1))
			return nil
		}
		fmt.Printf("differs: %d path(s) differ between %s and %s. This compares content, not history.\n", len(paths), fs.Arg(0), fs.Arg(1))
		for _, p := range paths {
			fmt.Printf("- %s\n", p)
		}
		os.Exit(1)
	}

	if fs.NArg() > 1 {
		fail(fmt.Errorf("usage: specsync review-map [<branch | base..head | pull request>]"))
	}
	base, head, err := resolveReviewRange(ctx, dir, fs.Arg(0), *cm.base, ghPullRange)
	if err != nil {
		fail(err)
	}

	focus, tasks, slug, note := reviewInputs(ctx, fs, cm, dir, head)
	m, err := specsync.BuildReviewMap(ctx, dir, base, head, focus, tasks)
	if err != nil {
		fail(err)
	}
	m.Change = slug
	m.Note = note
	if *cm.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
			fail(err)
		}
		return nil
	}
	fmt.Print(m.Markdown())
	return nil
}

// reviewInputs reads the change's review focus and tasks from the revision
// being mapped, not the working tree, and says in note why there is no focus.
func reviewInputs(ctx context.Context, fs *flag.FlagSet, cm *reviewMapCmd, dir, head string) (focus []specsync.ReviewFocus, tasks []specsync.TaskRef, slug, note string) {
	slug = *cm.change
	root := resolveRootUnchecked(fs, cm.openspec, cm.store)
	if slug == "" {
		if c := matchedChange(ctx, fs, cm.openspec, cm.store, dir, strings.TrimPrefix(head, "origin/")); c != nil {
			slug = c.Slug
		}
	}
	if slug == "" {
		return nil, nil, "", "No change matched this branch; pass -change <slug> to apply its review focus."
	}
	rel, err := changeDirInRepo(ctx, dir, root.Dir, slug)
	if err != nil {
		return nil, nil, slug, fmt.Sprintf("change %s not found at %s: %v", slug, head, err)
	}
	proposal, err := specsync.ShowFileAt(ctx, dir, head, rel+"/proposal.md")
	if err != nil {
		return nil, nil, slug, fmt.Sprintf("change %s not found at %s; nothing to apply.", slug, head)
	}
	if t, err := specsync.ShowFileAt(ctx, dir, head, rel+"/tasks.md"); err == nil {
		tasks = specsync.ParseTaskIDs(t)
	}
	focus = specsync.ParseReviewFocus(proposal)
	if len(focus) == 0 {
		return nil, tasks, slug, "Nothing declared. Add a `## Review focus` section to the change's proposal.md to mark paths."
	}
	return focus, tasks, slug, "No changed file matches the declared focus."
}

// changeDirInRepo is the slash path of a change folder relative to the
// repository root, as git names it.
func changeDirInRepo(ctx context.Context, dir, openspecDir, slug string) (string, error) {
	top, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	topDir, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil {
		return "", err
	}
	specDir, err := filepath.EvalSymlinks(openspecDir)
	if err != nil {
		specDir = openspecDir
	}
	rel, err := filepath.Rel(topDir, specDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("the spec root is outside this repository")
	}
	return path.Join(filepath.ToSlash(rel), "changes", slug), nil
}

// reorderKnownFlags moves flags (and the value of each non-boolean flag) ahead
// of positionals, so "review-map a..b -json" works like "review-map -json a..b".
func reorderKnownFlags(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

var pullRef = regexp.MustCompile(`^(?:\d+|https?://\S+/pull/\d+/?)$`)

// resolveReviewRange maps the target to a base and head: a pull request number
// or URL through lookup, "base..head" or "base...head" literally, or one ref
// against baseFlag or origin/HEAD.
func resolveReviewRange(ctx context.Context, dir, target, baseFlag string, lookup func(context.Context, string, string) (string, string, error)) (base, head string, err error) {
	switch {
	case target == "":
		head = "HEAD"
	case pullRef.MatchString(target) && !isCommit(ctx, dir, target):
		base, head, err = lookup(ctx, dir, target)
		if err == nil && baseFlag != "" {
			base = baseFlag
		}
		return base, head, err
	case strings.Contains(target, "..."):
		p := strings.SplitN(target, "...", 2)
		return p[0], p[1], nil
	case strings.Contains(target, ".."):
		p := strings.SplitN(target, "..", 2)
		return p[0], p[1], nil
	default:
		head = target
	}
	if baseFlag != "" {
		return baseFlag, head, nil
	}
	base, err = specsync.DefaultBase(ctx, dir)
	if err != nil {
		return "", "", fmt.Errorf("no base to compare against: %w; pass -base <ref> or a base..head range", err)
	}
	return base, head, nil
}

// isCommit reports whether a digit-only target names a commit here, in which
// case it is a ref and not a pull request number.
func isCommit(ctx context.Context, dir, rev string) bool {
	if strings.HasPrefix(rev, "http") {
		return false
	}
	return exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "-q", "--end-of-options", rev+"^{commit}").Run() == nil
}

// ghPullRange asks gh for a pull request's base and head commits, so a map does
// not depend on branch names that forks and deleted branches lose. A head that
// is not local is fetched from the pull request ref.
func ghPullRange(ctx context.Context, dir, ref string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", ref, "--json", "number,baseRefOid,headRefOid")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve pull request %s: %w", ref, err)
	}
	var pr struct {
		Number int    `json:"number"`
		Base   string `json:"baseRefOid"`
		Head   string `json:"headRefOid"`
	}
	if err := json.Unmarshal(out, &pr); err != nil || pr.Base == "" || pr.Head == "" {
		return "", "", fmt.Errorf("resolve pull request %s: unexpected gh output", ref)
	}
	local := func(oid string) bool {
		return exec.CommandContext(ctx, "git", "-C", dir, "cat-file", "-e", oid+"^{commit}").Run() == nil
	}
	if !local(pr.Head) && pr.Number > 0 {
		_ = exec.CommandContext(ctx, "git", "-C", dir, "fetch", "-q", "origin", fmt.Sprintf("refs/pull/%d/head", pr.Number)).Run()
	}
	for name, oid := range map[string]string{"head": pr.Head, "base": pr.Base} {
		if !local(oid) {
			return "", "", fmt.Errorf("the %s commit %.12s of pull request %s is not available locally and could not be fetched (a fork or a deleted branch?); fetch it, or pass a base..head range", name, oid, ref)
		}
	}
	return pr.Base, pr.Head, nil
}
