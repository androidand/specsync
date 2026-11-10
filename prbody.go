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
// repoRoot; both results are empty when there is none.
func FindPRTemplate(repoRoot string, cfg TextConfig) (path, text string) {
	candidates := prTemplateCandidates
	if cfg.Template != "" {
		candidates = []string{cfg.Template}
	}
	for _, c := range candidates {
		if b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(c))); err == nil {
			return c, string(b)
		}
	}
	return "", ""
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
		"changes":      bulletsOf(in.Change.Body, "what", changesWords),
		"focus":        bulletsOf(in.Change.Body, "review focus", focusWords),
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
	var out []string
	closesPlaced := false
	i := 0
	for i < len(lines) && !strings.HasPrefix(lines[i], "## ") {
		out = append(out, lines[i])
		i++
	}
	for i < len(lines) {
		heading := lines[i]
		j := i + 1
		for j < len(lines) && !strings.HasPrefix(lines[j], "## ") {
			j++
		}
		kind := templateKinds[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(heading, "## ")))]
		if g := generated[kind]; kind != "" && g != "" {
			out = append(out, heading, "", g, "")
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

func markdownSection(body, name string) []string {
	var out []string
	in := false
	for _, l := range strings.Split(body, "\n") {
		if m := headingLine.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			in = strings.EqualFold(m[2], name)
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return out
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
		if bulletLine.MatchString(l) {
			if len(para) > 0 {
				break
			}
			t = bulletLine.ReplaceAllString(l, "")
		}
		para = append(para, t)
	}
	if len(para) == 0 {
		return ""
	}
	sentences := splitSentences(strings.Join(para, " "))
	return truncateWords(strings.TrimSpace(sentences[0]), summaryWords)
}

func bulletsOf(body, section string, budget int) string {
	var out []string
	used := 0
	for _, l := range markdownSection(body, section) {
		if !bulletLine.MatchString(l) || checkboxLine.MatchString(l) {
			continue
		}
		text := strings.TrimSpace(bulletLine.ReplaceAllString(l, ""))
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
