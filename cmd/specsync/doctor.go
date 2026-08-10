package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// DoctorResult represents diagnostic output.
type DoctorResult struct {
	Status          string                 `json:"status"`
	Message         string                 `json:"message,omitempty"`
	Installation    InstallationInfo       `json:"installation,omitempty"`
	TokenAnalysis   TokenAnalysisInfo      `json:"token_analysis,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
}

// InstallationInfo describes skill installation status.
type InstallationInfo struct {
	Primary         string `json:"primary"`
	Installed       bool   `json:"installed"`
	Version         string `json:"version,omitempty"`
	BinaryVersion   string `json:"binary_version,omitempty"`
	NeedsUpdate     bool   `json:"needs_update,omitempty"`
	SizeBytes       int64  `json:"size_bytes"`
	SizeKB          float64 `json:"size_kb"`
	Lines           int    `json:"lines"`
	Profile         string `json:"profile"`
}

// TokenAnalysisInfo describes token usage.
type TokenAnalysisInfo struct {
	DefaultLoaded int `json:"default_loaded"`
	EstimatedMin  int `json:"estimated_min"`
	EstimatedMax  int `json:"estimated_max"`
}

// runDoctor handles the doctor command.
func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON output")
	skipUpdate := fs.Bool("skip-skill-update", false, "skip auto-updating skill files")
	if err := fs.Parse(args); err != nil {
		fail(err)
	}

	// Auto-update skills unless --skip-skill-update is set
	if !*skipUpdate && version != "" && version != "dev" {
		if updateSkillsIfNeeded(version) {
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	remaining := fs.Args()
	subcommand := "default"
	if len(remaining) > 0 {
		subcommand = remaining[0]
	}

	switch subcommand {
	case "default", "claude":
		doctorClaude(*jsonFlag)
	case "install":
		doctorInstall(*jsonFlag)
	case "context":
		doctorContext(*jsonFlag)
	case "skill":
		doctorSkill(*jsonFlag)
	default:
		fmt.Fprintf(os.Stderr, "specsync doctor: unknown subcommand %q\n", subcommand)
		fmt.Fprintf(os.Stderr, "Available: claude, install, context, skill\n")
		os.Exit(2)
	}
}

// doctorClaude provides Claude Code specific diagnostics.
func doctorClaude(asJSON bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}

	skillPath := filepath.Join(home, ".claude", "skills", "specsync", "SKILL.md")
	info, err := os.Stat(skillPath)

	result := DoctorResult{
		Status: "ok",
	}

	if err != nil {
		result.Status = "warning"
		result.Message = "Skill not installed for Claude Code"
		result.Recommendations = []string{
			"Install with: specsync install-skill --claude-code",
		}
	} else {
		// Read skill version
		content, _ := os.ReadFile(skillPath)
		installedVersion := extractSkillVersion(content)

		result.Installation = InstallationInfo{
			Primary:       skillPath,
			Installed:     true,
			Version:       installedVersion,
			BinaryVersion: version,
			NeedsUpdate:   installedVersion != "" && versionCompare(version, installedVersion),
			SizeBytes:     info.Size(),
			SizeKB:        float64(info.Size()) / 1024,
		}

		// Token estimation (rough)
		tokens := estimateTokens(int(info.Size()))
		result.TokenAnalysis = TokenAnalysisInfo{
			DefaultLoaded: tokens,
			EstimatedMin:  (tokens * 6) / 10, // Rough 60% reduction target
			EstimatedMax:  tokens,
		}

		if tokens > 600 {
			result.Status = "warning"
			result.Message = "Skill is larger than optimal"
			result.Recommendations = []string{
				fmt.Sprintf("Current: ~%d tokens", tokens),
				"Consider installing with --profile minimal for 60% reduction",
				"Run: specsync install-skill --claude-code --profile minimal",
			}
		}
	}

	if asJSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Claude Code Skill Diagnostics\n\n")
		fmt.Printf("Status: %s\n", result.Status)
		if result.Message != "" {
			fmt.Printf("Message: %s\n", result.Message)
		}
		if result.Installation.Installed {
			fmt.Printf("Location: %s\n", result.Installation.Primary)
			fmt.Printf("Size: %.1f KB (~%d tokens)\n", result.Installation.SizeKB, result.TokenAnalysis.DefaultLoaded)
			if result.Installation.Version != "" {
				fmt.Printf("Installed Version: %s\n", result.Installation.Version)
			}
			if result.Installation.BinaryVersion != "" {
				fmt.Printf("Binary Version: %s\n", result.Installation.BinaryVersion)
			}
			if result.Installation.NeedsUpdate {
				fmt.Printf("Status: ⚠ Update available\n")
			}
		}
		if len(result.Recommendations) > 0 {
			fmt.Println("\nRecommendations:")
			for _, rec := range result.Recommendations {
				fmt.Printf("  - %s\n", rec)
			}
		}
	}
}

// doctorInstall provides installation location diagnostics.
func doctorInstall(asJSON bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}

	locations := []struct {
		name string
		path string
	}{
		{"Claude Code", filepath.Join(home, ".claude", "skills", "specsync")},
		{"Codex", filepath.Join(home, ".codex", "skills", "specsync")},
		{"OpenCode", filepath.Join(home, ".config", "opencode", "skills", "specsync")},
	}

	fmt.Printf("Installation Locations\n\n")
	for _, loc := range locations {
		skillFile := filepath.Join(loc.path, "SKILL.md")
		if info, err := os.Stat(skillFile); err == nil {
			content, _ := os.ReadFile(skillFile)
			installedVersion := extractSkillVersion(content)
			versionStr := ""
			if installedVersion != "" {
				versionStr = fmt.Sprintf(" (v%s)", installedVersion)
				if versionCompare(version, installedVersion) {
					versionStr += " ⚠ outdated"
				}
			}
			fmt.Printf("%s: %s (%.1f KB)%s\n", loc.name, "✓ Installed", float64(info.Size())/1024, versionStr)
		} else {
			fmt.Printf("%s: (not installed)\n", loc.name)
		}
	}
}

// doctorContext provides token analysis.
func doctorContext(asJSON bool) {
	fmt.Printf("SpecSync Token Impact Analysis\n\n")
	fmt.Printf("Profile Tokens:\n")
	fmt.Printf("  minimal:  ~280 tokens (60%% savings)\n")
	fmt.Printf("  docs:     ~450 tokens (36%% savings)\n")
	fmt.Printf("  full:     ~700 tokens (baseline)\n\n")
	fmt.Printf("Annual Impact (5x/week usage):\n")
	fmt.Printf("  minimal:  ~27,300 tokens/year\n")
	fmt.Printf("  docs:     ~46,800 tokens/year\n")
	fmt.Printf("  full:     ~182,000 tokens/year\n")
}

// doctorSkill provides skill file analysis.
func doctorSkill(asJSON bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}

	skillPath := filepath.Join(home, ".claude", "skills", "specsync", "SKILL.md")
	info, err := os.Stat(skillPath)

	fmt.Printf("Skill File Analysis\n\n")
	if err != nil {
		fmt.Printf("Status: Not installed\n")
		fmt.Printf("Location: %s\n", skillPath)
		fmt.Printf("\nInstall with: specsync install-skill --claude-code\n")
		return
	}

	fmt.Printf("Status: Installed\n")
	fmt.Printf("Location: %s\n", skillPath)
	fmt.Printf("Size: %.1f KB\n", float64(info.Size())/1024)
	fmt.Printf("Estimated Tokens: ~%d\n", estimateTokens(int(info.Size())))

	content, _ := os.ReadFile(skillPath)
	installedVersion := extractSkillVersion(content)
	if installedVersion != "" {
		fmt.Printf("Installed Version: %s\n", installedVersion)
		fmt.Printf("Binary Version: %s\n", version)
		if versionCompare(version, installedVersion) {
			fmt.Printf("Status: Update available (run: specsync install-skill --all)\n")
		} else {
			fmt.Printf("Status: Up to date\n")
		}
	}
}

// estimateTokens estimates tokens from file size.
// Rough heuristic: ~1 token per 4 bytes (varies by content)
func estimateTokens(sizeBytes int) int {
	return sizeBytes / 4
}
