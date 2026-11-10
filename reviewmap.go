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
	testSuffixes      = []string{"_test.go", ".test.js", ".test.ts", ".test.tsx", ".spec.js", ".spec.ts", ".spec.tsx", "_test.py", "_spec.rb"}
	testClassSuffixes = []string{"Test.java", "Tests.java", "Test.kt", "Tests.kt", "Test.cs", "Tests.cs"}
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
	origBase := path.Base(file)
	for _, s := range testClassSuffixes {
		if strings.HasSuffix(origBase, s) {
			return "test"
		}
	}
	ext := path.Ext(base)
	if docExts[ext] || (dirs[0] == "docs" && !sourceExts[ext]) || strings.HasPrefix(base, "readme") || strings.HasPrefix(base, "license") || strings.HasPrefix(base, "changelog") {
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
var bulletLine = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)

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
		if glob, reason, ok := parseFocusLine(line); ok {
			out = append(out, ReviewFocus{Glob: glob, Reason: reason})
		}
	}
	return out
}

// parseFocusLine reads "- `glob` — reason" or "- glob: reason". A hyphen inside
// an unquoted glob stays in the glob: a dash only separates when it is
// surrounded by spaces.
func parseFocusLine(line string) (glob, reason string, ok bool) {
	m := bulletLine.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	rest := strings.TrimSpace(m[1])
	if strings.HasPrefix(rest, "`") {
		end := strings.Index(rest[1:], "`")
		if end < 0 {
			return "", "", false
		}
		glob, rest = rest[1:1+end], rest[end+2:]
	} else if i := strings.IndexAny(rest, " \t"); i < 0 {
		glob, rest = rest, ""
	} else {
		glob, rest = rest[:i], rest[i:]
	}
	glob = strings.TrimSuffix(glob, ":")
	if glob == "" {
		return "", "", false
	}
	rest = strings.TrimSpace(rest)
	for _, sep := range []string{"\u2014", "\u2013", "-", ":"} {
		if strings.HasPrefix(rest, sep) {
			rest = strings.TrimSpace(strings.TrimPrefix(rest, sep))
			break
		}
	}
	return glob, rest, true
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
	Base          string          `json:"base"`
	Head          string          `json:"head"`
	Change        string          `json:"change,omitempty"`
	Judgement     []JudgementFile `json:"needsJudgement"`
	Commits       []MapCommit     `json:"commits"`
	MergesSkipped int             `json:"mergesSkipped,omitempty"`
	Note          string          `json:"note,omitempty"`
	Totals        []CategoryTotal `json:"totals"`
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
	merges, err := gitIn(ctx, dir, "rev-list", "--merges", "--count", base+".."+head)
	if err != nil {
		return nil, err
	}
	m.MergesSkipped, _ = strconv.Atoi(strings.TrimSpace(merges))
	log, err := gitIn(ctx, dir, "log", "--reverse", "--topo-order", "--no-merges", "--no-renames", "--numstat", "-z",
		"--format=%x01%H%x1f%s%x1f%B%x1e", base+".."+head)
	if err != nil {
		return nil, err
	}
	for _, chunk := range strings.Split(log, "\x01") {
		end := strings.Index(chunk, "\x1e")
		if end < 0 {
			continue
		}
		head3 := strings.SplitN(chunk[:end], "\x1f", 3)
		if len(head3) < 3 {
			continue
		}
		c := MapCommit{SHA: head3[0], Subject: head3[1], Files: []MapFile{}}
		if id := LinkTask(head3[2], tasks); id != "" {
			c.Task = id
			for _, t := range tasks {
				if t.ID == id {
					c.TaskText = t.Text
				}
			}
		}
		for _, nf := range parseNumstatZ(chunk[end+1:]) {
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
		if m.Note != "" {
			b.WriteString(m.Note + "\n")
		} else {
			b.WriteString("Nothing declared.\n")
		}
	}
	for _, j := range m.Judgement {
		fmt.Fprintf(&b, "- [ ] `%s`", j.Path)
		if j.Reason != "" {
			fmt.Fprintf(&b, " — %s", j.Reason)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Commits in order\n")
	if m.MergesSkipped > 0 {
		plural := "s"
		if m.MergesSkipped == 1 {
			plural = ""
		}
		fmt.Fprintf(&b, "\n%d merge commit%s skipped; totals cover the whole range.\n", m.MergesSkipped, plural)
	}
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
// returns whether they are identical and, when not, the differing paths. A
// revision that is option-like or does not resolve is an error.
func TreesEqual(ctx context.Context, dir, oldRev, newRev string) (bool, []string, error) {
	trees := make([]string, 2)
	for i, rev := range []string{oldRev, newRev} {
		if rev == "" || strings.HasPrefix(rev, "-") {
			return false, nil, fmt.Errorf("%q is not a revision", rev)
		}
		tree, err := gitIn(ctx, dir, "rev-parse", "--verify", "-q", rev+"^{tree}")
		if err != nil {
			return false, nil, fmt.Errorf("%q is not a revision in this repository", rev)
		}
		trees[i] = tree
	}
	if trees[0] == trees[1] {
		return true, nil, nil
	}
	out, err := gitIn(ctx, dir, "diff", "--name-only", "-z", "--no-renames", oldRev, newRev, "--")
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

// ShowFileAt returns a file's content at a revision, or an error when the
// revision or the file does not exist.
func ShowFileAt(ctx context.Context, dir, rev, file string) (string, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return "", fmt.Errorf("%q is not a revision", rev)
	}
	return gitIn(ctx, dir, "show", rev+":"+file)
}
