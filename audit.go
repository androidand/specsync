package specsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuditFinding represents the result of auditing a single archived change.
type AuditFinding struct {
	Slug   string   // change slug
	Status string   // "unmerged", "shipped", or "orphaned"
	PR     *PRState // the matching PR, nil for orphaned
}

// AuditResult holds the findings from an audit run.
type AuditResult struct {
	Findings []AuditFinding
	Errors   []error // non-fatal errors during PR listing
}

// matchPRToChange returns true when the PR likely belongs to the given change slug.
// Matching strategies (priority order):
//  1. specsync marker comment in PR body
//  2. Branch name matches slug (exact, after prefix, or with issue number)
//  3. PR title contains the slug
func matchPRToChange(pr PRState, slug string) bool {
	// Strategy 1: specsync marker in body (highest priority, most reliable)
	if strings.Contains(pr.Body, marker(slug)) {
		return true
	}

	// Strategy 2: branch name matches slug
	branch := pr.HeadRefName
	if branch == slug {
		return true
	}

	// Check after every '/' in the branch name (handles "feat/slug",
	// "skein/feat/slug", and "skein/feat/52-slug" patterns)
	rest := branch
	for {
		idx := strings.IndexByte(rest, '/')
		if idx < 0 {
			break
		}
		after := rest[idx+1:]
		if after == slug {
			return true
		}
		// Convention pattern: "<issue>-<slug>" where issue is digits
		if strings.HasSuffix(after, slug) {
			prefix := after[:len(after)-len(slug)]
			if len(prefix) > 0 && prefix[len(prefix)-1] == '-' {
				beforeDash := prefix[:len(prefix)-1]
				if isDigits(beforeDash) && beforeDash != "0" {
					return true
				}
			}
		}
		rest = after
	}

	// Strategy 3: PR title contains the slug
	if strings.Contains(pr.Title, slug) {
		return true
	}

	return false
}

// isDigits reports whether s is a non-empty string of decimal digits.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Audit loads archived changes, queries GitHub for open and merged PRs, and
// classifies each archived change as unmerged (open PR), shipped (merged PR),
// or orphaned (no PR at all).
func Audit(ctx context.Context, provider *GitHubProvider, changes []Change) AuditResult {
	// Filter to only archived changes
	var archived []Change
	for _, c := range changes {
		if c.Archived {
			archived = append(archived, c)
		}
	}

	// Query PRs
	var result AuditResult
	openPRs, err := provider.ListOpenPRs(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("list open PRs: %w", err))
	}
	mergedPRs, err := provider.ListRecentMergedPRs(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("list merged PRs: %w", err))
	}

	var findings []AuditFinding
	for _, c := range archived {
		finding := AuditFinding{Slug: c.Slug}

		// Check for open (unmerged) PR first
		for i := range openPRs {
			if matchPRToChange(openPRs[i], c.Slug) {
				finding.Status = "unmerged"
				pr := openPRs[i]
				finding.PR = &pr
				break
			}
		}

		// If not unmerged, check for merged (shipped) PR
		if finding.Status == "" {
			for i := range mergedPRs {
				if matchPRToChange(mergedPRs[i], c.Slug) {
					finding.Status = "shipped"
					pr := mergedPRs[i]
					finding.PR = &pr
					break
				}
			}
		}

		// If neither, it's orphaned
		if finding.Status == "" {
			finding.Status = "orphaned"
		}

		findings = append(findings, finding)
	}

	result.Findings = findings
	return result
}

// HasUnmerged reports whether the result contains any unmerged findings.
func (r AuditResult) HasUnmerged() bool {
	for _, f := range r.Findings {
		if f.Status == "unmerged" {
			return true
		}
	}
	return false
}

// TaskAuditFinding represents the result of auditing a single change's tasks.
type TaskAuditFinding struct {
	Slug      string // change slug
	Unchecked int    // number of unchecked tasks
	Total     int    // total number of tasks
	HasCode   bool   // whether code files reference this change
	CodeRefs  int    // number of code references found
	Progress  string // task progress string
	Stage     string // current stage
	Note      string // additional note (e.g., "spun off", "external repo")
}

// TaskAuditResult holds the findings from a task audit run.
type TaskAuditResult struct {
	Findings   []TaskAuditFinding
	Mismatches []TaskAuditFinding // subset: unchecked tasks but code exists
}

// AuditTasks scans all changes and reports unchecked tasks, flagging mismatches
// where code exists but tasks remain unchecked (the dogfooding failure mode).
func AuditTasks(changes []Change) TaskAuditResult {
	var result TaskAuditResult

	for _, c := range changes {
		tc := countTaskStates(c.TasksMarkdown)
		if tc.LiveTotal() == 0 {
			continue
		}

		finding := TaskAuditFinding{
			Slug:      c.Slug,
			Unchecked: tc.Todo,
			Total:     tc.LiveTotal(),
			Progress:  string(c.Progress),
			Stage:     string(c.Stage),
		}

		if tc.Todo > 0 {
			if hasImplementationEvidence(c.Dir) {
				finding.HasCode = true
				finding.CodeRefs = 1
			}
		}

		result.Findings = append(result.Findings, finding)

		if tc.Todo > 0 && finding.HasCode {
			result.Mismatches = append(result.Mismatches, finding)
		}
	}

	return result
}

// HasMismatches reports whether the result contains any mismatches.
func (r TaskAuditResult) HasMismatches() bool {
	return len(r.Mismatches) > 0
}

// hasImplementationEvidence checks for strong evidence that code was written
// for this change. Relies on .specsync/metadata.json with stage "complete"
// or "implemented" — the only reliable signal that the change was actively
// worked on.
func hasImplementationEvidence(changeDir string) bool {
	metaPath := filepath.Join(changeDir, ".specsync", "metadata.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"stage":"complete"`) ||
			strings.Contains(content, `"stage":"implemented"`) ||
			strings.Contains(content, `"stage": "complete"`) ||
			strings.Contains(content, `"stage": "implemented"`) {
			return true
		}
	}

	return false
}
