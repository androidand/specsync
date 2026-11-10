package specsync

import "slices"

// TextConfig is the text standard for issues and pull requests: defaults,
// overridden by the text: section of openspec/specsync.yml.
type TextConfig struct {
	Profile        string // conventional (default) or plain
	PRTitle        int    // title length for pull requests and commits
	IssueTitle     int
	HeaderWords    int
	MaxHeaders     int
	SentenceWords  int
	ParagraphLines int
	IssueWords     int // visible words in an issue
	PRWords        int // visible words in a pull request body
	Headers        []string
	Filler         []string
	Template       string // path to a pull request template, relative to the repo root
}

var defaultHeaders = []string{
	"Why", "What", "Out of scope", "Related",
	"Release note", "Review focus", "Design notes", "Discoveries", "Original ask", "Tasks", "Plan changes",
}

var defaultFiller = []string{
	"basically", "simply", "just", "very", "actually", "obviously", "in order to", "it should be noted that", "please note",
}

// DefaultText returns the standard that applies when the repo sets nothing.
func DefaultText() TextConfig {
	return TextConfig{
		Profile: "conventional", PRTitle: 72, IssueTitle: 60,
		HeaderWords: 3, MaxHeaders: 7, SentenceWords: 20, ParagraphLines: 4,
		IssueWords: 150, PRWords: 200,
		Headers: slices.Clone(defaultHeaders), Filler: slices.Clone(defaultFiller),
	}
}

var textIntKeys = []string{"pr_title", "issue_title", "header_words", "max_headers", "sentence_words", "paragraph_lines", "issue_words", "pr_words"}

func (t *TextConfig) read(n *YNode, add addIssue) {
	if v := n.Str("profile"); v != "" {
		if v != "conventional" && v != "plain" {
			add(n.KeyLine["profile"], "text.profile", "profile must be conventional or plain, got %q", v)
		} else {
			t.Profile = v
		}
	}
	dst := map[string]*int{
		"pr_title": &t.PRTitle, "issue_title": &t.IssueTitle, "header_words": &t.HeaderWords, "max_headers": &t.MaxHeaders,
		"sentence_words": &t.SentenceWords, "paragraph_lines": &t.ParagraphLines, "issue_words": &t.IssueWords, "pr_words": &t.PRWords,
	}
	for _, k := range textIntKeys {
		if v := n.Str(k); v != "" {
			num, ok := parseCount(v)
			if !ok {
				add(n.KeyLine[k], "text."+k, "%s must be a whole number (0 turns the limit off), got %q", k, v)
				continue
			}
			*dst[k] = num
		}
	}
	for key, list := range map[string]*[]string{"headers": &t.Headers, "filler": &t.Filler} {
		c := n.Get(key)
		if c == nil {
			continue
		}
		if c.Kind != YList {
			add(n.KeyLine[key], "text."+key, "%s must be a list, for example [Non-goals, Impact]", key)
			continue
		}
		for _, item := range c.List {
			if item.Kind != YScalar || item.Scalar == "" {
				add(item.Line, "text."+key, "%s entries must be plain text", key)
				continue
			}
			if !slices.Contains(*list, item.Scalar) {
				*list = append(*list, item.Scalar)
			}
		}
	}
	t.Template = n.Str("template")
}

func parseCount(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			return 0, false
		}
	}
	return n, true
}
