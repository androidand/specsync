package specsync

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// LintFinding is one departure from the text standard. Lint only reports; it
// never changes the text it reads.
type LintFinding struct {
	Line       int    `json:"line"`
	Rule       string `json:"rule"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

var (
	conventionalTitle = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([^)]+\))?!?: \S`)
	fileNameInTitle   = regexp.MustCompile(`[\w./-]+\.(go|md|yml|yaml|json|js|ts|sh|toml)\b`)
	inlineComment     = regexp.MustCompile(`<!--.*?-->`)
	sliceTitle        = regexp.MustCompile(`^Slice \d+:`)
	headingLine       = regexp.MustCompile(`^(#{2,6})\s+(.*?)(?:\s+#+)?\s*$`)
	listItemLine      = regexp.MustCompile(`^\s*([-*+]|\d+[.)])\s+`)
	checkboxLine      = regexp.MustCompile(`^\s*[-*+]\s+\[[ xX]\]`)
	markdownLink      = regexp.MustCompile(`!?\[[^\]]*\]\([^)]*\)`)
	bareURL           = regexp.MustCompile(`https?://\S+`)
)

// LintTitle checks a title. kind is pr, commit or issue.
func LintTitle(kind, title string, t TextConfig) []LintFinding {
	var out []LintFinding
	add := func(rule, msg, sug string) {
		out = append(out, LintFinding{Line: 1, Rule: rule, Message: msg, Suggestion: sug})
	}
	n := len([]rune(title))
	if kind == "issue" {
		if t.IssueTitle > 0 && n > t.IssueTitle {
			sug, _ := shortenTitle(title)
			if sug == title {
				sug = ""
			}
			add("title-length", fmt.Sprintf("issue title is %d characters, the limit is %d", n, t.IssueTitle), sug)
		}
		if !sentenceCase(title) {
			add("sentence-case", "issue titles are in sentence case: one capital at the start", "")
		}
		if m := fileInTitle(title); m != "" {
			add("no-file-names", fmt.Sprintf("issue titles state the outcome, not a file: %q", m), "")
		}
		return out
	}
	if t.PRTitle > 0 && n > t.PRTitle {
		add("title-length", fmt.Sprintf("title is %d characters, the limit is %d", n, t.PRTitle), "")
	}
	if t.Profile != "plain" && !conventionalTitle.MatchString(title) {
		add("conventional-commit", "title is not in Conventional Commits form", "type(scope): subject, for example fix(sync): keep links out of sections")
	}
	if strings.HasSuffix(title, ".") {
		add("title-period", "a title does not end with a period", strings.TrimRight(title, "."))
	}
	return out
}

var productNames = []string{"next", "vue", "node", "chart", "three", "express", "react", "d3", "ember", "backbone", "nuxt", "alpine", "p5", "socket"}

// fileInTitle returns the first file name in a title. A name with a path
// separator or a source or doc extension is a file; `<word>.js` is a file only
// when the word is lower case and not a well-known product such as Next.js.
func fileInTitle(title string) string {
	for _, m := range fileNameInTitle.FindAllString(title, -1) {
		if !strings.Contains(m, "/") && strings.HasSuffix(m, ".js") {
			base := strings.TrimSuffix(m, ".js")
			if r := []rune(base); len(r) > 0 && unicode.IsUpper(r[0]) {
				continue
			}
			if slices.Contains(productNames, strings.ToLower(base)) {
				continue
			}
		}
		return m
	}
	return ""
}

func sentenceCase(title string) bool {
	words := strings.Fields(title)
	if len(words) == 0 {
		return true
	}
	first := []rune(words[0])[0]
	if unicode.IsLetter(first) && unicode.IsLower(first) {
		return false
	}
	capped := 0
	for _, w := range words[1:] {
		r := []rune(w)
		if len(r) > 1 && unicode.IsUpper(r[0]) && !isAllCaps(w) && !strings.ContainsAny(w, "0123456789") {
			capped++
		}
	}
	return capped < 2 || capped*2 <= len(words)-1
}

func isAllCaps(w string) bool {
	return w == strings.ToUpper(w)
}

type visibleLine struct {
	no   int
	text string
}

// LintDocument checks the prose of an issue body or proposal (kind issue) or a
// pull request body (kind pr). Code, comments, collapsed sections, task
// checkboxes and links are not prose and are not counted.
func LintDocument(kind, text string, t TextConfig) []LintFinding {
	var out []LintFinding
	add := func(line int, rule, msg, sug string) {
		out = append(out, LintFinding{Line: line, Rule: rule, Message: msg, Suggestion: sug})
	}
	var fillerRe *regexp.Regexp
	if len(t.Filler) > 0 {
		parts := make([]string, len(t.Filler))
		for i, f := range t.Filler {
			parts[i] = regexp.QuoteMeta(strings.ToLower(f))
		}
		fillerRe = regexp.MustCompile(`\b(` + strings.Join(parts, "|") + `)\b`)
	}
	allowed := map[string]bool{}
	for _, h := range t.Headers {
		allowed[strings.ToLower(h)] = true
	}
	if kind == "pr" {
		for _, h := range prBodyHeaders {
			allowed[strings.ToLower(h)] = true
		}
	}

	var para []visibleLine
	headers, words := 0, 0
	flush := func() {
		if len(para) == 0 {
			return
		}
		if t.ParagraphLines > 0 && len(para) > t.ParagraphLines {
			add(para[0].no, "paragraph-length", fmt.Sprintf("paragraph is %d lines, the limit is %d", len(para), t.ParagraphLines), "use bullets")
		}
		var joined []string
		for _, l := range para {
			joined = append(joined, l.text)
		}
		prose(para[0].no, strings.Join(joined, " "), t, fillerRe, &words, add)
		para = nil
	}

	var item []visibleLine
	flushItem := func() {
		if len(item) == 0 {
			return
		}
		if t.ParagraphLines > 0 && len(item) > t.ParagraphLines {
			add(item[0].no, "paragraph-length", fmt.Sprintf("list item is %d lines, the limit is %d", len(item), t.ParagraphLines), "shorten it")
		}
		var joined []string
		for _, l := range item {
			joined = append(joined, strings.TrimSpace(l.text))
		}
		prose(item[0].no, strings.Join(joined, " "), t, fillerRe, &words, add)
		item = nil
	}
	inTask := false
	for _, vl := range visibleLines(text) {
		no, line := vl.no, strings.TrimRight(vl.text, " \t")
		trim := strings.TrimSpace(line)
		if trim == "" {
			flush()
			flushItem()
			inTask = false
			continue
		}
		if m := headingLine.FindStringSubmatch(trim); m != nil {
			flush()
			flushItem()
			inTask = false
			name := m[2]
			if sliceTitle.MatchString(name) {
				continue
			}
			headers++
			if t.MaxHeaders > 0 && headers == t.MaxHeaders+1 {
				add(no, "header-count", fmt.Sprintf("more than %d headers", t.MaxHeaders), "merge sections")
			}
			if len(t.Headers) > 0 && !allowed[strings.ToLower(name)] {
				add(no, "header-set", fmt.Sprintf("header %q is not in the allowed set", name), strings.Join(t.Headers, ", "))
			}
			if n := len(strings.Fields(name)); t.HeaderWords > 0 && n > t.HeaderWords {
				add(no, "header-length", fmt.Sprintf("header %q has %d words, the limit is %d", name, n, t.HeaderWords), "")
			}
			continue
		}
		switch {
		case strings.HasPrefix(trim, "# ") || strings.HasPrefix(trim, "|") || strings.HasPrefix(trim, ">"):
			flush()
			flushItem()
			inTask = false
		case checkboxLine.MatchString(line):
			flush()
			flushItem()
			inTask = true
		case listItemLine.MatchString(line):
			flush()
			flushItem()
			inTask = false
			item = []visibleLine{{no, listItemLine.ReplaceAllString(line, "")}}
		case inTask:
		case len(item) > 0:
			item = append(item, visibleLine{no, trim})
		default:
			para = append(para, visibleLine{no, trim})
		}
	}
	flushItem()
	flush()

	budget, name := t.IssueWords, "issue"
	if kind == "pr" {
		budget, name = t.PRWords, "pull request"
	}
	if budget > 0 && words > budget {
		add(1, "length-budget", fmt.Sprintf("the visible part of the %s is %d words, the budget is %d", name, words, budget), "move detail into a collapsed section or a link")
	}
	return out
}

func prose(no int, s string, t TextConfig, filler *regexp.Regexp, words *int, add func(int, string, string, string)) {
	s = bareURL.ReplaceAllString(markdownLink.ReplaceAllString(s, " "), " ")
	*words += countWords(s)
	if t.SentenceWords > 0 {
		for _, sentence := range splitSentences(s) {
			if n := countWords(sentence); n > t.SentenceWords {
				add(no, "sentence-length", fmt.Sprintf("a sentence has %d words, the limit is %d", n, t.SentenceWords), "split it or use bullets")
				break
			}
		}
	}
	if filler != nil {
		seen := map[string]bool{}
		for _, m := range filler.FindAllString(strings.ToLower(s), -1) {
			if !seen[m] {
				seen[m] = true
				add(no, "filler", fmt.Sprintf("avoid %q", m), "remove it")
			}
		}
	}
}

func countWords(s string) int {
	n := 0
	for _, w := range strings.Fields(s) {
		if strings.IndexFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			n++
		}
	}
	return n
}

// prBodyHeaders are the sections of the generated pull request body.
var prBodyHeaders = []string{"Summary", "Changes", "Review focus", "Size", "Verification", "Closes"}

var abbreviations = []string{"e.g", "i.e", "etc", "vs", "cf"}

func splitSentences(s string) []string {
	var out []string
	rs := []rune(s)
	start := 0
	for i, r := range rs {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		end := i + 1
		for end < len(rs) && (rs[end] == '.' || rs[end] == '!' || rs[end] == '?') {
			end++
		}
		if end < len(rs) && rs[end] != ' ' {
			continue
		}
		next := end
		for next < len(rs) && rs[next] == ' ' {
			next++
		}
		if next < len(rs) && !unicode.IsUpper(rs[next]) && !unicode.IsDigit(rs[next]) {
			continue
		}
		before := strings.ToLower(string(rs[start:i]))
		skip := false
		for _, a := range abbreviations {
			if strings.HasSuffix(before, a) {
				skip = true
			}
		}
		if skip {
			continue
		}
		out = append(out, string(rs[start:end]))
		start = end
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// visibleLines returns the lines of text that are prose: code fences,
// collapsed sections and comments are dropped, one-line forms included, and
// each dropped region leaves an empty line so paragraphs do not join across it.
func visibleLines(text string) []visibleLine {
	var out []visibleLine
	skip := func(no int) {
		if n := len(out); n == 0 || out[n-1].text != "" {
			out = append(out, visibleLine{no, ""})
		}
	}
	inFence, inDetails, inComment := false, false, false
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		no := i + 1
		line := raw
		trim := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
				inFence = false
			}
			continue
		}
		if inComment {
			end := strings.Index(line, "-->")
			if end < 0 {
				continue
			}
			inComment = false
			line = line[end+3:]
			trim = strings.TrimSpace(line)
		}
		line = inlineComment.ReplaceAllString(line, "")
		if i := strings.Index(line, "<!--"); i >= 0 {
			line, inComment = line[:i], true
		}
		trim = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~"):
			marker := trim[:3]
			if !strings.Contains(trim[3:], marker) {
				inFence = true
			}
			skip(no)
			continue
		case strings.HasPrefix(trim, "</details"):
			inDetails = false
			skip(no)
			continue
		case inDetails:
			continue
		case strings.HasPrefix(trim, "<details"):
			if !strings.Contains(trim, "</details>") {
				inDetails = true
			}
			skip(no)
			continue
		}
		out = append(out, visibleLine{no, line})
	}
	return out
}
