package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FlowFacts is what the flow needs to know about the current work. It is
// read from the status collector and the repo; nothing here is guessed.
type FlowFacts struct {
	HasChange   bool
	Slug        string
	IssueID     string
	Stage       string
	Branch      string
	Worktree    string
	TasksDone   int
	TasksTotal  int
	Dirty       bool
	Head        string
	GatesPassed bool // a gate stamp exists for Head
	Trusted     bool // the config's commands may run
	PR          *ChangeRequest
	// PRUnknown is true when pull request state could not be read (offline, or
	// the lookup failed), so a missing PR does not mean there is none.
	PRUnknown bool
}

// StepState is one step of the flow with whether it is done.
type StepState struct {
	Step    string `json:"step"`
	Done    bool   `json:"done"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// FlowPosition is the configured steps and where the work is in them.
type FlowPosition struct {
	Flow     string      `json:"flow"`
	Steps    []StepState `json:"steps"`
	Current  *StepState  `json:"current,omitempty"`
	Complete bool        `json:"complete"`
}

// FactsFromRow turns a status row into flow facts. A nil row, or a branch that
// belongs to no change, is work that has not started.
func FactsFromRow(row *ChangeTopology) FlowFacts {
	if row == nil || row.NoChange || row.Slug == "" {
		return FlowFacts{}
	}
	f := FlowFacts{HasChange: true, Slug: row.Slug, Stage: row.Stage, Branch: row.Branch, Worktree: row.Worktree, PR: row.PR}
	if row.Issue != nil {
		f.IssueID = row.Issue.ID
	}
	if row.Tasks != nil {
		f.TasksDone, f.TasksTotal = row.Tasks.Done, row.Tasks.Total
	}
	if row.Local != nil {
		f.Head = row.Local.Head
		f.Dirty = row.Local.Modified+row.Local.Untracked > 0
	}
	return f
}

// GateStampFile is where a gate run records the commit it passed for, under
// the repository's git directory. specsync reads it; whoever runs the gates
// writes it.
const GateStampFile = "specsync/gates-passed"

// GatesPassedAt reports whether the gate stamp in dir's repository names head.
func GatesPassedAt(ctx context.Context, dir, head string) bool {
	if head == "" || dir == "" {
		return false
	}
	path, err := gitIn(ctx, dir, "rev-parse", "--git-path", GateStampFile)
	if err != nil {
		return false
	}
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(data)) == head
}

// Position applies the flow to the facts: every step is marked done or not,
// and the first step that is not done is the current one. It reads nothing
// and runs nothing.
func (w Ways) Position(f FlowFacts) FlowPosition {
	flow := w.Flow
	if flow == "" {
		flow = "custom"
	}
	pos := FlowPosition{Flow: flow}
	for _, name := range w.Steps {
		s := w.evaluate(name, f)
		if s.Done {
			s.Command, s.Reason = "", ""
		}
		pos.Steps = append(pos.Steps, s)
	}
	for i := range pos.Steps {
		if !pos.Steps[i].Done {
			c := pos.Steps[i]
			pos.Current = &c
			return pos
		}
	}
	pos.Complete = true
	return pos
}

func (w Ways) evaluate(step string, f FlowFacts) StepState {
	s := StepState{Step: step}
	strict := w.Flow == "strict"
	slug := f.Slug
	if slug == "" {
		slug = "<slug>"
	}
	switch step {
	case "new":
		s.Done = f.HasChange
		s.Command = "specsync pull -issue <n>"
		s.Reason = "no change for this work; pull an issue, or write openspec/changes/<slug>/ and run specsync -change <slug>"
	case "start":
		s.Done = f.HasChange && f.Branch != ""
		if f.IssueID != "" {
			s.Command = "specsync pull -issue " + f.IssueID + " -worktree"
		} else {
			s.Command = "git worktree add ../worktrees/" + slug + " -b feat/" + slug
		}
		s.Reason = "the change has no branch or worktree yet"
	case "build":
		s.Done = f.TasksTotal > 0 && f.TasksDone == f.TasksTotal
		s.Reason = fmt.Sprintf("tasks %d/%d", f.TasksDone, f.TasksTotal)
		if f.TasksTotal == 0 {
			s.Reason = "no tasks listed; add them to openspec/changes/" + slug + "/tasks.md"
		}
	case "gate":
		s.Done = f.GatesPassed && !f.Dirty
		s.Reason = "gates not run for HEAD"
		if f.GatesPassed && f.Dirty {
			s.Reason = "uncommitted changes since the last gate run"
		}
		switch {
		case len(w.Gates) == 0:
			s.Reason += "; no gates configured: add gates: to openspec/specsync.yml"
		case !f.Trusted:
			s.Command = "specsync trust"
			s.Reason += "; the commands in " + SpecSyncConfigPath + " have not been trusted: review the commands first (specsync trust prints them)"
		default:
			s.Command = "specsync gates"
			if opt := w.optionalGateNames(); strict && len(opt) > 0 {
				s.Command = "specsync gates -with " + strings.Join(opt, ",")
			}
			s.Reason += "; specsync gates runs the configured gates and stamps the commit"
		}
	case "review":
		s.Done, s.Command, s.Reason = w.reviewState(f, strict)
	case "merge":
		s.Done = f.PR != nil && f.PR.State == "merged"
		s.Reason = "approved and green: merge the pull request"
		if f.PR != nil {
			s.Command = fmt.Sprintf("gh pr merge %d", f.PR.Number)
		} else if f.PRUnknown {
			s.Reason = unknownPR
		}
	case "cleanup":
		s.Done, s.Command, s.Reason = w.cleanupState(f)
	}
	return s
}

const unknownPR = "pull request state unknown (offline, or the lookup failed); run without -state=off to check"

func (w Ways) reviewState(f FlowFacts, strict bool) (done bool, command, reason string) {
	pr := f.PR
	slug := f.Slug
	if slug == "" {
		slug = "<slug>"
	}
	switch {
	case pr == nil && f.PRUnknown:
		return false, "", unknownPR
	case pr == nil:
		return false, "gh pr create", "no pull request yet; specsync pr-body -change " + slug + " prints a body"
	case pr.State == "merged":
		return true, "", ""
	case pr.State == "closed":
		return false, "", fmt.Sprintf("pull request #%d was closed without merging", pr.Number)
	case pr.Draft:
		return false, fmt.Sprintf("gh pr ready %d", pr.Number), fmt.Sprintf("pull request #%d is a draft", pr.Number)
	case pr.Checks.Fail > 0:
		return false, "", fmt.Sprintf("checks failing on #%d: %s", pr.Number, strings.Join(pr.Checks.Failing, ", "))
	case pr.Checks.Pending > 0:
		return false, "", fmt.Sprintf("checks still running on #%d", pr.Number)
	case pr.ReviewDecision == "changes_requested":
		return false, "", fmt.Sprintf("changes requested on #%d", pr.Number)
	case strict && (pr.UnresolvedThreads == nil || *pr.UnresolvedThreads > 0):
		return false, "", fmt.Sprintf("#%d has unresolved review threads or their count is unknown", pr.Number)
	case pr.ReviewDecision != "approved":
		return false, "", fmt.Sprintf("awaiting review on #%d", pr.Number)
	}
	return true, "", ""
}

func (w Ways) cleanupState(f FlowFacts) (done bool, command, reason string) {
	if f.PR == nil && f.PRUnknown {
		return false, "", unknownPR
	}
	if f.PR == nil || f.PR.State != "merged" {
		return false, "", "nothing to clean up until the pull request is merged"
	}
	var cmds []string
	if f.Worktree != "" {
		cmds = append(cmds, "git worktree remove "+f.Worktree)
	}
	if w.Cleanup.Archive && f.Stage != "archived" {
		cmds = append(cmds, "specsync archive -change "+f.Slug)
	}
	if len(cmds) == 0 {
		return true, "", ""
	}
	return false, strings.Join(cmds, " && "), "merged: remove the worktree and archive the change"
}
