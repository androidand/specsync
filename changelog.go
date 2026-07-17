package specsync

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// ChangelogCategory is a Keep a Changelog section. Ordering follows the
// keepachangelog.com 1.1 convention.
type ChangelogCategory string

const (
	CatAdded    ChangelogCategory = "Added"
	CatChanged  ChangelogCategory = "Changed"
	CatFixed    ChangelogCategory = "Fixed"
	CatRemoved  ChangelogCategory = "Removed"
	CatSecurity ChangelogCategory = "Security"
)

var categoryOrder = []ChangelogCategory{CatAdded, CatChanged, CatFixed, CatRemoved, CatSecurity}

// ChangelogEntry is one published line: a shipped change (Slug set) or a loose
// commit (Hash set). Text is user language — the release note, the proposal
// title, or the commit subject — never a raw commit dump.
type ChangelogEntry struct {
	Category ChangelogCategory
	Text     string
	Breaking bool
	Slug     string   // shipped change entries
	IssueIDs []string // provider issue numbers bound to the change
	Hash     string   // loose-commit entries
}

// Changelog is a built release section plus the honesty counters: what was
// omitted is reported, never silent.
type Changelog struct {
	Entries        []ChangelogEntry
	OmittedCommits int // loose plumbing commits (chore/docs/ci/...) left out
}

// BuildChangelog turns gathered trace input into one release section:
// one entry per shipped change (a change with at least one linked in-range
// commit), plus honest fallback entries for loose feat/fix/breaking commits.
// deltasBySlug carries each shipped change's OpenSpec requirement deltas; the
// map may be nil or sparse when the OpenSpec CLI is unavailable.
func BuildChangelog(in TraceInput, deltasBySlug map[string][]OpenSpecDelta) Changelog {
	// A commit that was reverted inside the same range is a net no-op: the
	// release doesn't contain its behavior, so neither commit may publish an
	// entry (or count as an unlinked commit).
	in.Commits = cancelRevertPairs(in.Commits)

	// Which commits link to which change — same evidence rule as ResolveTrace:
	// a commit belongs to the first change whose bound issue ids it references.
	issueToChange := map[string]string{}
	for _, cr := range in.Changes {
		for _, id := range cr.IssueIDs {
			issueToChange[id] = cr.Change.Slug
		}
	}

	commitsBySlug := map[string][]Commit{}
	var loose []Commit
	for _, c := range in.Commits {
		slug := ""
		for _, ref := range append(append([]string{}, c.IssueRefs...), c.PRRefs...) {
			if s, ok := issueToChange[refNumber(ref)]; ok {
				slug = s
				break
			}
		}
		if slug == "" {
			loose = append(loose, c)
			continue
		}
		commitsBySlug[slug] = append(commitsBySlug[slug], c)
	}

	var cl Changelog
	for _, cr := range in.Changes {
		commits := commitsBySlug[cr.Change.Slug]
		if len(commits) == 0 {
			continue // not shipped in this range
		}
		// IssueIDs come from a map iteration upstream; sort so rendered refs
		// (and -apply re-runs) are byte-identical across runs.
		ids := append([]string(nil), cr.IssueIDs...)
		sort.Slice(ids, func(i, j int) bool {
			if len(ids[i]) != len(ids[j]) {
				return len(ids[i]) < len(ids[j]) // numeric ids: shorter is smaller
			}
			return ids[i] < ids[j]
		})
		cl.Entries = append(cl.Entries, ChangelogEntry{
			Category: categorize(deltasBySlug[cr.Change.Slug], commits),
			Text:     ReleaseNote(cr.Change),
			Breaking: anyBreaking(deltasBySlug[cr.Change.Slug], commits),
			Slug:     cr.Change.Slug,
			IssueIDs: ids,
		})
	}

	for _, c := range loose {
		entry, ok := looseEntry(c)
		if !ok {
			if c.ConventionalOK {
				cl.OmittedCommits++
			}
			continue
		}
		cl.Entries = append(cl.Entries, entry)
	}

	sortEntries(cl.Entries)
	return cl
}

// cancelRevertPairs drops every commit that is reverted by a later commit in
// the same range, together with the revert itself. Reverts are matched
// newest-first so a revert-of-revert chain resolves to its net effect (A,
// revert(A)=B, revert(B)=C leaves A: C cancels B, and A stands). A revert
// whose target is outside the range is kept — relative to the previous
// release it really does change behavior. Hashes match on the shorter
// prefix, since revert bodies may carry abbreviated hashes.
func cancelRevertPairs(commits []Commit) []Commit {
	consumed := make([]bool, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		if consumed[i] || commits[i].RevertsHash == "" {
			continue
		}
		for j := range commits {
			if j == i || consumed[j] || !hashMatches(commits[j].Hash, commits[i].RevertsHash) {
				continue
			}
			consumed[i], consumed[j] = true, true
			break
		}
	}
	var kept []Commit
	for i, c := range commits {
		if !consumed[i] {
			kept = append(kept, c)
		}
	}
	return kept
}

// hashMatches reports whether two commit hashes name the same commit,
// tolerating abbreviation on either side (minimum 7 characters).
func hashMatches(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(a) < 7 || len(b) < 7 {
		return a != "" && a == b
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return strings.HasPrefix(b, a)
}

// releaseNoteRE finds a "## Release note" heading (any case, optional "s").
var releaseNoteRE = regexp.MustCompile(`(?im)^##\s+release notes?\s*$`)

// ReleaseNote returns the change's published one-liner: the body of the
// proposal's "## Release note" section when present (whitespace-collapsed),
// else the proposal title. Authored at planning time, reviewed in the issue.
func ReleaseNote(c Change) string {
	loc := releaseNoteRE.FindStringIndex(c.Body)
	if loc == nil {
		return c.Title
	}
	rest := c.Body[loc[1]:]
	if i := strings.Index(rest, "\n## "); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.Index(rest, "\n# "); i >= 0 {
		rest = rest[:i]
	}
	note := strings.Join(strings.Fields(rest), " ")
	if note == "" {
		return c.Title
	}
	return note
}

// categorize picks the Keep a Changelog section for a shipped change. Delta
// signals outrank commit signals; Removed > Added > Changed. With no deltas, an
// all-fix change is Fixed, any feat makes it Added, anything else is Changed.
func categorize(deltas []OpenSpecDelta, commits []Commit) ChangelogCategory {
	var added, modified, removed bool
	for _, d := range deltas {
		switch d.Operation {
		case "ADDED":
			added = true
		case "MODIFIED":
			modified = true
		case "REMOVED":
			removed = true
		}
	}
	switch {
	case removed:
		return CatRemoved
	case added:
		return CatAdded
	case modified:
		return CatChanged
	}

	allFix, anyFeat, anyConventional := true, false, false
	for _, c := range commits {
		if !c.ConventionalOK {
			continue
		}
		anyConventional = true
		if c.Type != "fix" {
			allFix = false
		}
		if c.Type == "feat" {
			anyFeat = true
		}
	}
	switch {
	case anyConventional && allFix:
		return CatFixed
	case anyFeat:
		return CatAdded
	default:
		return CatChanged
	}
}

func anyBreaking(deltas []OpenSpecDelta, commits []Commit) bool {
	for _, d := range deltas {
		if d.Operation == "REMOVED" {
			return true
		}
	}
	for _, c := range commits {
		if c.Breaking {
			return true
		}
	}
	return false
}

// looseEntry maps an unlinked commit to a published entry. Only user-facing
// types make the cut: feat → Added, fix → Fixed, breaking → Changed. Plumbing
// (chore, docs, ci, ...) and non-conventional commits return ok=false.
func looseEntry(c Commit) (ChangelogEntry, bool) {
	if !c.ConventionalOK {
		return ChangelogEntry{}, false
	}
	e := ChangelogEntry{Text: c.Description, Breaking: c.Breaking, Hash: c.Hash}
	switch {
	case c.Breaking:
		e.Category = CatChanged
	case c.Type == "feat":
		e.Category = CatAdded
	case c.Type == "fix":
		e.Category = CatFixed
	default:
		return ChangelogEntry{}, false
	}
	return e, true
}

// sortEntries orders deterministically: category order, then changes before
// loose commits, then by slug/hash — repeated runs are byte-identical.
func sortEntries(entries []ChangelogEntry) {
	rank := map[ChangelogCategory]int{}
	for i, c := range categoryOrder {
		rank[c] = i
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if rank[a.Category] != rank[b.Category] {
			return rank[a.Category] < rank[b.Category]
		}
		if (a.Slug != "") != (b.Slug != "") {
			return a.Slug != ""
		}
		if a.Slug != b.Slug {
			return a.Slug < b.Slug
		}
		return a.Hash < b.Hash
	})
}

// RenderChangelogSection renders one Keep a Changelog version section.
// version is bare ("0.6.0"); empty means an Unreleased section, which per the
// Keep a Changelog convention carries no date. Loose-commit entries carry a
// short hash; change entries carry issue refs.
func RenderChangelogSection(cl Changelog, version, date string) string {
	var b strings.Builder
	header := "## [" + version + "]"
	if version == "" {
		header = "## [Unreleased]"
	}
	if date != "" && version != "" {
		header += " - " + date
	}
	b.WriteString(header + "\n")

	for _, cat := range categoryOrder {
		var lines []string
		for _, e := range cl.Entries {
			if e.Category != cat {
				continue
			}
			line := "- "
			if e.Breaking {
				line += "**Breaking:** "
			}
			line += e.Text
			var refs []string
			for _, id := range e.IssueIDs {
				refs = append(refs, "#"+id)
			}
			if e.Hash != "" {
				refs = append(refs, shortHash(e.Hash))
			}
			if len(refs) > 0 {
				line += " (" + strings.Join(refs, ", ") + ")"
			}
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString("\n### " + string(cat) + "\n\n")
		b.WriteString(strings.Join(lines, "\n") + "\n")
	}

	if cl.OmittedCommits > 0 {
		b.WriteString(fmt.Sprintf("\n<!-- %d internal commit(s) omitted (chore/docs/ci/...) -->\n", cl.OmittedCommits))
	}
	return b.String()
}

const changelogHeader = `# Changelog

All notable changes to this project are documented here. One entry per shipped
OpenSpec change — see the linked issues for the full spec and discussion.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
`

// sectionBounds locates the "## [version]" section in content: the byte range
// from its heading line through the start of the next "## " heading (or EOF).
// ok is false when no section for that version exists.
func sectionBounds(content, version string) (start, end int, ok bool) {
	heading := "## [" + version + "]"
	idx := -1
	if strings.HasPrefix(content, heading) {
		idx = 0
	} else if i := strings.Index(content, "\n"+heading); i >= 0 {
		idx = i + 1
	}
	if idx < 0 {
		return 0, 0, false
	}
	rest := content[idx:]
	if i := strings.Index(rest, "\n## "); i >= 0 {
		return idx, idx + i + 1, true
	}
	return idx, len(content), true
}

// ApplyChangelog writes section (from RenderChangelogSection) for version into
// the CHANGELOG.md at path, idempotently: it creates the file with the standard
// header when absent, replaces an existing section for the same version in
// place, and otherwise prepends the section above the first existing "## "
// heading. An empty version manages the "Unreleased" section. Other sections
// are left byte-identical.
func ApplyChangelog(path, version, section string) error {
	section = strings.TrimRight(section, "\n") + "\n"
	heading := version
	if heading == "" {
		heading = "Unreleased" // must match RenderChangelogSection's header
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", path, err)
		}
		return os.WriteFile(path, []byte(changelogHeader+"\n"+section), 0o644)
	}

	content := string(existing)
	if start, end, ok := sectionBounds(content, heading); ok {
		trailer := ""
		if end < len(content) {
			trailer = "\n" // keep a blank line before the next section
		}
		content = content[:start] + section + trailer + content[end:]
	} else if strings.HasPrefix(content, "## ") {
		content = section + "\n" + content
	} else if i := strings.Index(content, "\n## "); i >= 0 {
		content = content[:i+1] + section + "\n" + content[i+1:]
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + section
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
