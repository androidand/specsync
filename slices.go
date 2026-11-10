package specsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Slice is one author-declared, independently reviewable part of a change.
type Slice struct {
	N        int    `json:"n"`
	Title    string `json:"title"`
	After    []int  `json:"after,omitempty"`
	Budget   string `json:"budget,omitempty"`
	Contract string `json:"contract,omitempty"`
	Tasks    int    `json:"tasks"`
	Done     int    `json:"done"`
}

// SlicePlan is the slices declared in a change's tasks.md. OrphanTasks counts
// task checkboxes outside every slice; it matters only when slices exist.
type SlicePlan struct {
	Slices      []Slice
	OrphanTasks int
	// Problems are parse-time findings, such as a malformed `after:` line.
	Problems []string
}

var (
	sliceHead    = regexp.MustCompile(`^(#{2,4})\s+Slice\s+(\d+)\s*:\s*(.*?)\s*$`)
	anyHeading   = regexp.MustCompile(`^(#{1,6})\s`)
	sliceAfterRe = regexp.MustCompile(`^\s*(?:[-*]\s+)?after:\s*(.+?)\s*$`)
	sliceBudget  = regexp.MustCompile(`^\s*(?:[-*]\s+)?budget:\s*(.+?)\s*$`)
	sliceContrct = regexp.MustCompile(`^\s*(?:[-*]\s+)?contract:\s*(.+?)\s*$`)
)

// ParseSlices reads `## Slice <n>: <title>` blocks from tasks markdown. A slice
// runs to the next slice heading or to a heading of the same or a higher level;
// optional `after:`, `budget:` and `contract:` lines declare its order, size
// budget and contract. Nothing is inferred: no heading, no slice.
func ParseSlices(md string) SlicePlan {
	var plan SlicePlan
	var bodies []*strings.Builder
	var orphan strings.Builder
	cur, curLevel := -1, 0
	afterSeen := map[int]bool{}
	for _, line := range strings.Split(md, "\n") {
		if m := sliceHead.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			plan.Slices = append(plan.Slices, Slice{N: n, Title: m[3]})
			bodies = append(bodies, &strings.Builder{})
			cur, curLevel = len(plan.Slices)-1, len(m[1])
			continue
		}
		if h := anyHeading.FindStringSubmatch(line); h != nil && cur >= 0 && len(h[1]) <= curLevel {
			cur = -1
		}
		if cur < 0 {
			orphan.WriteString(line + "\n")
			continue
		}
		s := &plan.Slices[cur]
		switch {
		case sliceAfterRe.MatchString(line):
			if afterSeen[cur] {
				plan.Problems = append(plan.Problems, fmt.Sprintf("slice %d: after is declared more than once", s.N))
				break
			}
			afterSeen[cur] = true
			nums, ok := parseSliceNumbers(sliceAfterRe.FindStringSubmatch(line)[1])
			if !ok {
				plan.Problems = append(plan.Problems, fmt.Sprintf("slice %d: malformed after line %q: use slice numbers such as `after: 1, 2`", s.N, strings.TrimSpace(line)))
			}
			s.After = nums
		case sliceBudget.MatchString(line) && s.Budget == "":
			s.Budget = sliceBudget.FindStringSubmatch(line)[1]
		case sliceContrct.MatchString(line) && s.Contract == "":
			s.Contract = sliceContrct.FindStringSubmatch(line)[1]
		}
		bodies[cur].WriteString(line + "\n")
	}
	for i := range plan.Slices {
		plan.Slices[i].Tasks, plan.Slices[i].Done = CountCheckboxes(bodies[i].String())
	}
	plan.OrphanTasks, _ = CountCheckboxes(orphan.String())
	return plan
}

func parseSliceNumbers(s string) ([]int, bool) {
	var out []int
	ok := true
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		f = strings.TrimPrefix(strings.ToLower(f), "slice")
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			out = append(out, n)
		} else {
			ok = false
		}
	}
	return out, ok && len(out) > 0
}

// Get returns the slice numbered n.
func (p SlicePlan) Get(n int) (Slice, bool) {
	for _, s := range p.Slices {
		if s.N == n {
			return s, true
		}
	}
	return Slice{}, false
}

// Validate reports problems with the plan: numbers that are duplicated or not
// contiguous, `after` targets that are missing or form a cycle, tasks that
// belong to no slice, and slices without a contract. A change without slices
// has none.
func (p SlicePlan) Validate() []string {
	if len(p.Slices) == 0 {
		return nil
	}
	issues := append([]string(nil), p.Problems...)
	seen := map[int]bool{}
	var nums []int
	for _, s := range p.Slices {
		if seen[s.N] {
			issues = append(issues, fmt.Sprintf("slice %d is declared twice", s.N))
			continue
		}
		seen[s.N] = true
		nums = append(nums, s.N)
	}
	sort.Ints(nums)
	if len(nums) > 0 && nums[0] != 0 && nums[0] != 1 {
		issues = append(issues, fmt.Sprintf("slice numbers must start at 0 or 1, not %d", nums[0]))
	}
	for i := 1; i < len(nums); i++ {
		if nums[i] != nums[i-1]+1 {
			issues = append(issues, fmt.Sprintf("slice numbers are not contiguous: %d is followed by %d", nums[i-1], nums[i]))
		}
	}
	for _, s := range p.Slices {
		for _, a := range s.After {
			switch {
			case a == s.N:
				issues = append(issues, fmt.Sprintf("slice %d cannot come after itself", s.N))
			case !seen[a]:
				issues = append(issues, fmt.Sprintf("slice %d: after %d does not exist", s.N, a))
			}
		}
		if strings.TrimSpace(s.Contract) == "" {
			issues = append(issues, fmt.Sprintf("slice %d has no contract", s.N))
		}
	}
	if cyc := p.cycle(); len(cyc) > 0 {
		sort.Ints(cyc)
		names := make([]string, len(cyc))
		for i, n := range cyc {
			names[i] = strconv.Itoa(n)
		}
		list := strings.Join(names, ", ")
		if len(names) == 2 {
			list = names[0] + " and " + names[1]
		}
		issues = append(issues, fmt.Sprintf("slices %s form a cycle", list))
	}
	if p.OrphanTasks > 0 {
		issues = append(issues, fmt.Sprintf("%d task(s) belong to no slice: every task must sit under a slice heading", p.OrphanTasks))
	}
	return issues
}

// cycle returns the slice numbers on one `after` cycle, or nil.
func (p SlicePlan) cycle() []int {
	after := map[int][]int{}
	for _, s := range p.Slices {
		after[s.N] = s.After
	}
	state := map[int]int{}
	var stack []int
	var found []int
	var visit func(n int) bool
	visit = func(n int) bool {
		state[n] = 1
		stack = append(stack, n)
		for _, a := range after[n] {
			if _, ok := after[a]; !ok || a == n {
				continue
			}
			if state[a] == 1 {
				for i, v := range stack {
					if v == a {
						found = append([]int(nil), stack[i:]...)
						return true
					}
				}
			}
			if state[a] == 0 && visit(a) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = 2
		return false
	}
	for _, s := range p.Slices {
		if state[s.N] == 0 && visit(s.N) {
			return found
		}
	}
	return nil
}

// SliceBinding records the branch and worktree `slice start` created for a
// slice. It is local state in the repository's git dir, never committed.
type SliceBinding struct {
	Branch   string `json:"branch"`
	Worktree string `json:"worktree,omitempty"`
	Base     string `json:"base,omitempty"`
}

// sliceBindingsPath is where a change's bindings live: under the repository's
// common git dir, so every worktree of the change reads the same file and no
// worktree holds a copy. Outside a git repository it falls back to the change
// folder's own .specsync/.
func sliceBindingsPath(changeDir string) string {
	if common := gitCommonDir(changeDir); common != "" {
		return filepath.Join(common, "specsync", "slices", filepath.Base(changeDir)+".json")
	}
	return filepath.Join(changeDir, ".specsync", "slices.json")
}

func gitCommonDir(dir string) string {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	out, err := gitIn(context.Background(), dir, "rev-parse", "--git-common-dir")
	if err != nil || out == "" {
		return ""
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(dir, out)
	}
	return filepath.Clean(out)
}

// mainCheckout is the root of the repository's main checkout, whichever of its
// worktrees dir is.
func mainCheckout(dir string) string {
	if common := gitCommonDir(dir); common != "" && filepath.Base(common) == ".git" {
		return filepath.Dir(common)
	}
	return dir
}

// LoadSliceBindings reads the recorded bindings, keyed by slice number.
func LoadSliceBindings(changeDir string) (map[int]SliceBinding, error) {
	out := map[int]SliceBinding{}
	data, err := os.ReadFile(sliceBindingsPath(changeDir))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	raw := map[string]SliceBinding{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("read %s: %w", sliceBindingsPath(changeDir), err)
	}
	for k, b := range raw {
		if n, err := strconv.Atoi(k); err == nil {
			out[n] = b
		}
	}
	return out, nil
}

// SaveSliceBinding records slice n's binding, keeping the others.
func SaveSliceBinding(changeDir string, n int, b SliceBinding) error {
	all, err := LoadSliceBindings(changeDir)
	if err != nil {
		return err
	}
	all[n] = b
	raw := map[string]SliceBinding{}
	for k, v := range all {
		raw[strconv.Itoa(k)] = v
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sliceBindingsPath(changeDir)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sliceBindingsPath(changeDir), append(data, '\n'), 0o644)
}

// SliceStatus is a slice with its derived state: not started, branch exists,
// open, merged or closed. State comes only from a recorded binding, whether the
// branch exists, and the provider's change request on that branch.
type SliceStatus struct {
	Slice
	State   string         `json:"state"`
	Branch  string         `json:"branch,omitempty"`
	PR      *ChangeRequest `json:"pr,omitempty"`
	Binding *SliceBinding  `json:"binding,omitempty"`
}

// SliceStatuses derives each slice's state, in slice-number order. requests may
// be nil when the provider could not be asked.
func SliceStatuses(plan SlicePlan, bindings map[int]SliceBinding, branchExists func(string) bool, requests *ChangeRequestSet) []SliceStatus {
	slices := append([]Slice(nil), plan.Slices...)
	sort.SliceStable(slices, func(i, j int) bool { return slices[i].N < slices[j].N })
	out := make([]SliceStatus, 0, len(slices))
	for _, s := range slices {
		st := SliceStatus{Slice: s, State: "not started"}
		if b, ok := bindings[s.N]; ok && b.Branch != "" {
			b := b
			exists := branchExists(b.Branch)
			var cr *ChangeRequest
			if requests != nil {
				cr = requests.ForBranch(b.Branch)
			}
			switch {
			case cr != nil && (cr.State == "open" || cr.State == "merged" || cr.State == "closed"):
				st.PR, st.State = cr, cr.State
				st.Binding, st.Branch = &b, b.Branch
			case exists:
				st.Binding, st.Branch, st.State = &b, b.Branch, "branch exists"
			}
		}
		out = append(out, st)
	}
	return out
}

// StartSliceOptions configures StartSlice.
type StartSliceOptions struct {
	RepoDir     string
	ChangeDir   string
	Slug        string
	IssueID     string // optional; names the branch feat/<issue>-<slug>-slice-<n>
	N           int
	WorktreeDir string // folder that holds the worktrees; default ../worktrees next to RepoDir
	Base        string // base for a slice with no `after`; default origin/HEAD
	DryRun      bool
}

// StartSliceResult says what StartSlice did, or on a dry run would do.
type StartSliceResult struct {
	Branch   string
	Worktree string
	Base     string
	Stacked  bool
	Created  bool
}

// BranchRef returns the ref a branch can be based on: the local branch, else
// origin/<branch>, else "" when neither exists.
func BranchRef(ctx context.Context, repoDir, branch string) string {
	if _, err := gitIn(ctx, repoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return branch
	}
	if _, err := gitIn(ctx, repoDir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch); err == nil {
		return "origin/" + branch
	}
	return ""
}

// StartSlice creates slice N's branch and worktree and records the binding. A
// slice with `after` is based on the branch of the last slice it names (the
// local branch, else its remote), so slices form a stack; when that branch no
// longer exists, because the slice merged and its branch was deleted, or when
// the slice has no `after`, it is based on Base (default origin/HEAD). It never
// creates a change request, and starting an already started slice changes
// nothing.
func StartSlice(ctx context.Context, opts StartSliceOptions) (*StartSliceResult, error) {
	tasks, err := os.ReadFile(filepath.Join(opts.ChangeDir, "tasks.md"))
	if err != nil {
		return nil, fmt.Errorf("read tasks.md: %w", err)
	}
	plan := ParseSlices(string(tasks))
	s, ok := plan.Get(opts.N)
	if !ok {
		return nil, fmt.Errorf("change %s declares no slice %d", opts.Slug, opts.N)
	}
	bindings, err := LoadSliceBindings(opts.ChangeDir)
	if err != nil {
		return nil, err
	}

	root := mainCheckout(opts.RepoDir)
	name := opts.Slug
	if opts.IssueID != "" {
		name = opts.IssueID + "-" + opts.Slug
	}
	res := &StartSliceResult{Branch: fmt.Sprintf("feat/%s-slice-%d", name, s.N)}
	wtDir := opts.WorktreeDir
	if wtDir == "" {
		wtDir = filepath.Join(filepath.Dir(root), "worktrees")
	}
	res.Worktree = filepath.Join(wtDir, fmt.Sprintf("%s-%s-slice-%d", filepath.Base(root), name, s.N))

	if b, ok := bindings[s.N]; ok && b.Branch != "" && BranchRef(ctx, opts.RepoDir, b.Branch) != "" {
		if _, err := os.Stat(b.Worktree); err == nil {
			return &StartSliceResult{Branch: b.Branch, Worktree: b.Worktree, Base: b.Base, Stacked: len(s.After) > 0}, nil
		}
	}

	if len(s.After) > 0 {
		prev := s.After[len(s.After)-1]
		pb, ok := bindings[prev]
		if !ok || pb.Branch == "" {
			return nil, fmt.Errorf("slice %d comes after slice %d, which has not been started: run `specsync slice start %d` first", s.N, prev, prev)
		}
		if ref := BranchRef(ctx, opts.RepoDir, pb.Branch); ref != "" {
			res.Base, res.Stacked = ref, true
		}
	}
	if res.Base == "" {
		res.Base = opts.Base
		if res.Base == "" {
			if res.Base, err = DefaultBase(ctx, opts.RepoDir); err != nil {
				return nil, fmt.Errorf("no base for slice %d: %w; pass -base", s.N, err)
			}
		}
	}
	if opts.DryRun {
		return res, nil
	}

	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree folder: %w", err)
	}
	args := []string{"worktree", "add", "-b", res.Branch, res.Worktree, res.Base}
	if _, err := gitIn(ctx, opts.RepoDir, "rev-parse", "--verify", "--quiet", "refs/heads/"+res.Branch); err == nil {
		args = []string{"worktree", "add", res.Worktree, res.Branch}
	}
	if _, err := gitIn(ctx, opts.RepoDir, args...); err != nil {
		return nil, err
	}
	res.Created = true
	if err := SaveSliceBinding(opts.ChangeDir, s.N, SliceBinding{Branch: res.Branch, Worktree: res.Worktree, Base: res.Base}); err != nil {
		return nil, err
	}
	return res, nil
}
