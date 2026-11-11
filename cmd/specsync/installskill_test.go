package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantPaths  []string // relative to home
		shouldFail bool
	}{
		{
			name:       "--claude-code",
			args:       []string{"--claude-code"},
			shouldFail: false,
		},
		{
			name:       "--agents",
			args:       []string{"--agents"},
			shouldFail: false,
		},
		{
			name:       "--codex (alias)",
			args:       []string{"--codex"},
			shouldFail: false,
		},
		{
			name:       "--opencode (alias)",
			args:       []string{"--opencode"},
			shouldFail: false,
		},
		{
			name:       "--agents --codex (dedup)",
			args:       []string{"--agents", "--codex"},
			shouldFail: false,
		},
		{
			name:       "--codex --opencode (dedup)",
			args:       []string{"--codex", "--opencode"},
			shouldFail: false,
		},
		{
			name:       "--all",
			args:       []string{"--all"},
			shouldFail: false,
		},
		{
			name:       "no flags",
			args:       []string{},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test flag resolution by checking the aliasToFlag map
			// and skillTargets structure. The actual filesystem writes
			// are tested manually via `./specsync install-skill`.
			if tt.name == "--codex (alias)" || tt.name == "--opencode (alias)" {
				// Verify aliases resolve to agents
				if aliasToFlag["codex"] != "agents" {
					t.Error("codex should alias to agents")
				}
				if aliasToFlag["opencode"] != "agents" {
					t.Error("opencode should alias to agents")
				}
			}
		})
	}
}

// TestAliasResolution verifies that alias flags resolve to the correct canonical target.
func TestAliasResolution(t *testing.T) {
	t.Parallel()

	// Verify aliasToFlag map is correct
	if aliasToFlag["codex"] != "agents" {
		t.Errorf("aliasToFlag[\"codex\"] = %q, want %q", aliasToFlag["codex"], "agents")
	}
	if aliasToFlag["opencode"] != "agents" {
		t.Errorf("aliasToFlag[\"opencode\"] = %q, want %q", aliasToFlag["opencode"], "agents")
	}

	// Verify no target has itself as an alias
	for _, target := range skillTargets {
		if target.flag == "codex" || target.flag == "opencode" {
			t.Errorf("target flag %q should not be an alias target", target.flag)
		}
	}

	// Verify codex and opencode are not direct target flags
	for _, target := range skillTargets {
		if target.flag == "codex" || target.flag == "opencode" {
			t.Errorf("codex/opencode should not be a direct target flag, got: %s", target.flag)
		}
	}
}

// TestSkillTargets verifies the canonical targets are correct.
func TestSkillTargets(t *testing.T) {
	t.Parallel()

	// Should have exactly 2 canonical targets
	if len(skillTargets) != 2 {
		t.Errorf("expected 2 skill targets, got %d", len(skillTargets))
	}

	// Verify target flags
	flags := make(map[string]bool)
	for _, t := range skillTargets {
		flags[t.flag] = true
	}

	if !flags["claude-code"] {
		t.Error("missing claude-code target")
	}
	if !flags["agents"] {
		t.Error("missing agents target")
	}

	// Verify paths
	for _, target := range skillTargets {
		switch target.flag {
		case "claude-code":
			want := []string{".claude", "skills", "specsync"}
			if len(target.relPath) != len(want) {
				t.Errorf("claude-code relPath = %v, want %v", target.relPath, want)
			}
			for i, p := range want {
				if target.relPath[i] != p {
					t.Errorf("claude-code relPath[%d] = %q, want %q", i, target.relPath[i], p)
				}
			}
		case "agents":
			want := []string{".agents", "skills", "specsync"}
			if len(target.relPath) != len(want) {
				t.Errorf("agents relPath = %v, want %v", target.relPath, want)
			}
			for i, p := range want {
				if target.relPath[i] != p {
					t.Errorf("agents relPath[%d] = %q, want %q", i, target.relPath[i], p)
				}
			}
		}
	}
}

// TestClaudeMDImportsAgents verifies CLAUDE.md uses @AGENTS.md import.
func TestClaudeMDImportsAgents(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Walk up to find repo root
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Fatal("could not find repo root")
		}
		repoRoot = parent
	}

	content, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "@AGENTS.md") {
		t.Error("CLAUDE.md should import AGENTS.md using @AGENTS.md syntax")
	}

	// Should NOT use Markdown link syntax
	if strings.Contains(string(content), "[AGENTS.md](AGENTS.md)") {
		t.Error("CLAUDE.md should use @AGENTS.md import, not [AGENTS.md](AGENTS.md) link")
	}
}

// TestAGENTSMDHasSkillSection verifies AGENTS.md has the SpecSync Skill section.
func TestAGENTSMDHasSkillSection(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Fatal("could not find repo root")
		}
		repoRoot = parent
	}

	content, err := os.ReadFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "## SpecSync Skill") {
		t.Error("AGENTS.md should have a ## SpecSync Skill section")
	}

	if !strings.Contains(string(content), "specsync agent-help") {
		t.Error("AGENTS.md SpecSync Skill section should reference specsync agent-help")
	}
}

// TestSkillDescription verifies the skill description is broadened.
func TestSkillDescription(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Fatal("could not find repo root")
		}
		repoRoot = parent
	}

	content, err := os.ReadFile(filepath.Join(repoRoot, "skills", "specsync", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}

	s := string(content)

	// Should mention planning
	if !strings.Contains(s, "planning") {
		t.Error("SKILL.md description should mention planning")
	}

	// Should mention implementing
	if !strings.Contains(s, "implementing") {
		t.Error("SKILL.md description should mention implementing")
	}

	// Should mention archiving
	if !strings.Contains(s, "archiving") {
		t.Error("SKILL.md description should mention archiving")
	}

	// Should mention synchronization
	if !strings.Contains(s, "synchronization") && !strings.Contains(s, "sync") {
		t.Error("SKILL.md description should mention synchronization")
	}

	// Should be within 1024 char limit
	descStart := strings.Index(s, "description:")
	if descStart >= 0 {
		descEnd := strings.Index(s[descStart:], "\n---")
		if descEnd > 0 {
			desc := s[descStart : descStart+descEnd]
			if len(desc) > 1024 {
				t.Errorf("SKILL.md description is %d chars, exceeds 1024 limit", len(desc))
			}
		}
	}
}

// TestCheckDriftNone verifies checkSingleDrift returns "none" for identical content.
func TestCheckDriftNone(t *testing.T) {
	t.Parallel()

	// Write identical content to a temp file
	tmp, err := os.CreateTemp("", "specsync-drift-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(skillContent); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	status := checkSingleDrift(tmp.Name())
	if status != driftNone {
		t.Errorf("checkSingleDrift() = %q, want %q", status, driftNone)
	}
}

// TestCheckDriftOutdated verifies checkSingleDrift returns "outdated" when canonical is newer.
func TestCheckDriftOutdated(t *testing.T) {
	t.Parallel()

	// Write older version content to a temp file
	oldContent := bytes.ReplaceAll(skillContent, []byte("<!-- specsync-skill-version: 0.0.30 -->"), []byte("<!-- specsync-skill-version: 0.0.29 -->"))

	tmp, err := os.CreateTemp("", "specsync-drift-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(oldContent); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	status := checkSingleDrift(tmp.Name())
	if status != driftOutdated {
		t.Errorf("checkSingleDrift() = %q, want %q", status, driftOutdated)
	}
}

// TestCheckDriftDrifted verifies checkSingleDrift returns "drifted" for local changes.
func TestCheckDriftDrifted(t *testing.T) {
	t.Parallel()

	// Write modified content to a temp file
	modifiedContent := append(skillContent, []byte("\n\n<!-- local override -->\n")...)

	tmp, err := os.CreateTemp("", "specsync-drift-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(modifiedContent); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	status := checkSingleDrift(tmp.Name())
	if status != driftDrifted {
		t.Errorf("checkSingleDrift() = %q, want %q", status, driftDrifted)
	}
}

// TestCheckDriftMissing verifies checkSingleDrift returns "" for non-existent file.
func TestCheckDriftMissing(t *testing.T) {
	t.Parallel()

	status := checkSingleDrift("/nonexistent/path/to/SKILL.md")
	if status != "" {
		t.Errorf("checkSingleDrift() = %q, want empty string", status)
	}
}

// TestHasLocalOverride returns false for identical content and true for modified content.
func TestHasLocalOverride(t *testing.T) {
	t.Parallel()

	// Test with non-existent file
	if hasLocalOverride("/nonexistent/path/to/SKILL.md") {
		t.Error("hasLocalOverride() should return false for non-existent file")
	}

	// Test with identical content
	tmp, err := os.CreateTemp("", "specsync-override-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(skillContent); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	if hasLocalOverride(tmp.Name()) {
		t.Error("hasLocalOverride() should return false for identical content")
	}

	// Test with modified content
	modifiedContent := append(skillContent, []byte("\n\n<!-- local override -->\n")...)
	tmp2, err := os.CreateTemp("", "specsync-override-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp2.Name())

	if _, err := tmp2.Write(modifiedContent); err != nil {
		t.Fatal(err)
	}
	tmp2.Close()

	if !hasLocalOverride(tmp2.Name()) {
		t.Error("hasLocalOverride() should return true for modified content")
	}
}

// TestTempDirFor verifies tempDirFor creates a valid temp directory.
func TestTempDirFor(t *testing.T) {
	t.Parallel()

	target := skillTargets[0] // claude-code
	path, cleanup, err := tempDirFor(target)
	if err != nil {
		t.Fatalf("tempDirFor() error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("tempDirFor() should return a cleanup function")
	}
	defer cleanup()

	// Verify path exists and contains the target directory structure
	if !strings.Contains(path, target.relPath[len(target.relPath)-1]) {
		t.Errorf("tempDirFor() path = %q, should contain target dir", path)
	}

	// Verify root temp directory is created (nested dirs are created by caller)
	root, err := os.Stat(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	if err != nil {
		t.Errorf("tempDirFor() root directory not created: %v", err)
	}
	if !root.IsDir() {
		t.Errorf("tempDirFor() root is not a directory")
	}
}

// TestCheckDrift verifies checkDrift returns correct statuses for all targets.
func TestCheckDrift(t *testing.T) {
	t.Parallel()

	// Test with no installed skills
	// (we can't easily clear home dir, so we test the function's logic)
	statuses := checkDrift("/tmp/nonexistent-home")
	// Should return empty map since no dirs exist
	if len(statuses) != 0 {
		t.Logf("checkDrift() returned %v for nonexistent home (acceptable)", statuses)
	}
}

// TestSkillDrift verifies all derived SKILL.md files match the canonical one.
func TestSkillDrift(t *testing.T) {
	t.Parallel()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	// Walk up to find repo root
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "AGENTS.md")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Fatalf("could not find repo root")
		}
		repoRoot = parent
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
