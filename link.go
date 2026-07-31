package specsync

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// reBareIssue matches "#N" or "N" (bare issue number).
var reBareIssue = regexp.MustCompile(`^#?\d+$`)

// LinkOptions configures a link run: establishing cross-references between
// local specs and/or existing issues so their GitHub issues reference each
// other. Arguments may be slugs, issue references (#N, owner/repo#N, URL),
// or a mix of both.
type LinkOptions struct {
	OpenSpecDir string   // path to the openspec/ directory
	Args        []string // slugs and/or issue references (at least 2)
	Repo        string   // -repo flag; resolves bare #N against this repo
	DryRun      bool
}

// LinkedPair describes one spec after linking — the caller uses Repo to
// construct the right provider and sync the updated body to GitHub.
type LinkedPair struct {
	Slug string
	Dir  string
	Repo string // "owner/name" from the ref key, or "" for auto-detect
	Ref  Ref    // the issue ref to update
}

// LinkedRef describes one issue reference after linking — the caller uses Repo
// and ID to construct the right provider and edit the issue body.
type LinkedRef struct {
	Repo string // "owner/name"
	ID   string // issue number
	Ref  Ref    // the issue ref to update
}

// LinkResult groups the output of a link run: slug pairs and reference edits.
type LinkResult struct {
	Pairs []LinkedPair
	Refs  []LinkedRef
}

// linkEntryKind is the classified kind of a link argument.
type linkEntryKind int

const (
	kindSlug linkEntryKind = iota
	kindIssueRef // #N, owner/repo#N, URL
)

// linkEntry is a classified argument.
type linkEntry struct {
	kind    linkEntryKind
	ref     Ref
	slug    string // change slug (slug entries only)
	slugDir string // change dir (slug entries only)
}

// Link resolves cross-references between local specs and/or existing issues.
// For slug arguments, it writes links.md and returns LinkedPairs for the
// caller to sync. For issue-reference arguments, it returns LinkedRefs for
// the caller to edit the issue body directly.
func Link(ctx context.Context, opts LinkOptions) (*LinkResult, error) {
	if len(opts.Args) < 2 {
		return nil, fmt.Errorf("link: at least 2 arguments required")
	}

	entries, err := classifyEntries(opts)
	if err != nil {
		return nil, err
	}

	// Build the set of all refs (from both slugs and references).
	allRefs := make([]Ref, 0, len(entries))
	for _, e := range entries {
		allRefs = append(allRefs, e.ref)
	}

	result := &LinkResult{}

	// Slug path: write links.md, build pairs.
	for _, e := range entries {
		if e.kind != kindSlug {
			continue
		}
		links := refsExcept(allRefs, e.ref)
		if !opts.DryRun {
			if err := saveLinksToMD(e.slugDir, links, nil, nil); err != nil {
				return nil, fmt.Errorf("save links for %s: %w", e.slugDir, err)
			}
		}
		result.Pairs = append(result.Pairs, LinkedPair{
			Slug: e.slug,
			Dir:  e.slugDir,
			Repo: repoFromKey(e.ref.Provider),
			Ref:  e.ref,
		})
	}

	// Reference path: build LinkedRefs for direct body edit.
	for _, e := range entries {
		if e.kind != kindIssueRef {
			continue
		}
		repo := repoFromKey(e.ref.Provider)
		result.Refs = append(result.Refs, LinkedRef{
			Repo: repo,
			ID:   e.ref.ID,
			Ref:  e.ref,
		})
	}

	return result, nil
}

// classifyEntries classifies each argument as a slug or issue reference.
func classifyEntries(opts LinkOptions) ([]linkEntry, error) {
	entries := make([]linkEntry, 0, len(opts.Args))
	for _, arg := range opts.Args {
		e, err := classifyArg(arg, opts.OpenSpecDir, opts.Repo)
		if err != nil {
			return nil, fmt.Errorf("classify %q: %w", arg, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// classifyArg classifies one argument as slug or issue reference.
func classifyArg(arg, openspecDir, repo string) (linkEntry, error) {
	// Full URL.
	if strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") {
		if ref := refFromURL(arg); ref != nil && ref.Provider != "" {
			return linkEntry{kind: kindIssueRef, ref: *ref}, nil
		}
	}

	// GitHub shorthand: owner/repo#N.
	if reShorthand.MatchString(arg) {
		idx := strings.LastIndex(arg, "#")
		repoPath := arg[:idx]
		num := arg[idx+1:]
		url := "https://github.com/" + repoPath + "/issues/" + num
		return linkEntry{kind: kindIssueRef, ref: Ref{
			Provider: "github:" + repoPath,
			ID:       num,
			URL:      url,
		}}, nil
	}

	// Bare #N or N (issue number in the resolved repo).
	if reBareIssue.MatchString(arg) {
		num := strings.TrimPrefix(arg, "#")
		if repo == "" {
			return linkEntry{}, fmt.Errorf("bare issue %q requires -repo flag or auto-detected repo", arg)
		}
		url := "https://github.com/" + repo + "/issues/" + num
		return linkEntry{kind: kindIssueRef, ref: Ref{
			Provider: "github:" + repo,
			ID:       num,
			URL:      url,
		}}, nil
	}

	// Slug: try to resolve as a local change.
	c, err := LoadChangeBySlug(openspecDir, arg)
	if err != nil {
		return linkEntry{}, fmt.Errorf("%q is not a known change slug", arg)
	}
	refs, err := loadRefs(c.Dir)
	if err != nil {
		return linkEntry{}, err
	}
	if len(refs) == 0 {
		return linkEntry{}, fmt.Errorf("change %q has no synced ref; run specsync -change %s first", arg, arg)
	}
	_, ref := firstRef(refs)
	return linkEntry{kind: kindSlug, ref: ref, slug: c.Slug, slugDir: c.Dir}, nil
}

// refsExcept returns all refs except the one that matches the given ref.
func refsExcept(all []Ref, except Ref) []Ref {
	var out []Ref
	for _, r := range all {
		if r.ID == except.ID && r.Provider == except.Provider {
			continue
		}
		out = append(out, r)
	}
	return out
}

// UpsertRelatedSection replaces an existing "## Related" block in-place (up
// to the next "##" heading or EOF), or appends one at the end if absent. It
// is idempotent: re-running produces the same result.
func UpsertRelatedSection(body string, links []Ref) string {
	if len(links) == 0 {
		return body
	}

	relatedHeader := "\n\n## Related\n\n"
	idx := strings.Index(body, relatedHeader)
	if idx >= 0 {
		// Find the end of the existing Related block — next "##" heading or EOF.
		rest := body[idx+len(relatedHeader):]
		endIdx := strings.Index(rest, "\n\n## ")
		if endIdx < 0 {
			endIdx = len(rest)
		}
		// Replace the block content. Preserve the original block's trailing
		// newline when there's no next section; when there IS a next section,
		// the \n\n before it provides spacing so no trailing newline is needed.
		newBlock := strings.Join(refLabels(links), "\n")
		if endIdx >= len(rest) && endIdx > 0 && rest[endIdx-1] == '\n' {
			newBlock += "\n"
		}
		return body[:idx+len(relatedHeader)] + newBlock + rest[endIdx:]
	}

	// Append new block at the end.
	newBlock := strings.Join(refLabels(links), "\n")
	return body + relatedHeader + newBlock + "\n"
}

// UpsertDependencySections appends or replaces "## Related", "## Blocked by",
// and "## Blocks" sections in body. It uses UpsertRelatedSection for the
// Related section, then appends Blocked by and Blocks as needed.
func UpsertDependencySections(body string, links, blockedBy, blocks []Ref) string {
	if len(links) == 0 && len(blockedBy) == 0 && len(blocks) == 0 {
		return body
	}

	body = UpsertRelatedSection(body, links)

	if len(blockedBy) > 0 {
		body = upsertSection(body, "## Blocked by", refLabels(blockedBy))
	}

	if len(blocks) > 0 {
		body = upsertSection(body, "## Blocks", refLabels(blocks))
	}

	return body
}

// upsertSection replaces or appends a section header with the given lines.
func upsertSection(body, header string, lines []string) string {
	headerBlock := "\n\n" + header + "\n\n"
	idx := strings.Index(body, headerBlock)
	if idx >= 0 {
		// Find the end of the existing block — next "##" heading or EOF.
		rest := body[idx+len(headerBlock):]
		endIdx := strings.Index(rest, "\n\n## ")
		if endIdx < 0 {
			endIdx = len(rest)
		}
		newBlock := strings.Join(lines, "\n")
		if endIdx >= len(rest) && endIdx > 0 && rest[endIdx-1] == '\n' {
			newBlock += "\n"
		}
		return body[:idx+len(headerBlock)] + newBlock + rest[endIdx:]
	}

	// Append new block at the end.
	newBlock := strings.Join(lines, "\n")
	return body + headerBlock + newBlock + "\n"
}

// refLabels returns formatted link labels for a list of Refs.
func refLabels(links []Ref) []string {
	var lines []string
	for _, ref := range links {
		lines = append(lines, "- "+refLabel(ref))
	}
	return lines
}

// firstRef picks the ref to link against: canonical "github:owner/repo" keys
// first (the bare "github" key is a pre-migration relic and may be stale),
// then the bare key, then anything else. Keys are scanned in sorted order so
// the pick is deterministic when several remain.
func firstRef(refs map[string]Ref) (string, Ref) {
	keys := make([]string, 0, len(refs))
	for k := range refs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, "github:") {
			return k, refs[k]
		}
	}
	if r, ok := refs["github"]; ok {
		return "github", r
	}
	for _, k := range keys {
		return k, refs[k]
	}
	return "", Ref{}
}

// repoFromKey extracts "owner/name" from "github:owner/name", returning ""
// for the plain "github" key (meaning auto-detect from git remote).
func repoFromKey(key string) string {
	const prefix = "github:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return ""
}
