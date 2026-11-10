package specsync

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Categories a changed file can fall into, in the order totals are listed.
var ReviewCategories = []string{"source", "test", "docs", "config", "generated", "other"}

var (
	generatedNames = map[string]bool{
		"go.sum": true, "package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
		"cargo.lock": true, "poetry.lock": true, "gemfile.lock": true, "composer.lock": true,
		"pipfile.lock": true, "bun.lockb": true,
	}
	generatedSuffixes = []string{".min.js", ".min.css", ".pb.go", ".pb.cc", ".pb.h", "_generated.go", ".generated.ts", ".snap"}
	generatedDirs     = map[string]bool{"dist": true, "vendor": true, "node_modules": true, "__generated__": true}
	testDirs          = map[string]bool{"test": true, "tests": true, "__tests__": true, "spec": true, "testdata": true, "e2e": true, "fixtures": true}
	testSuffixes      = []string{"_test.go", ".test.js", ".test.ts", ".test.tsx", ".spec.js", ".spec.ts", ".spec.tsx", "_test.py", "_spec.rb", "test.java", "tests.cs"}
	docExts           = map[string]bool{".md": true, ".mdx": true, ".rst": true, ".txt": true, ".adoc": true}
	configExts        = map[string]bool{".yml": true, ".yaml": true, ".json": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true, ".env": true, ".properties": true}
	configNames       = map[string]bool{"dockerfile": true, "makefile": true, ".gitignore": true, ".gitattributes": true, ".editorconfig": true, "go.mod": true, "go.work": true}
	sourceExts        = map[string]bool{
		".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true,
		".rs": true, ".java": true, ".kt": true, ".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true,
		".cs": true, ".rb": true, ".php": true, ".swift": true, ".sh": true, ".sql": true, ".css": true,
		".scss": true, ".html": true, ".vue": true, ".svelte": true, ".lua": true, ".scala": true, ".ex": true,
	}
)

// CategoryOf classifies a slash path by its name and directories alone; file
// contents are never read. The first matching rule wins: generated, test, docs,
// config, source, other.
func CategoryOf(file string) string {
	lower := strings.ToLower(file)
	base := path.Base(lower)
	dirs := strings.Split(path.Dir(lower), "/")
	if generatedNames[base] {
		return "generated"
	}
	for _, s := range generatedSuffixes {
		if strings.HasSuffix(base, s) {
			return "generated"
		}
	}
	for _, d := range dirs {
		if generatedDirs[d] {
			return "generated"
		}
	}
	for _, d := range dirs {
		if testDirs[d] {
			return "test"
		}
	}
	for _, s := range testSuffixes {
		if strings.HasSuffix(base, s) {
			return "test"
		}
	}
	if strings.HasPrefix(base, "test_") {
		return "test"
	}
	ext := path.Ext(base)
	if docExts[ext] || dirs[0] == "docs" || strings.HasPrefix(base, "readme") || strings.HasPrefix(base, "license") || strings.HasPrefix(base, "changelog") {
		return "docs"
	}
	if configExts[ext] || configNames[base] || dirs[0] == ".github" {
		return "config"
	}
	if sourceExts[ext] {
		return "source"
	}
	return "other"
}

// ReviewFocus is one declared judgement area: a path glob and why it matters.
type ReviewFocus struct {
	Glob   string `json:"glob"`
	Reason string `json:"reason,omitempty"`
}

var focusHeading = regexp.MustCompile(`(?mi)^#{2,4}\s+Review focus\s*$`)
var nextHeading = regexp.MustCompile(`(?m)^#{1,4}\s`)
var focusItem = regexp.MustCompile("^\\s*[-*]\\s+(?:`([^`]+)`|(\\S+?))\\s*(?:[:—–-]+\\s*(.*))?$")

// ParseReviewFocus reads the `## Review focus` section of a proposal: one
// bullet per glob, optionally in backticks, followed by a reason after a colon
// or dash. Without the section it returns nothing.
func ParseReviewFocus(proposal string) []ReviewFocus {
	loc := focusHeading.FindStringIndex(proposal)
	if loc == nil {
		return nil
	}
	rest := proposal[loc[1]:]
	if n := nextHeading.FindStringIndex(rest); n != nil {
		rest = rest[:n[0]]
	}
	var out []ReviewFocus
	for _, line := range strings.Split(rest, "\n") {
		m := focusItem.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		glob := m[1]
		if glob == "" {
			glob = strings.TrimRight(m[2], ":")
		}
		if glob == "" {
			continue
		}
		out = append(out, ReviewFocus{Glob: glob, Reason: strings.TrimSpace(m[3])})
	}
	return out
}

var taskLine = regexp.MustCompile(`(?m)^\s*[-*]\s+\[[ xX]\]\s+(\d+(?:\.\d+)*)\s+(.*?)\s*$`)

// TaskRef is a task id from a change's tasks.md with its text.
type TaskRef struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ParseTaskIDs lists the numbered checklist items of a tasks.md.
func ParseTaskIDs(tasks string) []TaskRef {
	var out []TaskRef
	for _, m := range taskLine.FindAllStringSubmatch(tasks, -1) {
		out = append(out, TaskRef{ID: m[1], Text: m[2]})
	}
	return out
}

var taskMention = regexp.MustCompile(`(?i)\btasks?\s+(\d+(?:\.\d+)*)`)

// LinkTask returns the id of the first task from tasks that the message names
// as "task <id>". It never matches by similarity.
func LinkTask(message string, tasks []TaskRef) string {
	known := map[string]bool{}
	for _, t := range tasks {
		known[t.ID] = true
	}
	for _, m := range taskMention.FindAllStringSubmatch(message, -1) {
		if known[m[1]] {
			return m[1]
		}
	}
	return ""
}

// MapFile is one file touched by a commit, or by the whole range.
type MapFile struct {
	Path      string `json:"path"`
	Category  string `json:"category"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Judgement bool   `json:"needsJudgement,omitempty"`
}

// MapCommit is one commit in order, with the task it names, if any.
type MapCommit struct {
	SHA      string    `json:"sha"`
	Subject  string    `json:"subject"`
	Task     string    `json:"task,omitempty"`
	TaskText string    `json:"taskText,omitempty"`
	Files    []MapFile `json:"files"`
}

// JudgementFile is a changed file that matches a declared focus glob.
type JudgementFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
	Glob   string `json:"glob"`
}

// CategoryTotal sums the range's files in one category.
type CategoryTotal struct {
	Category  string `json:"category"`
	Files     int    `json:"files"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// ReviewMap is the checklist for one branch or range.
type ReviewMap struct {
	Base      string          `json:"base"`
	Head      string          `json:"head"`
	Change    string          `json:"change,omitempty"`
	Judgement []JudgementFile `json:"needsJudgement"`
	Commits   []MapCommit     `json:"commits"`
	Totals    []CategoryTotal `json:"totals"`
}

type numstatFile struct {
	path string
	add  int
	del  int
}

func parseNumstatZ(out string) []numstatFile {
	var files []numstatFile
	for _, rec := range strings.Split(out, "\x00") {
		rec = strings.TrimLeft(rec, "\n")
		f := strings.SplitN(rec, "\t", 3)
		if len(f) < 3 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		d, _ := strconv.Atoi(f[1])
		files = append(files, numstatFile{path: f[2], add: a, del: d})
	}
	return files
}

// BuildReviewMap lists the commits of base..head in order with their files,
// marks files matching focus as needing judgement, links commits to tasks only
// by explicit id, and totals the whole range by category. It only reads git.
func BuildReviewMap(ctx context.Context, dir, base, head string, focus []ReviewFocus, tasks []TaskRef) (*ReviewMap, error) {
	type glob struct {
		re *regexp.Regexp
		f  ReviewFocus
	}
	var globs []glob
	for _, f := range focus {
		if re := globRegexp(f.Glob); re != nil {
			globs = append(globs, glob{re, f})
		}
	}
	match := func(p string) (ReviewFocus, bool) {
		for _, g := range globs {
			if g.re.MatchString(p) {
				return g.f, true
			}
		}
		return ReviewFocus{}, false
	}

	m := &ReviewMap{Base: base, Head: head, Judgement: []JudgementFile{}, Commits: []MapCommit{}}
	shas, err := gitIn(ctx, dir, "rev-list", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	for _, sha := range strings.Fields(shas) {
		subject, err := gitIn(ctx, dir, "log", "-1", "--format=%s", sha)
		if err != nil {
			return nil, err
		}
		body, err := gitIn(ctx, dir, "log", "-1", "--format=%B", sha)
		if err != nil {
			return nil, err
		}
		stat, err := gitIn(ctx, dir, "show", "--format=", "--numstat", "-z", "--no-renames", sha)
		if err != nil {
			return nil, err
		}
		c := MapCommit{SHA: sha, Subject: subject, Files: []MapFile{}}
		if id := LinkTask(body, tasks); id != "" {
			c.Task = id
			for _, t := range tasks {
				if t.ID == id {
					c.TaskText = t.Text
				}
			}
		}
		for _, nf := range parseNumstatZ(stat) {
			_, hit := match(nf.path)
			c.Files = append(c.Files, MapFile{Path: nf.path, Category: CategoryOf(nf.path), Additions: nf.add, Deletions: nf.del, Judgement: hit})
		}
		sort.SliceStable(c.Files, func(i, j int) bool { return c.Files[i].Path < c.Files[j].Path })
		m.Commits = append(m.Commits, c)
	}

	diff, err := gitIn(ctx, dir, "diff", "--numstat", "-z", "--no-renames", base+"..."+head)
	if err != nil {
		return nil, err
	}
	totals := map[string]*CategoryTotal{}
	for _, c := range ReviewCategories {
		totals[c] = &CategoryTotal{Category: c}
	}
	files := parseNumstatZ(diff)
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	for _, nf := range files {
		t := totals[CategoryOf(nf.path)]
		t.Files++
		t.Additions += nf.add
		t.Deletions += nf.del
		if f, ok := match(nf.path); ok {
			m.Judgement = append(m.Judgement, JudgementFile{Path: nf.path, Reason: f.Reason, Glob: f.Glob})
		}
	}
	for _, c := range ReviewCategories {
		m.Totals = append(m.Totals, *totals[c])
	}
	return m, nil
}

// Markdown renders the map as a checklist: judgement areas first, then the
// commits in order, then totals by category.
func (m *ReviewMap) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review map: %s against %s\n", m.Head, m.Base)
	if m.Change != "" {
		fmt.Fprintf(&b, "\nChange: %s\n", m.Change)
	}
	b.WriteString("\n## Needs human judgement\n\n")
	if len(m.Judgement) == 0 {
		b.WriteString("Nothing declared. Add a `## Review focus` section to the change's proposal.md to mark paths.\n")
	}
	for _, j := range m.Judgement {
		fmt.Fprintf(&b, "- [ ] `%s`", j.Path)
		if j.Reason != "" {
			fmt.Fprintf(&b, " — %s", j.Reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Commits in order\n")
	for i, c := range m.Commits {
		short := c.SHA
		if len(short) > 7 {
			short = short[:7]
		}
		fmt.Fprintf(&b, "\n### %d. %s %s\n\n", i+1, short, c.Subject)
		if c.Task != "" {
			fmt.Fprintf(&b, "Task %s", c.Task)
			if c.TaskText != "" {
				fmt.Fprintf(&b, ": %s", c.TaskText)
			}
			b.WriteString("\n\n")
		} else {
			b.WriteString("Task: unlinked\n\n")
		}
		for _, f := range c.Files {
			fmt.Fprintf(&b, "- [ ] `%s` (%s, +%d -%d)", f.Path, f.Category, f.Additions, f.Deletions)
			if f.Judgement {
				b.WriteString(" needs judgement")
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Totals by category\n\n| category | files | added | removed |\n|---|---|---|---|\n")
	for _, t := range m.Totals {
		fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", t.Category, t.Files, t.Additions, t.Deletions)
	}
	return b.String()
}

// TreesEqual compares the trees of two revisions, not their history. It
// returns whether they are identical and, when not, the differing paths.
func TreesEqual(ctx context.Context, dir, oldRev, newRev string) (bool, []string, error) {
	a, err := gitIn(ctx, dir, "rev-parse", oldRev+"^{tree}")
	if err != nil {
		return false, nil, err
	}
	b, err := gitIn(ctx, dir, "rev-parse", newRev+"^{tree}")
	if err != nil {
		return false, nil, err
	}
	if a == b {
		return true, nil, nil
	}
	out, err := gitIn(ctx, dir, "diff", "--name-only", "-z", "--no-renames", oldRev, newRev)
	if err != nil {
		return false, nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return false, paths, nil
}
