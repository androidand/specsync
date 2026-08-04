package specsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSignificant(t *testing.T) {
	dir := t.TempDir()

	// No signal: not significant.
	tasks := "- [ ] do something\n"
	if isSignificant(dir, tasks) {
		t.Errorf("no signal: expected not significant")
	}

	// Explicit marker file.
	markerPath := filepath.Join(dir, significantMarker)
	if err := os.WriteFile(markerPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !isSignificant(dir, tasks) {
		t.Errorf("marker present: expected significant")
	}
	os.Remove(markerPath)

	// design.md present.
	designPath := filepath.Join(dir, designDoc)
	if err := os.WriteFile(designPath, []byte("# Design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isSignificant(dir, tasks) {
		t.Errorf("design.md present: expected significant")
	}
	os.Remove(designPath)

	// Task count >= threshold.
	multiTask := ""
	for i := 0; i < defaultTaskThreshold; i++ {
		multiTask += "- [ ] task " + string(rune('a'+i)) + "\n"
	}
	if !isSignificant(dir, multiTask) {
		t.Errorf("task count >= %d: expected significant", defaultTaskThreshold)
	}

	// Task count below threshold.
	singleTask := "- [ ] do one thing\n"
	if isSignificant(dir, singleTask) {
		t.Errorf("task count < %d: expected not significant", defaultTaskThreshold)
	}

	// Marker overrides heuristics.
	os.WriteFile(markerPath, nil, 0o644)
	os.WriteFile(designPath, []byte("# Design\n"), 0o644)
	if !isSignificant(dir, singleTask) {
		t.Errorf("marker + design + few tasks: expected significant")
	}
	os.Remove(markerPath)
	os.Remove(designPath)

	// Empty tasks, no signals.
	if isSignificant(dir, "") {
		t.Errorf("empty tasks, no signals: expected not significant")
	}

	// Empty tasks, marker present.
	os.WriteFile(markerPath, nil, 0o644)
	if !isSignificant(dir, "") {
		t.Errorf("empty tasks, marker present: expected significant")
	}
	os.Remove(markerPath)
}

func TestLoadChangeSignificant(t *testing.T) {
	openspecDir := t.TempDir()
	changeDir := filepath.Join(openspecDir, "changes", "test-significant")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write proposal.md.
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Test change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No significant signal.
	c, err := LoadChange(changeDir, false, openspecDir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Significant {
		t.Errorf("no signal: expected Significant=false")
	}

	// Add marker file.
	if err := os.WriteFile(filepath.Join(changeDir, significantMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	c, err = LoadChange(changeDir, false, openspecDir)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Significant {
		t.Errorf("marker present: expected Significant=true")
	}

	// Remove marker, add design.md.
	os.Remove(filepath.Join(changeDir, significantMarker))
	if err := os.WriteFile(filepath.Join(changeDir, designDoc), []byte("# Design\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err = LoadChange(changeDir, false, openspecDir)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Significant {
		t.Errorf("design.md present: expected Significant=true")
	}
}

func TestIsSignificantThreshold(t *testing.T) {
	dir := t.TempDir()

	// Just below threshold.
	below := ""
	for i := 0; i < defaultTaskThreshold-1; i++ {
		below += "- [ ] task " + string(rune('a'+i)) + "\n"
	}
	if isSignificant(dir, below) {
		t.Errorf("task count %d (below threshold %d): expected not significant", defaultTaskThreshold-1, defaultTaskThreshold)
	}

	// Exactly at threshold.
	at := ""
	for i := 0; i < defaultTaskThreshold; i++ {
		at += "- [ ] task " + string(rune('a'+i)) + "\n"
	}
	if !isSignificant(dir, at) {
		t.Errorf("task count %d (at threshold %d): expected significant", defaultTaskThreshold, defaultTaskThreshold)
	}

	// Above threshold.
	above := ""
	for i := 0; i < defaultTaskThreshold+10; i++ {
		above += "- [ ] task " + string(rune('a'+i)) + "\n"
	}
	if !isSignificant(dir, above) {
		t.Errorf("task count %d (above threshold %d): expected significant", defaultTaskThreshold+10, defaultTaskThreshold)
	}
}
