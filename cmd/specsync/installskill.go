package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed"
)

//go:embed SKILL.md
var skillContent []byte

// target describes a named agent skill directory.
type target struct {
	flag    string
	label   string
	relPath []string // relative to home dir
	aliases []string // other flags that resolve to this target
}

var skillTargets = []target{
	{
		flag:    "claude-code",
		label:   "Claude Code",
		relPath: []string{".claude", "skills", "specsync"},
	},
	{
		flag:    "agents",
		label:   "Agent Skills",
		relPath: []string{".agents", "skills", "specsync"},
		aliases: []string{"codex", "opencode"},
	},
}

// aliasToFlag maps alias flags to their canonical target flag.
var aliasToFlag = func() map[string]string {
	m := make(map[string]string)
	for _, t := range skillTargets {
		for _, a := range t.aliases {
			m[a] = t.flag
		}
	}
	return m
}()

// driftStatus reports the relationship between installed and canonical content.
type driftStatus string

const (
	driftNone     driftStatus = "none"
	driftOutdated driftStatus = "outdated"
	driftDrifted  driftStatus = "drifted"
)

// checkDrift compares every installed SKILL.md against the canonical source
// and returns a map of target_flag -> drift status string ("none", "outdated",
// "drifted").  A target is skipped (not present in the map) when its
// destination directory does not exist.
func checkDrift(home string) map[string]string {
	result := make(map[string]string)

	for _, t := range skillTargets {
		dest := filepath.Join(append([]string{home}, t.relPath...)...)
		dest = filepath.Join(dest, "SKILL.md")
		status := checkSingleDrift(dest)
		if status == "" {
			continue // directory does not exist
		}
		result[t.flag] = string(status)
	}

	return result
}

// checkSingleDrift evaluates a single installed SKILL.md against the canonical
// content and returns the drift status string, or "" when the file does not
// exist.
func checkSingleDrift(installedPath string) driftStatus {
	installedContent, err := os.ReadFile(installedPath)
	if err != nil {
		return "" // file / directory absent
	}

	// Byte-for-byte comparison first: identical content means no drift.
	if bytes.Equal(installedContent, skillContent) {
		return driftNone
	}

	// Content differs — distinguish "outdated" from "drifted".
	installedVer := extractSkillVersion(installedContent)
	canonicalVer := extractSkillVersion(skillContent)

	if installedVer != "" && canonicalVer != "" {
		// Both have versions: if canonical is newer it is "outdated",
		// otherwise the user has local changes → "drifted".
		if versionCompare(canonicalVer, installedVer) {
			return driftOutdated
		}
	}

	return driftDrifted
}

// hasLocalOverride reports whether dest contains content that differs from the
// canonical skill content.  Returns false when the file does not exist.
func hasLocalOverride(dest string) bool {
	content, err := os.ReadFile(dest)
	if err != nil {
		return false
	}
	return !bytes.Equal(content, skillContent)
}

// tempDirFor creates a temporary top-level directory under the system temp
// directory and returns the skill path inside it (mirroring the target's
// relPath).  The caller is responsible for cleaning up the temp root.
func tempDirFor(target target) (string, func(), error) {
	root, err := os.MkdirTemp("", "specskill-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp root: %w", err)
	}
	cleanup := func() { os.RemoveAll(root) }
	skillPath := filepath.Join(append([]string{root}, target.relPath...)...)
	return skillPath, cleanup, nil
}

// runInstallSkill installs or manages the SpecSync skill file for one or more
// agent harnesses.
//
// Flags
//
//	--all            install to every known agent directory
//	--claude-code    install to Claude Code (~/.claude/skills/specsync)
//	--agents         install to Agent Skills (~/.agents/skills/specsync)
//	--codex          alias for --agents
//	--opencode       alias for --agents
//	--drift          report drift for every known agent directory (no install)
//	--temp           install to a temporary directory for testing (isolated)
//	--rollback       restore canonical content to all selected targets,
//	                 overwriting local overrides
//
// installSkillCmd installs and manages the embedded skill.
type installSkillCmd struct {
	all      *bool
	flags    map[string]*bool
	drift    *bool
	temp     *bool
	rollback *bool
}

func init() { Register(&installSkillCmd{}) }

func (*installSkillCmd) Spec() Spec {
	return Spec{
		Name:     "install-skill",
		Group:    GroupHousekeeping,
		Summary:  "Install the specsync skill for your coding agent.",
		Mutates:  true,
		Workflow: AgentCommandWorkflow{Position: "setup"},
		Safety: []string{
			"Writes SKILL.md under the agent's skills directory in your home directory",
		},
		Examples: []string{
			"specsync install-skill -claude-code",
			"specsync install-skill -all",
			"specsync install-skill -drift",
		},
	}
}

func (cm *installSkillCmd) Flags(fs *flag.FlagSet) {
	cm.all = fs.Bool("all", false, "install to every known agent directory")
	cm.flags = make(map[string]*bool)
	for _, t := range skillTargets {
		cm.flags[t.flag] = fs.Bool(t.flag, false, fmt.Sprintf("install to %s (~/%s)", t.label, filepath.Join(t.relPath...)))
		for _, a := range t.aliases {
			cm.flags[a] = fs.Bool(a, false, fmt.Sprintf("alias for -%s", t.flag))
		}
	}
	cm.drift = fs.Bool("drift", false, "report drift for every known agent directory (no install)")
	cm.temp = fs.Bool("temp", false, "install to a temporary directory for testing (isolated)")
	cm.rollback = fs.Bool("rollback", false, "restore canonical content to all selected targets, overwriting local overrides")
}

func (cm *installSkillCmd) Run(_ context.Context, args []string) error {
	fs := flagSetFor(cm)
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)
	cm.run()
	return nil
}

func (cm *installSkillCmd) run() {
	all, flags, drift, temp, rollback := cm.all, cm.flags, cm.drift, cm.temp, cm.rollback

	home, err := os.UserHomeDir()
	if err != nil {
		fail(fmt.Errorf("install-skill: cannot determine home directory: %w", err))
	}

	// --drift: report drift and exit.
	if *drift {
		statuses := checkDrift(home)
		if len(statuses) == 0 {
			fmt.Println("No installed skill files found.")
			return
		}

		// Print a header.
		fmt.Printf("%-15s %-10s\n", "TARGET", "STATUS")
		fmt.Println(strings.Repeat("-", 30))

		for _, t := range skillTargets {
			status, ok := statuses[t.flag]
			if !ok {
				fmt.Printf("%-15s (not installed)\n", t.label)
				continue
			}
			fmt.Printf("%-15s %s\n", t.label, status)
		}
		return
	}

	// Resolve selected targets, deduplicating aliases.
	selected := make(map[string]bool)
	for _, t := range skillTargets {
		if *all {
			selected[t.flag] = true
			continue
		}
		if *flags[t.flag] {
			selected[t.flag] = true
			continue
		}
		for _, a := range t.aliases {
			if *flags[a] {
				selected[t.flag] = true
			}
		}
	}

	// --rollback: restore canonical content to all selected targets,
	// overwriting local overrides.
	if *rollback {
		if !selected["claude-code"] && !selected["agents"] {
			fmt.Fprintln(os.Stderr, "specsync install-skill --rollback: specify --all or at least one platform flag")
			fmt.Fprintln(os.Stderr, "  --all            rollback every known agent directory")
			fmt.Fprintln(os.Stderr, "  --claude-code    rollback Claude Code")
			fmt.Fprintln(os.Stderr, "  --agents         rollback Agent Skills")
			fmt.Fprintln(os.Stderr, "  --codex          alias for --agents")
			fmt.Fprintln(os.Stderr, "  --opencode       alias for --agents")
			os.Exit(1)
		}

		fmt.Fprintln(os.Stderr, "Rolling back SpecSync skill to canonical revision...")

		canonicalVer := extractSkillVersion(skillContent)

		for _, t := range skillTargets {
			if !selected[t.flag] {
				continue
			}
			dir := filepath.Join(append([]string{home}, t.relPath...)...)
			dest := filepath.Join(dir, "SKILL.md")

			// Ensure the directory exists.
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
				continue
			}

			// Write canonical content (overriding any local changes).
			if err := os.WriteFile(dest, skillContent, 0o644); err != nil {
				fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
				continue
			}

			fmt.Printf("  rolled   %-20s %s\n", t.label, dest)
		}

		if canonicalVer != "" && canonicalVer != "dev" {
			fmt.Printf("\nRolled back to specsync skill version %s\n", canonicalVer)
		}
		return
	}

	// --temp: install to a temporary directory for testing.
	if *temp {
		if !selected["claude-code"] && !selected["agents"] {
			fmt.Fprintln(os.Stderr, "specsync install-skill --temp: specify --all or at least one platform flag")
			fmt.Fprintln(os.Stderr, "  --all            install to temp dirs for every platform")
			fmt.Fprintln(os.Stderr, "  --claude-code    install to temp dir for Claude Code")
			fmt.Fprintln(os.Stderr, "  --agents         install to temp dir for Agent Skills")
			fmt.Fprintln(os.Stderr, "  --codex          alias for --agents")
			fmt.Fprintln(os.Stderr, "  --opencode       alias for --agents")
			os.Exit(1)
		}

		fmt.Fprintln(os.Stderr, "Installing SpecSync skill to temporary directory...")

		canonicalVer := extractSkillVersion(skillContent)

		for _, t := range skillTargets {
			if !selected[t.flag] {
				continue
			}

			skillPath, cleanup, err := tempDirFor(t)
			if err != nil {
				fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
				cleanup()
				fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
				continue
			}

			if err := os.WriteFile(skillPath, skillContent, 0o644); err != nil {
				cleanup()
				fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
				continue
			}

			fmt.Printf("  wrote    %-20s %s\n", t.label, skillPath)
			fmt.Printf("             temp root: %s (not cleaned up)\n", filepath.Dir(filepath.Dir(skillPath)))

			// Store the cleanup function so we can report it;
			// we do NOT call cleanup here — the temp dir must persist
			// for the user to inspect.
			_ = cleanup
		}

		if canonicalVer != "" && canonicalVer != "dev" {
			fmt.Printf("\nInstalled specsync skill version %s (temp)\n", canonicalVer)
		}
		return
	}

	// Normal install path: skip destinations with local overrides.
	installedVersion := extractSkillVersion(skillContent)

	fmt.Fprintln(os.Stderr, "Installing SpecSync skill...")

	for _, t := range skillTargets {
		if !selected[t.flag] {
			continue
		}
		dir := filepath.Join(append([]string{home}, t.relPath...)...)
		dest := filepath.Join(dir, "SKILL.md")

		// Check for local overrides — skip if present.
		if hasLocalOverride(dest) {
			fmt.Printf("  skipped  %-20s (local override preserved)\n", t.label)
			continue
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
			continue
		}
		if err := os.WriteFile(dest, skillContent, 0o644); err != nil {
			fmt.Printf("  skipped  %-20s (%s)\n", t.label, err)
			continue
		}

		var extra string
		if t.label == "Agent Skills" {
			extra = "  Used by: Codex, OpenCode, and other Agent Skills hosts"
		}

		fmt.Printf("  wrote    %-20s %s\n", t.label, dest)
		if extra != "" {
			fmt.Printf("             %s\n", extra)
		}
	}

	if installedVersion != "" && installedVersion != "dev" {
		fmt.Printf("\nInstalled specsync skill version %s\n", installedVersion)
	}
}
