package specsync

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// ptr returns a pointer to an int (test fixture helper).
func ptr(v int) *int { return &v }

// Priority bounds are inclusive: 1 and 100 both load.
func TestPriorityBounds(t *testing.T) {
	for _, p := range []int{1, 100} {
		root := t.TempDir()
		cdir := filepath.Join(root, "changes", "test")
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
		mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
			fmt.Sprintf(`{"version":1,"priority":%d}`, p))

		c, err := LoadChange(cdir, false, root)
		if err != nil {
			t.Fatalf("LoadChange(priority=%d): %v", p, err)
		}
		if c.Priority == nil || *c.Priority != p {
			t.Errorf("priority = %v, want %d", c.Priority, p)
		}
	}
}

// Out-of-range priorities in metadata.json are a load error, not silently accepted.
func TestPriorityOutOfRangeRejected(t *testing.T) {
	for _, p := range []int{0, -1, 101} {
		root := t.TempDir()
		cdir := filepath.Join(root, "changes", "test")
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
		mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
			fmt.Sprintf(`{"version":1,"priority":%d}`, p))

		_, err := LoadChange(cdir, false, root)
		if err == nil {
			t.Errorf("LoadChange(priority=%d): want error, got nil", p)
		} else if !strings.Contains(err.Error(), "priority") {
			t.Errorf("LoadChange(priority=%d): error should mention priority: %v", p, err)
		}
	}
}

// Priorities load correctly across a directory of changes, nil when unset.
func TestPriorityLoadMultipleChanges(t *testing.T) {
	root := t.TempDir()

	priorities := map[string]*int{
		"critical": ptr(95),
		"high":     ptr(75),
		"low":      ptr(5), // single digit: exercises plain JSON numbers
		"none":     nil,    // no metadata file at all
	}
	for slug, p := range priorities {
		cdir := filepath.Join(root, "changes", slug)
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# "+slug+"\n")
		if p != nil {
			mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
				fmt.Sprintf(`{"version":1,"priority":%d}`, *p))
		}
	}

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}
	if len(changes) != len(priorities) {
		t.Fatalf("loaded %d changes, want %d", len(changes), len(priorities))
	}

	for _, c := range changes {
		want := priorities[c.Slug]
		switch {
		case want == nil && c.Priority != nil:
			t.Errorf("%s: priority = %d, want nil", c.Slug, *c.Priority)
		case want != nil && (c.Priority == nil || *c.Priority != *want):
			t.Errorf("%s: priority = %v, want %d", c.Slug, c.Priority, *want)
		}
	}
}
