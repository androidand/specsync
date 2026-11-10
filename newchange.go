package specsync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Slugify turns a title into a change slug.
func Slugify(title string) string { return slugify(title) }

const proposalTemplate = `# %s

## Why

(Say what is wrong today, in two sentences.)

## What

- (The change, one bullet per behaviour.)

## Out of scope

- (What this does not do.)
`

const tasksTemplate = "# Tasks\n\n- [ ] 1.1 (First step)\n"

// ScaffoldChange creates openspec/changes/<slug>/ with the short template and
// returns the slug. It never overwrites an existing change.
func ScaffoldChange(openspecDir, title string) (string, error) {
	title = strings.TrimSpace(title)
	slug := slugify(title)
	if slug == "" {
		return "", fmt.Errorf("the title needs some letters or digits to make a slug from")
	}
	dir := filepath.Join(openspecDir, "changes", slug)
	if _, err := os.Stat(dir); err == nil {
		return slug, fmt.Errorf("change %s already exists", slug)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return slug, err
	}
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte(fmt.Sprintf(proposalTemplate, title)), 0o644); err != nil {
		return slug, err
	}
	return slug, os.WriteFile(filepath.Join(dir, "tasks.md"), []byte(tasksTemplate), 0o644)
}

var titleFiller = map[string]bool{"a": true, "an": true, "the": true, "of": true, "to": true, "in": true, "on": true, "for": true,
	"and": true, "or": true, "with": true, "is": true, "be": true, "by": true, "at": true, "as": true, "it": true, "from": true}

// KeyWords returns a title's words without filler, lowercased, in order.
func KeyWords(title string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if w := b.String(); w != "" && !titleFiller[w] {
			out = append(out, w)
		}
		b.Reset()
	}
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// DuplicateCandidate is an existing change or issue to compare a title with.
type DuplicateCandidate struct {
	Kind  string `json:"kind"` // change or issue
	Ref   string `json:"ref"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// FindDuplicates returns the candidates that share at least two key words
// with the title and at least half of the title's key words, best first.
func FindDuplicates(title string, cands []DuplicateCandidate) []DuplicateCandidate {
	want := KeyWords(title)
	if len(want) == 0 {
		return nil
	}
	type scored struct {
		c DuplicateCandidate
		n int
	}
	var hits []scored
	for _, c := range cands {
		have := map[string]bool{}
		for _, w := range KeyWords(c.Title + " " + c.Ref) {
			have[w] = true
		}
		n := 0
		for _, w := range want {
			if have[w] {
				n++
			}
		}
		if n >= 2 && n*2 >= len(want) {
			hits = append(hits, scored{c, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].n > hits[j].n })
	out := make([]DuplicateCandidate, len(hits))
	for i, h := range hits {
		out[i] = h.c
	}
	return out
}

var scaffoldPlaceholders = []string{
	"(Say what is wrong today, in two sentences.)",
	"(The change, one bullet per behaviour.)",
	"(What this does not do.)",
}

// HasScaffoldPlaceholders reports whether any template placeholder is still in
// the proposal text.
func HasScaffoldPlaceholders(text string) bool {
	for _, p := range scaffoldPlaceholders {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}
