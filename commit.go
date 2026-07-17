package specsync

import (
	"regexp"
	"strings"
)

// Commit is a parsed Git commit. Conventional Commit fields are populated when
// the message conforms (ConventionalOK); otherwise the raw message is preserved
// and ConventionalOK is false. Parsing is pure: no I/O, no dependency. It
// extracts only what linking and the follow-up report need — type, breaking
// signal, and issue/PR references — and deliberately stops short of a full
// commit linter.
type Commit struct {
	Hash           string
	Type           string // feat, fix, ... ("" when not conventional)
	Scope          string // optional scope inside type(scope)
	Description    string // the header description (or the raw header when not conventional)
	Body           string // everything after the header's trailing blank line
	Breaking       bool   // from a "!" marker or a BREAKING CHANGE footer
	BreakingFooter string // text following BREAKING CHANGE:, when present
	IssueRefs      []string
	PRRefs         []string
	Author         string
	Date           string
	Raw            string
	ConventionalOK bool
	// RevertsHash is the hash named by a "This reverts commit <hash>." body
	// line (git's standard revert message), "" otherwise. Lets the changelog
	// cancel a commit against its in-range revert instead of publishing an
	// entry for behavior the release doesn't contain.
	RevertsHash string
}

// headerRE matches a Conventional Commits 1.0.0 header: type(scope)!: description.
// A colon followed by at least one space is required, per the spec.
var headerRE = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^)]*)\))?(!)?:[ \t]+(.+)$`)

// breakingRE matches a BREAKING CHANGE / BREAKING-CHANGE footer and captures its value.
var breakingRE = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:[ \t]*(.*)$`)

// revertsRE matches git's standard revert body line "This reverts commit <hash>."
var revertsRE = regexp.MustCompile(`(?im)^this reverts commit ([0-9a-f]{7,40})\.?\s*$`)

// trailingPRRE matches the squash-merge convention of a trailing "(#123)" in the header.
var trailingPRRE = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// refRE matches an issue/PR reference: "#123" or "owner/repo#123".
var refRE = regexp.MustCompile(`(?:([\w.-]+/[\w.-]+))?#(\d+)`)

// closeKeywordRE matches a closing keyword preceding a reference (these are issues).
var closeKeywordRE = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+(?:([\w.-]+/[\w.-]+))?#(\d+)`)

// trailerRefRE matches a trailer-style line that is nothing but a reference
// keyword and a ref, e.g. "Refs: #35" or "See-also: #12" — deliberate evidence,
// as opposed to a bare "#N" mentioned in passing within narrative prose.
var trailerRefRE = regexp.MustCompile(`(?im)^[ \t]*(?:refs?|see-also|related)[ \t]*:?[ \t]+(?:([\w.-]+/[\w.-]+))?#(\d+)[ \t]*$`)

// ParseCommit parses a commit message (and optional metadata) into a Commit. It
// never errors: a non-conventional message yields ConventionalOK=false with the
// raw text preserved, because in this tool the messy message is the normal case.
func ParseCommit(hash, author, date, message string) Commit {
	c := Commit{Hash: hash, Author: author, Date: date, Raw: message}

	lines := strings.Split(message, "\n")
	header := ""
	if len(lines) > 0 {
		header = strings.TrimRight(lines[0], "\r")
	}

	// Body is everything after the first blank line following the header.
	if i := strings.Index(message, "\n\n"); i >= 0 {
		c.Body = strings.TrimSpace(message[i+2:])
	}

	if m := headerRE.FindStringSubmatch(header); m != nil {
		c.ConventionalOK = true
		c.Type = m[1]
		c.Scope = m[2]
		c.Breaking = m[3] == "!"
		c.Description = strings.TrimSpace(m[4])
	} else {
		c.Description = strings.TrimSpace(header)
	}

	// A BREAKING CHANGE footer marks the commit breaking regardless of the "!".
	if bm := breakingRE.FindStringSubmatch(message); bm != nil {
		c.Breaking = true
		c.BreakingFooter = strings.TrimSpace(bm[1])
	}

	if rm := revertsRE.FindStringSubmatch(message); rm != nil {
		c.RevertsHash = strings.ToLower(rm[1])
	}

	c.IssueRefs, c.PRRefs = extractRefs(header, message)
	return c
}

// extractRefs pulls issue and PR references out of a commit. The squash-merge
// trailing "(#N)" in the header is treated as a PR. A same-repo bare "#N" is
// only accepted as issue evidence when it appears somewhere deliberate — after
// a closing keyword or on its own trailer line — because a bare number can
// also show up incidentally in narrative prose (e.g. quoting another repo's
// issue number without its "owner/repo" prefix), which is not evidence of a
// link in this repo. A "owner/repo#N" reference is unambiguous regardless of
// where it appears, since it can't collide with a same-repo issue number. The
// trace layer reconciles the exact kind against the tracker, which actually
// knows whether a number is an issue or a PR.
func extractRefs(header, message string) (issues, prs []string) {
	seenIssue := map[string]bool{}
	seenPR := map[string]bool{}

	addPR := func(ref string) {
		if !seenPR[ref] {
			seenPR[ref] = true
			prs = append(prs, ref)
		}
	}
	addIssue := func(ref string) {
		if !seenIssue[ref] {
			seenIssue[ref] = true
			issues = append(issues, ref)
		}
	}

	// Trailing "(#N)" in the header → PR.
	if pm := trailingPRRE.FindStringSubmatch(header); pm != nil {
		addPR("#" + pm[1])
	}

	// Closing keywords always denote issues, wherever they appear.
	for _, m := range closeKeywordRE.FindAllStringSubmatch(message, -1) {
		addIssue(refString(m[1], m[2]))
	}

	// A genuine trailer line ("Refs: #N", "See-also: #N") is deliberate
	// evidence too, even without a closing keyword.
	for _, m := range trailerRefRE.FindAllStringSubmatch(message, -1) {
		addIssue(refString(m[1], m[2]))
	}

	// A cross-repo "owner/repo#N" reference counts regardless of position. A
	// bare "#N" outside the cases above is narrative prose, not linking
	// evidence, and is deliberately dropped here.
	for _, m := range refRE.FindAllStringSubmatch(message, -1) {
		if m[1] == "" {
			continue
		}
		ref := refString(m[1], m[2])
		if !seenPR[ref] {
			addIssue(ref)
		}
	}
	return issues, prs
}

func refString(repo, num string) string {
	if repo != "" {
		return repo + "#" + num
	}
	return "#" + num
}
