package specsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PRBodyInput is everything a pull request body is rendered from.
type PRBodyInput struct {
	Change    Change
	Template  string // the repository's template text, "" for the default
	Reference string // the closing reference line, for example "Part of #12"
	Size      string // a one-line size summary, "" when unknown
	Text      TextConfig
}

var prTemplateCandidates = []string{
	".github/pull_request_template.md",
	".github/PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md",
	"pull_request_template.md",
	"PULL_REQUEST_TEMPLATE.md",
}

// FindPRTemplate returns the repository's pull request template: the
// configured path when set, else the usual locations. The path is relative to
// repoRoot; both results are empty when there is none. A configured path must
// stay inside the repository, symlinks included.
func FindPRTemplate(repoRoot string, cfg TextConfig) (path, text string, err error) {
	if cfg.Template != "" {
		full, err := insideRepo(repoRoot, cfg.Template)
		if err != nil {
			return "", "", fmt.Errorf("text.template %q must be a path inside the repository: %w", cfg.Template, err)
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return "", "", fmt.Errorf("text.template %q: %w", cfg.Template, err)
		}
		return cfg.Template, string(b), nil
	}
	for _, c := range prTemplateCandidates {
		full, err := insideRepo(repoRoot, c)
		if err != nil {
			continue
		}
		if b, err := os.ReadFile(full); err == nil {
			return c, string(b), nil
		}
	}
	return "", "", nil
}

func insideRepo(repoRoot, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("it leaves the repository")
	}
	root, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, clean)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return full, nil
		}
		return "", err
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		return "", fmt.Errorf("it resolves outside the repository")
	}
	return resolved, nil
}

const (
	summaryWords = 30
	changesWords = 80
	focusWords   = 40
)

type prSection struct {
	kind string // summary, changes, focus, size, verification, closes
	body string
}

// RenderPRBody renders a short pull request body from the change. Sections
// with nothing to say are left out, and each stays within its word budget.
func RenderPRBody(in PRBodyInput) string {
	generated := map[string]string{
		"summary":      summaryOf(in.Change.Body),
		"changes":      bulletsOf(in.Change.Body, changesWords, "what", "what changes", "changes"),
		"focus":        bulletsOf(in.Change.Body, focusWords, "review focus"),
		"size":         in.Size,
		"verification": verificationOf(in.Change.TasksMarkdown),
		"closes":       in.Reference,
	}
	if strings.TrimSpace(in.Template) != "" {
		return renderFromTemplate(in.Template, generated)
	}
	var parts []string
	for _, s := range []struct{ kind, heading string }{
		{"summary", "Summary"}, {"changes", "Changes"}, {"focus", "Review focus"},
		{"size", "Size"}, {"verification", "Verification"}, {"closes", "Closes"},
	} {
		if g := generated[s.kind]; g != "" {
			parts = append(parts, "## "+s.heading+"\n\n"+g+"\n")
		}
	}
	return strings.Join(parts, "\n")
}

var templateKinds = map[string]string{
	"summary": "summary", "description": "summary", "why": "summary", "overview": "summary",
	"changes": "changes", "what": "changes", "what changed": "changes",
	"review focus": "focus",
	"size":         "size",
	"verification": "verification", "testing": "verification", "tests": "verification", "test plan": "verification",
	"closes": "closes", "issue": "closes", "issues": "closes", "related issue": "closes", "linked issue": "closes",
}

func renderFromTemplate(tpl string, generated map[string]string) string {
	lines := strings.Split(strings.ReplaceAll(tpl, "\r\n", "\n"), "\n")
	heading := templateHeadings(lines)
	var out []string
	closesPlaced := false
	filled := map[string]bool{}
	i := 0
	for i < len(lines) && !heading[i] {
		out = append(out, lines[i])
		i++
	}
	for i < len(lines) {
		j := i + 1
		for j < len(lines) && !heading[j] {
			j++
		}
		kind := templateKinds[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(lines[i], "## ")))]
		if g := generated[kind]; kind != "" && g != "" && !filled[kind] {
			out = append(out, lines[i], "", g, "")
			filled[kind] = true
			closesPlaced = closesPlaced || kind == "closes"
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	body := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	if !closesPlaced && generated["closes"] != "" {
		body += "\n" + generated["closes"] + "\n"
	}
	return body
}

// templateHeadings marks the lines that start a section: `## ` lines outside
// code fences and comments.
func templateHeadings(lines []string) map[int]bool {
	out := map[int]bool{}
	inFence, inComment := false, false
	for i, l := range lines {
		trim := strings.TrimSpace(l)
		switch {
		case inFence:
			if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
				inFence = false
			}
			continue
		case inComment:
			if strings.Contains(l, "-->") {
				inComment = false
			}
			continue
		case strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~"):
			inFence = !strings.Contains(trim[3:], trim[:3])
			continue
		}
		if rest, ok := strings.CutPrefix(l, "<!--"); ok || strings.Contains(l, "<!--") {
			if !ok {
				rest = l[strings.Index(l, "<!--")+4:]
			}
			inComment = !strings.Contains(rest, "-->")
			if strings.HasPrefix(l, "<!--") {
				continue
			}
		}
		if strings.HasPrefix(l, "## ") {
			out[i] = true
		}
	}
	return out
}

// markdownSection returns the lines under the first-level match of any name,
// sub-headings included, up to the next heading of the same or a higher level.
func markdownSection(body string, names ...string) []string {
	var out []string
	level := 0
	for _, l := range visibleLines(body) {
		if m := headingLine.FindStringSubmatch(strings.TrimSpace(l.text)); m != nil {
			depth := len(m[1])
			switch {
			case level > 0 && depth > level:
			case matchesAny(m[2], names):
				level = depth
			default:
				level = 0
			}
			continue
		}
		if level > 0 {
			out = append(out, l.text)
		}
	}
	return out
}

func matchesAny(name string, names []string) bool {
	for _, n := range names {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

func summaryOf(body string) string {
	var para []string
	for _, l := range markdownSection(body, "why") {
		t := strings.TrimSpace(l)
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if listItemLine.MatchString(l) {
			if len(para) > 0 {
				break
			}
			t = listItemLine.ReplaceAllString(l, "")
		}
		para = append(para, t)
	}
	if len(para) == 0 {
		return ""
	}
	sentences := splitSentences(strings.Join(para, " "))
	return truncateWords(strings.TrimSpace(sentences[0]), summaryWords)
}

func bulletsOf(body string, budget int, sections ...string) string {
	var out []string
	used := 0
	for _, l := range markdownSection(body, sections...) {
		if !listItemLine.MatchString(l) || checkboxLine.MatchString(l) {
			continue
		}
		text := strings.TrimSpace(listItemLine.ReplaceAllString(l, ""))
		n := len(strings.Fields(text))
		if len(out) == 0 && n > budget {
			text, n = truncateWords(text, budget), budget
		}
		if used+n > budget {
			break
		}
		out = append(out, "- "+text)
		used += n
	}
	return strings.Join(out, "\n")
}

func verificationOf(tasks string) string {
	total, done := CountCheckboxes(tasks)
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d tasks done.", done, total)
}

func truncateWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) <= n {
		return s
	}
	return strings.TrimRight(strings.Join(w[:n], " "), ".,;:") + "…"
}
