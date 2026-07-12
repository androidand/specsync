package specsync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillDrift(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	canon := filepath.Join(repoRoot, "skills", "specsync", "SKILL.md")
	derived := []string{
		filepath.Join(repoRoot, "cmd", "specsync", "SKILL.md"),
		filepath.Join(repoRoot, "npm", "skills", "specsync", "SKILL.md"),
		filepath.Join(repoRoot, ".claude", "skills", "specsync", "SKILL.md"),
	}

	want, err := os.ReadFile(canon)
	if err != nil {
		t.Fatalf("canonical skill unreadable: %v", err)
	}

	for _, path := range derived {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("derived skill %s unreadable: %v", path, err)
		}

		if !bytes.Equal(want, got) {
			t.Errorf("derived %s drifts from canonical:\n--- canonical ---\n%s\n--- derived ---\n%s",
				filepath.Base(path), want, got)
		}
	}
}

func TestSkillInNpmPackage(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	// The npm package.json must list skills/ in the files array, so the
	// published tarball includes the canonical skill.
	path := filepath.Join(repoRoot, "npm", "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("npm/package.json unreadable: %v", err)
	}

	var pkg struct {
		Name  string   `json:"name"`
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("npm/package.json invalid JSON: %v", err)
	}

	found := false
	for _, f := range pkg.Files {
		if f == "skills/" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("npm package.json files does not include \"skills/\": %v", pkg.Files)
	}

	// Verify the canonical skill file exists and is readable.
	canon := filepath.Join(repoRoot, "skills", "specsync", "SKILL.md")
	if _, err := os.Stat(canon); os.IsNotExist(err) {
		t.Errorf("canonical skill does not exist: %s", canon)
	}
}
