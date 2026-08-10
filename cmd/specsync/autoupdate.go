package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// skillVersionRegex extracts the semantic version from the skill version marker.
// Matches: <!-- specsync-skill-version: X.Y.Z -->
var skillVersionRegex = regexp.MustCompile(`<!--\s*specsync-skill-version:\s*([^\s]+)\s*-->`)

// extractSkillVersion extracts the version string from embedded SKILL.md content.
// Returns empty string if the marker is not found.
func extractSkillVersion(content []byte) string {
	matches := skillVersionRegex.FindSubmatch(content)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return ""
}

// readInstalledSkillVersion reads the version marker from an installed skill file.
// Returns empty string if the file doesn't exist or has no marker.
func readInstalledSkillVersion(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return extractSkillVersion(content)
}

// versionCompare returns true if current > installed semantically.
// Simplified: splits on dots, compares numeric segments left-to-right.
// Returns false if either version is unparseable or equal.
func versionCompare(current, installed string) bool {
	if current == "" || installed == "" {
		return false
	}
	if current == installed {
		return false
	}

	currParts := strings.Split(strings.TrimSpace(current), ".")
	instParts := strings.Split(strings.TrimSpace(installed), ".")

	// Compare up to the length of the shorter version
	minLen := len(currParts)
	if len(instParts) < minLen {
		minLen = len(instParts)
	}

	for i := 0; i < minLen; i++ {
		var currNum, instNum int
		_, _ = fmt.Sscanf(currParts[i], "%d", &currNum)
		_, _ = fmt.Sscanf(instParts[i], "%d", &instNum)

		if currNum > instNum {
			return true
		}
		if currNum < instNum {
			return false
		}
	}

	// If all compared parts are equal, current is newer if it has more parts
	return len(currParts) > len(instParts)
}

// checkSkillsStale checks if any installed skill files are older than the binary version.
// Returns true if at least one skill is stale, false otherwise.
func checkSkillsStale(binaryVersion string) bool {
	if binaryVersion == "" || binaryVersion == "dev" {
		// Don't auto-update dev builds
		return false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check each skill target directory
	for _, t := range skillTargets {
		skillPath := filepath.Join(append([]string{home}, t.relPath...)...)
		skillFile := filepath.Join(skillPath, "SKILL.md")

		installedVersion := readInstalledSkillVersion(skillFile)
		// If installed version is older than binary version, skills are stale
		if installedVersion != "" && versionCompare(binaryVersion, installedVersion) {
			return true
		}
	}

	return false
}

// updateSkillsIfNeeded checks if skills are stale and updates them silently if so.
// Returns true if an update was performed, false otherwise.
// Errors are logged but do not block execution.
func updateSkillsIfNeeded(binaryVersion string) bool {
	if !checkSkillsStale(binaryVersion) {
		return false
	}

	// Call install-skill to update all targets
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "specsync: warning: could not auto-update skills (%v)\n", err)
		return false
	}

	var updated int
	for _, t := range skillTargets {
		dir := filepath.Join(append([]string{home}, t.relPath...)...)
		dest := filepath.Join(dir, "SKILL.md")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		if err := os.WriteFile(dest, skillContent, 0o644); err != nil {
			continue
		}
		updated++
	}

	if updated > 0 {
		fmt.Fprintf(os.Stderr, "specsync: auto-updated %d skill file(s) to version %s\n", updated, binaryVersion)
		return true
	}

	return false
}
