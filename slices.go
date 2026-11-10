package specsync

import (
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
		case sliceAfterRe.MatchString(line) && s.After == nil:
			s.After = parseSliceNumbers(sliceAfterRe.FindStringSubmatch(line)[1])
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

func parseSliceNumbers(s string) []int {
	var out []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		f = strings.TrimPrefix(strings.ToLower(f), "slice")
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil {
			out = append(out, n)
		}
	}
	return out
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
	var issues []string
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
// slice. It is local state under the change's .specsync/ folder, never committed.
type SliceBinding struct {
	Branch   string `json:"branch"`
	Worktree string `json:"worktree,omitempty"`
	Base     string `json:"base,omitempty"`
}

func sliceBindingsPath(changeDir string) string {
	return filepath.Join(changeDir, ".specsync", "slices.json")
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
		if b, ok := bindings[s.N]; ok && b.Branch != "" && branchExists(b.Branch) {
			b := b
			st.Binding, st.Branch, st.State = &b, b.Branch, "branch exists"
			if requests != nil {
				if cr := requests.ForBranch(b.Branch); cr != nil {
					st.PR = cr
					if cr.State == "open" || cr.State == "merged" || cr.State == "closed" {
						st.State = cr.State
					}
				}
			}
		}
		out = append(out, st)
	}
	return out
}
