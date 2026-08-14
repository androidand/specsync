package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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

// installSkillCmd copies the embedded skill into agent skill directories.
type installSkillCmd struct {
	all   *bool
	flags map[string]*bool
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
}

func (cm *installSkillCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *installSkillCmd) run(args []string) {
	fs := flagSetFor(cm)
	all, flags := cm.all, cm.flags
	_ = fs.Parse(args)
	checkArgs(fs, args, 0)

	home, err := os.UserHomeDir()
	if err != nil {
		fail(fmt.Errorf("install-skill: cannot determine home directory: %w", err))
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

	if !selected["claude-code"] && !selected["agents"] {
		fmt.Fprintln(os.Stderr, "specsync install-skill: specify --all or at least one platform flag")
		fmt.Fprintln(os.Stderr, "  --all            install to every known agent directory")
		fmt.Fprintln(os.Stderr, "  --claude-code    install to Claude Code (~/.claude/skills/specsync)")
		fmt.Fprintln(os.Stderr, "  --agents         install to Agent Skills (~/.agents/skills/specsync)")
		fmt.Fprintln(os.Stderr, "  --codex          alias for --agents")
		fmt.Fprintln(os.Stderr, "  --opencode       alias for --agents")
		os.Exit(1)
	}

	installedVersion := extractSkillVersion(skillContent)

	fmt.Fprintln(os.Stderr, "Installing SpecSync skill...")

	for _, t := range skillTargets {
		if !selected[t.flag] {
			continue
		}
		dir := filepath.Join(append([]string{home}, t.relPath...)...)
		dest := filepath.Join(dir, "SKILL.md")
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
