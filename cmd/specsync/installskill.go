package main

import (
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
}

var skillTargets = []target{
	{"claude-code", "Claude Code", []string{".claude", "skills", "specsync"}},
	{"codex", "Codex / Skein", []string{".codex", "skills", "specsync"}},
	{"opencode", "OpenCode", []string{".config", "opencode", "skills", "specsync"}},
	{"copilot", "GitHub Copilot", []string{".copilot", "skills", "specsync"}},
	{"agents", "Generic (.agents)", []string{".agents", "skills", "specsync"}},
}

func runInstallSkill(args []string) {
	fs := flag.NewFlagSet("install-skill", flag.ExitOnError)
	all := fs.Bool("all", false, "install to every known agent directory")
	flags := make([]*bool, len(skillTargets))
	for i, t := range skillTargets {
		flags[i] = fs.Bool(t.flag, false, fmt.Sprintf("install to %s (~/%s)", t.label, filepath.Join(t.relPath...)))
	}
	_ = fs.Parse(args)

	home, err := os.UserHomeDir()
	if err != nil {
		fail(fmt.Errorf("install-skill: cannot determine home directory: %w", err))
	}

	selected := *all
	if !selected {
		for _, f := range flags {
			if *f {
				selected = true
				break
			}
		}
	}
	if !selected {
		fmt.Fprintln(os.Stderr, "specsync install-skill: specify --all or at least one platform flag")
		fmt.Fprintln(os.Stderr, "  --all            install to every known agent directory")
		for _, t := range skillTargets {
			fmt.Fprintf(os.Stderr, "  --%-16s install to %s\n", t.flag, t.label)
		}
		os.Exit(1)
	}

	for i, t := range skillTargets {
		if !*all && !*flags[i] {
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
		fmt.Printf("  wrote    %-20s %s\n", t.label, dest)
	}
}
