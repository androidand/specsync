package specsync

import (
	"reflect"
	"strings"
	"testing"
)

const validPlan = `# Tasks

## Slice 1: Parser
contract: parse the grammar
budget: files<=5, lines<=200
- [x] 1.1 grammar
- [ ] 1.2 fixtures

## Slice 2: Commands
after: 1
contract: list and start slices
- [ ] 2.1 slices command
`

func TestParseSlices_ValidPlan(t *testing.T) {
	p := ParseSlices(validPlan)
	if len(p.Slices) != 2 {
		t.Fatalf("slices = %+v", p.Slices)
	}
	s1, s2 := p.Slices[0], p.Slices[1]
	if s1.N != 1 || s1.Title != "Parser" || s1.Contract != "parse the grammar" || s1.Budget != "files<=5, lines<=200" || s1.Tasks != 2 || s1.Done != 1 {
		t.Fatalf("slice 1 = %+v", s1)
	}
	if s2.N != 2 || !reflect.DeepEqual(s2.After, []int{1}) || s2.Tasks != 1 || s2.Done != 0 {
		t.Fatalf("slice 2 = %+v", s2)
	}
	if p.OrphanTasks != 0 {
		t.Fatalf("orphans = %d", p.OrphanTasks)
	}
	if issues := p.Validate(); len(issues) != 0 {
		t.Fatalf("a valid plan must have no issues: %v", issues)
	}
}

func TestParseSlices_NoSlicesIsEmptyAndValid(t *testing.T) {
	p := ParseSlices("# Tasks\n\n## 1. Work\n- [ ] a\n- [x] b\n")
	if len(p.Slices) != 0 || len(p.Validate()) != 0 {
		t.Fatalf("a change without slice headings must behave as before: %+v %v", p, p.Validate())
	}
}

func TestSlicePlanValidate(t *testing.T) {
	cases := []struct {
		name, md, want string
	}{
		{"missing contract", "## Slice 1: A\n- [ ] a\n", "slice 1 has no contract"},
		{"gap", "## Slice 1: A\ncontract: x\n- [ ] a\n## Slice 3: B\ncontract: y\n- [ ] b\n", "not contiguous"},
		{"duplicate", "## Slice 1: A\ncontract: x\n- [ ] a\n## Slice 1: B\ncontract: y\n- [ ] b\n", "declared twice"},
		{"unknown after", "## Slice 1: A\ncontract: x\nafter: 7\n- [ ] a\n", "after 7 does not exist"},
		{"self after", "## Slice 1: A\ncontract: x\nafter: 1\n- [ ] a\n", "cannot come after itself"},
		{"cycle", "## Slice 1: A\ncontract: x\nafter: 2\n- [ ] a\n## Slice 2: B\ncontract: y\nafter: 1\n- [ ] b\n", "slices 1 and 2 form a cycle"},
		{"orphan task", "# Tasks\n- [ ] stray\n## Slice 1: A\ncontract: x\n- [ ] a\n", "1 task(s) belong to no slice"},
		{"task under a plain heading", "## Slice 1: A\ncontract: x\n- [ ] a\n## Docs\n- [ ] b\n", "1 task(s) belong to no slice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := ParseSlices(c.md).Validate()
			if !strings.Contains(strings.Join(issues, "\n"), c.want) {
				t.Fatalf("want an issue containing %q, got %v", c.want, issues)
			}
		})
	}
}

func TestParseSlices_StartsAtZeroAndAfterList(t *testing.T) {
	p := ParseSlices("## Slice 0: A\ncontract: x\n- [ ] a\n## Slice 1: B\ncontract: y\nafter: 0\n- [ ] b\n## Slice 2: C\ncontract: z\nafter: 0, 1\n- [ ] c\n")
	if issues := p.Validate(); len(issues) != 0 {
		t.Fatalf("a plan may start at 0: %v", issues)
	}
	if !reflect.DeepEqual(p.Slices[2].After, []int{0, 1}) {
		t.Fatalf("after = %v", p.Slices[2].After)
	}
}

func TestParseSlices_NeverInventsSlices(t *testing.T) {
	for _, md := range []string{"", "# Tasks\n- [ ] a\n", "Slice 1: not a heading\n- [ ] a\n", "### Slice without number\n- [ ] a\n"} {
		if n := len(ParseSlices(md).Slices); n != 0 {
			t.Fatalf("%q produced %d slices; only declared headings count", md, n)
		}
	}
}

func TestSlicePlanValidate_MalformedAndDuplicateAfterAreReported(t *testing.T) {
	for name, c := range map[string]struct{ md, want string }{
		"malformed": {"## Slice 1: A\ncontract: x\n- [ ] a\n## Slice 2: B\ncontract: y\nafter: one\n- [ ] b\n", "slice 2: malformed after"},
		"partly":    {"## Slice 1: A\ncontract: x\n- [ ] a\n## Slice 2: B\ncontract: y\nafter: 1, x\n- [ ] b\n", "slice 2: malformed after"},
		"twice":     {"## Slice 1: A\ncontract: x\n- [ ] a\n## Slice 2: B\ncontract: y\nafter: 1\nafter: 1\n- [ ] b\n", "slice 2: after is declared more than once"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := strings.Join(ParseSlices(c.md).Validate(), "\n"); !strings.Contains(got, c.want) {
				t.Fatalf("want %q in %q", c.want, got)
			}
		})
	}
}
