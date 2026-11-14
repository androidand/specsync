package specsync

import (
	"sort"
	"strings"
)

// OpenIssue is an open tracker item a new issue might duplicate.
type OpenIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// PriorArtMatch is an open issue that shares distinctive words with a title.
type PriorArtMatch struct {
	Number int
	Title  string
	URL    string
	Shared []string
}

// priorArtStop holds words common enough in issue titles to say nothing about
// whether two issues cover the same work.
var priorArtStop = map[string]bool{
	"this": true, "that": true, "with": true, "from": true, "into": true, "when": true, "then": true,
	"have": true, "should": true, "does": true, "more": true, "than": true, "also": true,
	"make": true, "using": true, "change": true, "changes": true,
	"update": true, "support": true, "issue": true, "issues": true, "work": true, "need": true, "needs": true,
	"after": true, "before": true, "instead": true, "about": true, "over": true, "only": true, "every": true,
}

// DistinctiveWords returns the lowercased words of s that are at least four
// characters and not in the stopword list, deduplicated, in order.
func DistinctiveWords(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range KeyWords(s) {
		if len(w) < 4 || priorArtStop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// RankPriorArt returns up to max open issues that share at least one
// distinctive word with title, most shared words first. Matching on shared
// jargon rather than title similarity is deliberate: two people describing one
// problem rarely phrase it alike, but they usually name the same component.
func RankPriorArt(title string, open []OpenIssue, max int) []PriorArtMatch {
	want := DistinctiveWords(title)
	if len(want) == 0 || max <= 0 {
		return nil
	}
	var hits []PriorArtMatch
	for _, is := range open {
		have := map[string]bool{}
		for _, w := range DistinctiveWords(is.Title) {
			have[w] = true
		}
		var shared []string
		for _, w := range want {
			if have[w] {
				shared = append(shared, w)
			}
		}
		if len(shared) > 0 {
			hits = append(hits, PriorArtMatch{Number: is.Number, Title: is.Title, URL: is.URL, Shared: shared})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return len(hits[i].Shared) > len(hits[j].Shared) })
	if len(hits) > max {
		hits = hits[:max]
	}
	return hits
}

// SharedList renders the shared words for display.
func (m PriorArtMatch) SharedList() string { return strings.Join(m.Shared, ", ") }
