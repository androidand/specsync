package specsync

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Budget is the advisory size limit for one branch. A zero field means no
// limit on that measure.
type Budget struct {
	Files   int
	Lines   int
	Commits int
}

// BudgetEnv names the environment variable that sets the budget.
const BudgetEnv = "SPECSYNC_BUDGET"

// DefaultBudget is conservative policy, not a claim about the right size.
var DefaultBudget = Budget{Files: 30, Lines: 1000, Commits: 30}

// ParseBudget reads "files=15,lines=400,commits=30"; "<=" and ":" work as "=".
func ParseBudget(s string) (Budget, error) {
	var b Budget
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.IndexAny(part, "=<:")
		if i < 0 {
			return Budget{}, fmt.Errorf("budget %q: want key=number, for example files=15", part)
		}
		key := strings.TrimSpace(part[:i])
		val := strings.TrimSpace(strings.TrimLeft(part[i:], "=<:"))
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return Budget{}, fmt.Errorf("budget %q: %q is not a number", part, val)
		}
		switch key {
		case "files":
			b.Files = n
		case "lines":
			b.Lines = n
		case "commits":
			b.Commits = n
		default:
			return Budget{}, fmt.Errorf("budget %q: unknown measure %q (use files, lines or commits)", part, key)
		}
	}
	return b, nil
}

// ResolveBudget layers the sources per measure: the flag, then the slice's
// budget line, then SPECSYNC_BUDGET, then DefaultBudget.
func ResolveBudget(flagValue, sliceLine, env string) (Budget, error) {
	out := DefaultBudget
	for _, src := range []string{env, sliceLine, flagValue} {
		b, err := ParseBudget(src)
		if err != nil {
			return Budget{}, err
		}
		if b.Files > 0 {
			out.Files = b.Files
		}
		if b.Lines > 0 {
			out.Lines = b.Lines
		}
		if b.Commits > 0 {
			out.Commits = b.Commits
		}
	}
	return out, nil
}

// Exceeded names each limit the size is over, for example "lines 1200 > 1000".
func (b Budget) Exceeded(s *BranchSize) []string {
	if s == nil {
		return nil
	}
	var out []string
	if b.Files > 0 && s.Files > b.Files {
		out = append(out, fmt.Sprintf("files %d > %d", s.Files, b.Files))
	}
	if lines := s.Additions + s.Deletions; b.Lines > 0 && lines > b.Lines {
		out = append(out, fmt.Sprintf("lines %d > %d", lines, b.Lines))
	}
	if b.Commits > 0 && s.Commits > b.Commits {
		out = append(out, fmt.Sprintf("commits %d > %d", s.Commits, b.Commits))
	}
	return out
}

var commitScope = regexp.MustCompile(`^[a-z]+\(([^()\s:]+)\)!?:`)

// CommitScopes returns the distinct conventional-commit scopes in subjects,
// sorted. Subjects without a well-formed scope contribute nothing.
func CommitScopes(subjects []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range subjects {
		if m := commitScope.FindStringSubmatch(s); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

func excluded(file string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, file); ok {
			return true
		}
		if ok, _ := path.Match(g, path.Base(file)); ok {
			return true
		}
	}
	return false
}

// AnalyzeBranch measures head against its merge base with base using git
// alone, and lists the distinct commit scopes on the branch. Paths matching an
// exclude glob (on the full path or the file name) are left out of the counts.
func AnalyzeBranch(ctx context.Context, dir, base, head string, exclude []string) (*BranchSize, []string, error) {
	out, err := gitIn(ctx, dir, "rev-list", "--count", base+".."+head)
	if err != nil {
		return nil, nil, err
	}
	size := &BranchSize{}
	size.Commits, _ = strconv.Atoi(strings.TrimSpace(out))
	diff, err := gitIn(ctx, dir, "diff", "--numstat", base+"..."+head)
	if err != nil {
		return nil, nil, err
	}
	for _, line := range strings.Split(diff, "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) < 3 || excluded(f[2], exclude) {
			continue
		}
		size.Files++
		a, _ := strconv.Atoi(f[0])
		d, _ := strconv.Atoi(f[1])
		size.Additions += a
		size.Deletions += d
	}
	subjects, err := gitIn(ctx, dir, "log", "--format=%s", base+".."+head)
	if err != nil {
		return size, nil, nil
	}
	return size, CommitScopes(strings.Split(subjects, "\n")), nil
}

var sliceHeading = regexp.MustCompile(`(?m)^#{2,4}\s+Slice\b`)
var budgetLine = regexp.MustCompile(`(?m)^\s*(?:[-*]\s+)?budget:\s*(.+?)\s*$`)

func changeText(changeDir string) string {
	var b strings.Builder
	for _, f := range []string{"tasks.md", "proposal.md"} {
		if data, err := os.ReadFile(filepath.Join(changeDir, f)); err == nil {
			b.Write(data)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// HasSlices reports whether the change declares slices as `## Slice` headings
// in its tasks.md or proposal.md.
func HasSlices(changeDir string) bool {
	return sliceHeading.MatchString(changeText(changeDir))
}

// SliceBudgetLine returns the value of the first `budget:` line in the change's
// tasks.md or proposal.md, or "".
func SliceBudgetLine(changeDir string) string {
	if m := budgetLine.FindStringSubmatch(changeText(changeDir)); m != nil {
		return m[1]
	}
	return ""
}

// DefaultBase resolves the branch work is measured against: origin/HEAD.
func DefaultBase(ctx context.Context, dir string) (string, error) {
	out, err := gitIn(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("origin/HEAD is not set")
	}
	return strings.TrimSpace(out), nil
}
