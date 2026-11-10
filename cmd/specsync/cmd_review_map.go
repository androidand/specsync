package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
			"-equivalent compares the content of two revisions, not their history",
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
			fail(err)
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

	var focus []specsync.ReviewFocus
	var tasks []specsync.TaskRef
	slug := ""
	if c := reviewChange(ctx, fs, cm, dir, head); c != nil {
		slug = c.Slug
		focus = specsync.ParseReviewFocus(c.Body)
		tasks = specsync.ParseTaskIDs(c.TasksMarkdown)
	}
	m, err := specsync.BuildReviewMap(ctx, dir, base, head, focus, tasks)
	if err != nil {
		fail(err)
	}
	m.Change = slug
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

func reviewChange(ctx context.Context, fs *flag.FlagSet, cm *reviewMapCmd, dir, head string) *specsync.Change {
	if *cm.change != "" {
		abs := resolveRoot(fs, cm.openspec, cm.store).Dir
		c, err := specsync.LoadChangeBySlug(abs, *cm.change)
		if err != nil {
			fail(err)
		}
		return c
	}
	return matchedChange(ctx, fs, cm.openspec, cm.store, dir, strings.TrimPrefix(head, "origin/"))
}

var pullRef = regexp.MustCompile(`^(?:\d+|https?://\S+/pull/\d+/?)$`)

// resolveReviewRange maps the target to a base and head: a pull request number
// or URL through lookup, "base..head" or "base...head" literally, or one ref
// against baseFlag or origin/HEAD.
func resolveReviewRange(ctx context.Context, dir, target, baseFlag string, lookup func(context.Context, string, string) (string, string, error)) (base, head string, err error) {
	switch {
	case target == "":
		head = "HEAD"
	case pullRef.MatchString(target):
		return lookup(ctx, dir, target)
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

// ghPullRange asks gh for a pull request's branches and prefers the remote
// copies, so a map does not depend on local branch names.
func ghPullRange(ctx context.Context, dir, ref string) (string, string, error) {
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", ref, "--json", "baseRefName,headRefName").Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve pull request %s: %w", ref, err)
	}
	var pr struct {
		Base string `json:"baseRefName"`
		Head string `json:"headRefName"`
	}
	if err := json.Unmarshal(out, &pr); err != nil || pr.Base == "" || pr.Head == "" {
		return "", "", fmt.Errorf("resolve pull request %s: unexpected gh output", ref)
	}
	pick := func(name string) string {
		if exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "-q", "origin/"+name).Run() == nil {
			return "origin/" + name
		}
		return name
	}
	return pick(pr.Base), pick(pr.Head), nil
}
