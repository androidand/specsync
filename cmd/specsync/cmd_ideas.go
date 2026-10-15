package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/androidand/specsync"
)

// ideasCmd lists open stage:intake issues for the repo.
type ideasCmd struct {
	openspec  *string
	storeFlag *string
	repo      *string
	asJSON    *bool
}

func init() { Register(&ideasCmd{}) }

func (*ideasCmd) Spec() Spec {
	return Spec{
		Name:     "ideas",
		Group:    GroupOverview,
		Summary:  "List open stage:intake issues for the repo.",
		Workflow: AgentCommandWorkflow{Position: "review", RelatedAfter: []string{"pull"}},
		Safety: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync ideas",
			"specsync ideas -json",
		},
	}
}

func (cm *ideasCmd) Flags(fs *flag.FlagSet) {
	cm.openspec, cm.storeFlag = addRootFlags(fs)
	cm.repo = fs.String("repo", "", "repo as owner/name (default: config ideas_repo, $SPECSYNC_IDEAS_REPO, or current repo)")
	cm.asJSON = fs.Bool("json", false, "output as JSON")
}

func (cm *ideasCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *ideasCmd) run(args []string) {
	fs := flagSetFor(cm)
	openspec, storeFlag, repo, asJSON := cm.openspec, cm.storeFlag, cm.repo, cm.asJSON
	if err := fs.Parse(args); err != nil {
		fail(err)
	}
	checkArgs(fs, args, 0)

	abs := resolveRoot(fs, openspec, storeFlag).Dir
	repoRoot := filepath.Dir(abs)
	targetRepo := specsync.ResolveIdeasRepo(*repo, repoRoot)

	// List open issues with stage:intake label.
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
	ghArgs := append([]string{"issue", "list"}, repoArgs...)
	ghArgs = append(ghArgs, "--label", "stage:intake", "--state", "open", "--json", "number,title,url,createdAt")
	out, err := runGH(context.Background(), ghArgs...)
	if err != nil {
		fail(err)
	}

	var issues []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		URL       string    `json:"url"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		fail(fmt.Errorf("parse issues: %w", err))
	}

	// Sort by createdAt (oldest first).
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})

	if *asJSON {
		data, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			fail(fmt.Errorf("marshal JSON: %w", err))
		}
		fmt.Println(string(data))
		return
	}

	if len(issues) == 0 {
		fmt.Println("No open ideas.")
		return
	}

	fmt.Printf("%-6s %-20s %s\n", "NUM", "TITLE", "CREATED")
	fmt.Println("────── ──────────────────── ─────────────")
	for _, issue := range issues {
		title := issue.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Printf("%-6d %-40s %s\n", issue.Number, title, issue.CreatedAt.Format("2006-01-02"))
	}
}
