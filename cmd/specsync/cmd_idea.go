package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/androidand/specsync"
)

// ideaCmd captures an idea as a GitHub issue labeled stage:intake.
// Text is provided via argument or stdin. Title is derived mechanically
// (first line or first sentence, truncated to 70 chars). Body is the
// verbatim text plus a capture timestamp.
type ideaCmd struct {
	openspec  *string
	storeFlag *string
	repo      *string
}

func init() { Register(&ideaCmd{}) }

func (*ideaCmd) Spec() Spec {
	return Spec{
		Name:     "idea",
		Group:    GroupPlan,
		Summary:  "Capture a free-text idea as a stage:intake issue.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "capture", RelatedAfter: []string{"ideas", "pull"}},
		Safety: []string{
			"Creates a real GitHub issue — there is no -dry-run",
		},
		Examples: []string{
			"specsync idea 'Consider caching the trace graph'",
			"echo 'Consider caching the trace graph' | specsync idea",
		},
	}
}

func (cm *ideaCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "repo as owner/name (default: config ideas_repo, $SPECSYNC_IDEAS_REPO, or current repo)")
}

func (cm *ideaCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *ideaCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, repo := cm.openspec, cm.storeFlag, cm.repo
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 1) // idea text is the one expected positional

	// Resolve repo: -repo flag → openspec/specsync.yml → SPECSYNC_IDEAS_REPO → auto-detect.
	abs := resolveRoot(fs, openspec, storeFlag).Dir
	repoRoot := filepath.Dir(abs)
	targetRepo := specsync.ResolveIdeasRepo(*repo, repoRoot)

	// Read idea text from arg or stdin.
	var ideaText string
	if fs.NArg() > 0 {
		ideaText = fs.Args()[0]
	} else {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			fail(fmt.Errorf("read stdin: %w", err))
		}
		ideaText = string(data)
	}

	ideaText = strings.TrimSpace(ideaText)
	if ideaText == "" {
		fail(fmt.Errorf("idea text is empty"))
	}

	// Derive title: first line or first sentence, truncated to 70 chars.
	title := deriveIdeaTitle(ideaText)

	// Build body: verbatim text + timestamp.
	body := ideaText + "\n\n---\n*captured " + time.Now().UTC().Format(time.RFC3339) + "*"

	// Create the GitHub issue.
	provider := specsync.NewGitHubProviderWithRepo(targetRepo)

	// Ensure stage:intake label exists.
	if err := provider.EnsureLabels(context.Background(), []string{"stage:intake"}); err != nil {
		fail(fmt.Errorf("create stage:intake label: %w", err))
	}

	// Create issue via gh CLI directly.
	runGH := func(ctx context.Context, ghArgs ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "gh", ghArgs...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("gh %s: %w\n%s", strings.Join(ghArgs, " "), err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}

	var repoArgs []string
	if targetRepo != "" {
		repoArgs = append(repoArgs, "--repo", targetRepo)
	}
	ghArgs := append([]string{"issue", "create"}, repoArgs...)
	ghArgs = append(ghArgs, "--title", title, "--body", body, "--label", "stage:intake")
	url, err := runGH(context.Background(), ghArgs...)
	if err != nil {
		fail(err)
	}

	fmt.Println(url)
}
