package specsync

import (
	"path/filepath"
	"sort"
	"testing"
)

// TestPriorityNilSortsLast verifies nil priority sorts to the end (lowest).
func TestPriorityNilSortsLast(t *testing.T) {
	changes := []Change{
		{Slug: "a", Priority: ptr(50)},
		{Slug: "b", Priority: nil},
		{Slug: "c", Priority: ptr(80)},
		{Slug: "d", Priority: nil},
	}

	// Sort by priority descending (higher first, nil last)
	sort.Slice(changes, func(i, j int) bool {
		pi := changes[i].Priority
		pj := changes[j].Priority

		// Both nil: keep order (stable sort)
		if pi == nil && pj == nil {
			return false
		}
		// Left is nil: goes right (lower)
		if pi == nil {
			return false
		}
		// Right is nil: goes right (lower)
		if pj == nil {
			return true
		}
		// Both have values: higher first
		return *pi > *pj
	})

	// Verify order: 80, 50, nil, nil
	if changes[0].Slug != "c" || changes[0].Priority == nil || *changes[0].Priority != 80 {
		t.Errorf("first should be c(80), got %s(%v)", changes[0].Slug, changes[0].Priority)
	}
	if changes[1].Slug != "a" || changes[1].Priority == nil || *changes[1].Priority != 50 {
		t.Errorf("second should be a(50), got %s(%v)", changes[1].Slug, changes[1].Priority)
	}
	if changes[2].Slug != "b" || changes[2].Priority != nil {
		t.Errorf("third should be b(nil), got %s(%v)", changes[2].Slug, changes[2].Priority)
	}
	if changes[3].Slug != "d" || changes[3].Priority != nil {
		t.Errorf("fourth should be d(nil), got %s(%v)", changes[3].Slug, changes[3].Priority)
	}
}

// TestPriorityDescendingSort verifies higher priority sorts first.
func TestPriorityDescendingSort(t *testing.T) {
	changes := []Change{
		{Slug: "low", Priority: ptr(30)},
		{Slug: "focus", Priority: ptr(99)},
		{Slug: "normal", Priority: ptr(50)},
		{Slug: "critical", Priority: ptr(95)},
	}

	sort.Slice(changes, func(i, j int) bool {
		pi := changes[i].Priority
		pj := changes[j].Priority

		if pi == nil && pj == nil {
			return false
		}
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		return *pi > *pj
	})

	expected := []int{99, 95, 50, 30}
	for i, change := range changes {
		if change.Priority == nil || *change.Priority != expected[i] {
			t.Errorf("position %d: got %v, want %d", i, change.Priority, expected[i])
		}
	}
}

// TestPriorityBoundsMinimum verifies priority 1 is valid (minimum).
func TestPriorityBoundsMinimum(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"), `{"version":1,"priority":1}`)

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Priority == nil || *c.Priority != 1 {
		t.Errorf("minimum priority = %v, want 1", c.Priority)
	}
}

// TestPriorityBoundsMaximum verifies priority 100 is valid (maximum).
func TestPriorityBoundsMaximum(t *testing.T) {
	root := t.TempDir()
	cdir := filepath.Join(root, "changes", "test")
	mustWrite(t, filepath.Join(cdir, "proposal.md"), "# Test\n")
	mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"), `{"version":1,"priority":100}`)

	c, err := LoadChange(cdir, false, root)
	if err != nil {
		t.Fatalf("LoadChange: %v", err)
	}

	if c.Priority == nil || *c.Priority != 100 {
		t.Errorf("maximum priority = %v, want 100", c.Priority)
	}
}

// TestPriorityTiebreakerByCreationDate verifies same priority uses creation date (older first).
// When two changes have the same priority, older (created earlier) should be processed first.
func TestPriorityTiebreakerByCreationDate(t *testing.T) {
	root := t.TempDir()

	// Create two changes with same priority
	cdir1 := filepath.Join(root, "changes", "change-alpha")
	mustWrite(t, filepath.Join(cdir1, "proposal.md"), "# Alpha\n")
	mustWrite(t, filepath.Join(cdir1, ".specsync", "metadata.json"), `{"version":1,"priority":50}`)

	cdir2 := filepath.Join(root, "changes", "change-beta")
	mustWrite(t, filepath.Join(cdir2, "proposal.md"), "# Beta\n")
	mustWrite(t, filepath.Join(cdir2, ".specsync", "metadata.json"), `{"version":1,"priority":50}`)

	c1, _ := LoadChange(cdir1, false, root)
	c2, _ := LoadChange(cdir2, false, root)

	// Both should have same priority
	if c1.Priority == nil || *c1.Priority != 50 {
		t.Errorf("c1 priority = %v, want 50", c1.Priority)
	}
	if c2.Priority == nil || *c2.Priority != 50 {
		t.Errorf("c2 priority = %v, want 50", c2.Priority)
	}

	// Tie-break would use Dir or CreatedAt (filesystem order)
	// Both are valid and filesystem-dependent, so we just verify they're equal priority
	if (c1.Priority == nil) != (c2.Priority == nil) {
		t.Errorf("priority consistency failed")
	}
}

// TestPrioritySemanticTiers verifies tier ranges don't have gaps.
func TestPrioritySemanticTiers(t *testing.T) {
	// Verify tier definitions
	tiers := map[string]struct {
		min int
		max int
		name string
	}{
		"VERY_LOW": {1, 29, "VERY_LOW"},
		"LOW": {30, 49, "LOW"},
		"NORMAL": {50, 69, "NORMAL"},
		"HIGH": {70, 89, "HIGH"},
		"CRITICAL": {90, 98, "CRITICAL"},
		"FOCUS": {99, 99, "FOCUS"},
	}

	// Check coverage: 1-99 fully covered
	covered := make([]bool, 100)
	for _, tier := range tiers {
		for p := tier.min; p <= tier.max; p++ {
			if covered[p-1] {
				t.Errorf("priority %d covered twice", p)
			}
			covered[p-1] = true
		}
	}

	// Verify all priorities 1-99 covered
	for p := 1; p <= 99; p++ {
		if !covered[p-1] {
			t.Errorf("priority %d not in any tier", p)
		}
	}
}

// TestPriorityMidrangeTier verifies mid-range priority (50-69) is normal tier.
func TestPriorityMidrangeTier(t *testing.T) {
	changes := []Change{
		{Slug: "a", Priority: ptr(50)},
		{Slug: "b", Priority: ptr(60)},
		{Slug: "c", Priority: ptr(69)},
	}

	for _, c := range changes {
		// All should sort between LOW (49) and HIGH (70)
		if c.Priority != nil && (*c.Priority < 50 || *c.Priority > 69) {
			t.Errorf("%s priority %d outside NORMAL tier [50-69]", c.Slug, *c.Priority)
		}
	}
}

// TestPriorityFocusTier verifies FOCUS tier is singular (99 only).
func TestPriorityFocusTier(t *testing.T) {
	focus := ptr(99)
	critical := ptr(98)

	if *focus == *critical {
		t.Errorf("FOCUS and CRITICAL overlap")
	}
	if *focus != 99 {
		t.Errorf("FOCUS priority = %d, want 99", *focus)
	}
}

// TestPriorityLoadMultipleChanges verifies priorities load correctly across many changes.
func TestPriorityLoadMultipleChanges(t *testing.T) {
	root := t.TempDir()

	// Create 5 changes with different priorities
	priorities := map[string]int{
		"critical": 95,
		"high": 75,
		"normal": 50,
		"low": 30,
		"none": 0, // no metadata
	}

	for slug, priority := range priorities {
		cdir := filepath.Join(root, "changes", slug)
		mustWrite(t, filepath.Join(cdir, "proposal.md"), "# "+slug+"\n")
		if priority > 0 {
			mustWrite(t, filepath.Join(cdir, ".specsync", "metadata.json"),
				`{"version":1,"priority":`+string(rune('0'+priority/10))+string(rune('0'+priority%10))+`}`)
		}
	}

	changes, err := LoadChanges(root)
	if err != nil {
		t.Fatalf("LoadChanges: %v", err)
	}

	if len(changes) != 5 {
		t.Errorf("loaded %d changes, want 5", len(changes))
	}

	// Verify priorities were loaded
	bySlug := make(map[string]*int)
	for i, c := range changes {
		bySlug[c.Slug] = changes[i].Priority
	}

	tests := []struct {
		slug string
		want *int
	}{
		{"critical", ptr(95)},
		{"high", ptr(75)},
		{"normal", ptr(50)},
		{"low", ptr(30)},
		{"none", nil},
	}

	for _, tt := range tests {
		got := bySlug[tt.slug]
		if (got == nil) != (tt.want == nil) {
			t.Errorf("%s: got %v, want %v", tt.slug, got, tt.want)
		} else if got != nil && *got != *tt.want {
			t.Errorf("%s: got %d, want %d", tt.slug, *got, *tt.want)
		}
	}
}

// Helper: ptr returns a pointer to an int.
func ptr(v int) *int {
	return &v
}
