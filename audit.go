package specsync

import (
	"context"
	"strings"
)

// AuditFinding represents the result of auditing a single archived change.
type AuditFinding struct {
	Slug   string      // change slug
	Status string      // "unmerged", "shipped", or "orphaned"
	PR     *PRState    // the matching PR, nil for orphaned
}

// AuditResult holds the findings from an audit run.
type AuditResult struct {
	Findings []AuditFinding
}

// matchPRToChange returns true when the PR likely belongs to the given change slug.
// Matching strategies (priority order):
//  1. specsync marker comment in PR body
//  2. Branch name starts with the slug (exact or with prefix/suffix)
//  3. PR title contains the slug
func matchPRToChange(pr PRState, slug string) bool {
	// Strategy 1: specsync marker in body (highest priority, most reliable)
	if strings.Contains(pr.Body, marker(slug)) {
		return true
	}

	// Strategy 2: branch name prefix
	// Match exact branch name or branch with prefix like "skein/" or "feature/"
	branch := pr.HeadRefName
	if branch == slug {
		return true
	}
	if idx := strings.IndexByte(branch, '/'); idx > 0 {
		if branch[idx+1:] == slug {
			return true
		}
	}

	// Strategy 3: PR title contains the slug
	if strings.Contains(pr.Title, slug) {
		return true
	}

	return false
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
	openPRs, _ := provider.ListOpenPRs(ctx)
	mergedPRs, _ := provider.ListRecentMergedPRs(ctx)

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

	return AuditResult{Findings: findings}
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
