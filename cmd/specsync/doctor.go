package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DoctorResult represents diagnostic output.
type DoctorResult struct {
	Status          string            `json:"status"`
	Message         string            `json:"message,omitempty"`
	Installation    InstallationInfo  `json:"installation,omitempty"`
	TokenAnalysis   TokenAnalysisInfo `json:"token_analysis,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
	Dependencies    []DependencyInfo  `json:"dependencies,omitempty"`
}

// DependencyInfo describes a runtime CLI dependency specsync shells out to
// (starting with `openspec`). Found is false, with Path/Version empty, when
// the binary isn't reachable on $PATH. When found but its version can't be
// determined (command failed or output unparseable), Found stays true and
// Version stays empty — a diagnostic degrades gracefully rather than failing.
type DependencyInfo struct {
	Name    string `json:"name"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

// openspecVersionRE matches the bare version string `openspec --version`
// prints today (e.g. "1.5.0", no "v" prefix, no surrounding text). Anything
// else is treated as unparseable rather than assumed to be a valid version.
var openspecVersionRE = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){0,3}$`)

// checkOpenspecBinary probes for the `openspec` binary on $PATH and,
// best-effort, its reported version. It never returns an error: "not found"
// and "version unparseable" are both reportable diagnostic states, not
// failures the caller must handle.
func checkOpenspecBinary() DependencyInfo {
	info := DependencyInfo{Name: "openspec"}

	path, err := exec.LookPath("openspec")
	if err != nil {
		return info
	}
	info.Found = true
	info.Path = path

	out, err := exec.Command("openspec", "--version").Output()
	if err != nil {
		return info
	}
	v := strings.TrimSpace(string(out))
	if openspecVersionRE.MatchString(v) {
		info.Version = v
	}
	return info
}

// InstallationInfo describes skill installation status.
type InstallationInfo struct {
	Primary       string  `json:"primary"`
	Installed     bool    `json:"installed"`
	Version       string  `json:"version,omitempty"`
	BinaryVersion string  `json:"binary_version,omitempty"`
	NeedsUpdate   bool    `json:"needs_update,omitempty"`
	SizeBytes     int64   `json:"size_bytes"`
	SizeKB        float64 `json:"size_kb"`
	Lines         int     `json:"lines"`
	Profile       string  `json:"profile"`
}

// TokenAnalysisInfo describes token usage.
type TokenAnalysisInfo struct {
	DefaultLoaded int `json:"default_loaded"`
	EstimatedMin  int `json:"estimated_min"`
	EstimatedMax  int `json:"estimated_max"`
}

// installLocation describes a skill installation location.
type installLocation struct {
	Name     string
	Path     string
	IsLegacy bool
}

// installInfoJSON is the JSON representation of an install location.
type installInfoJSON struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Status    string `json:"status"`
}

// repoInfoJSON is the JSON representation of repository integration.
type repoInfoJSON struct {
	AgentMD              bool `json:"agents_md"`
	ClaudeMD             bool `json:"claude_md"`
	ClaudeMDImportsAgent bool `json:"claude_md_imports_agents"`
	ClaudeMDLinksAgent   bool `json:"claude_md_links_agents"`
}

// installDoctorJSON is the JSON output for doctor install.
type installDoctorJSON struct {
	Status       string            `json:"status"`
	Message      string            `json:"message,omitempty"`
	Canonical    []installInfoJSON `json:"canonical,omitempty"`
	Legacy       []installInfoJSON `json:"legacy,omitempty"`
	Repo         repoInfoJSON      `json:"repository,omitempty"`
	Dependencies []DependencyInfo  `json:"dependencies"`
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
				"Reinstall to pick up the current canonical skill (there is only one profile; no --profile flag exists): specsync install-skill --claude-code",
				"Detailed reference material belongs in `specsync agent-help`, not the skill file itself — if this file has grown beyond the router content, that's worth trimming upstream",
			}
		}
	}

	dep := checkOpenspecBinary()
	result.Dependencies = []DependencyInfo{dep}
	if !dep.Found {
		if result.Status == "ok" {
			result.Status = "warning"
		}
		result.Recommendations = append(result.Recommendations, "Install with: openspec init")
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
				fmt.Printf("Status: Update available\n")
			}
		}
		fmt.Println("\nDependencies:")
		printDependency(dep)
		if len(result.Recommendations) > 0 {
			fmt.Println("\nRecommendations:")
			for _, rec := range result.Recommendations {
				fmt.Printf("  - %s\n", rec)
			}
		}
	}
}

// SkillLocationInfo describes one agent's installed-skill location.
type SkillLocationInfo struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Installed   bool    `json:"installed"`
	Version     string  `json:"version,omitempty"`
	SizeKB      float64 `json:"size_kb,omitempty"`
	NeedsUpdate bool    `json:"needs_update,omitempty"`
}

// InstallResult is the machine-readable output of `doctor install --json`.
type InstallResult struct {
	Locations    []SkillLocationInfo `json:"locations"`
	Dependencies []DependencyInfo    `json:"dependencies"`
}

// doctorInstall provides installation location diagnostics.
func doctorInstall(asJSON bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		fail(err)
	}

	repoRoot, repoErr := os.Getwd()
	if repoErr != nil {
		repoRoot = home
	}

	// Canonical locations
	canonicals := []installLocation{
		{"Claude Code", filepath.Join(home, ".claude", "skills", "specsync", "SKILL.md"), false},
		{"Agent Skills", filepath.Join(home, ".agents", "skills", "specsync", "SKILL.md"), false},
	}

	// Legacy locations
	legacy := []installLocation{
		{"Codex (legacy)", filepath.Join(home, ".codex", "skills", "specsync", "SKILL.md"), true},
		{"OpenCode (legacy)", filepath.Join(home, ".config", "opencode", "skills", "specsync", "SKILL.md"), true},
		{"Copilot (unverified)", filepath.Join(home, ".copilot", "skills", "specsync", "SKILL.md"), true},
	}

	dep := checkOpenspecBinary()

	// CLAUDE.md validation
	agentMDPath := filepath.Join(repoRoot, "AGENTS.md")
	claudeMDPath := filepath.Join(repoRoot, "CLAUDE.md")
	agentMDExists := false
	claudeMDPathExists := false
	claudeMDImportsAgentMD := false
	claudeMDLinksAgentMD := false

	if _, err := os.Stat(agentMDPath); err == nil {
		agentMDExists = true
	}
	if _, err := os.Stat(claudeMDPath); err == nil {
		claudeMDPathExists = true
		content, _ := os.ReadFile(claudeMDPath)
		claudeMDImportsAgentMD = bytes.Contains(content, []byte("@AGENTS.md"))
		claudeMDLinksAgentMD = bytes.Contains(content, []byte("[AGENTS.md](AGENTS.md)"))
	}

	if asJSON {
		j := installDoctorJSON{Status: "ok", Dependencies: []DependencyInfo{dep}}

		for _, loc := range canonicals {
			info := installInfoJSON{Name: loc.Name, Path: loc.Path}
			if _, err := os.Stat(loc.Path); err == nil {
				info.Installed = true
				content, _ := os.ReadFile(loc.Path)
				info.Version = extractSkillVersion(content)
				info.Status = "ok"
			} else {
				info.Status = "missing"
				if loc.Name == "Agent Skills" {
					j.Status = "warning"
				}
			}
			j.Canonical = append(j.Canonical, info)
		}

		for _, loc := range legacy {
			info := installInfoJSON{Name: loc.Name, Path: loc.Path}
			if _, err := os.Stat(loc.Path); err == nil {
				info.Installed = true
				info.Status = "legacy"
				j.Status = "warning"
			} else {
				info.Installed = false
				info.Status = "not found"
			}
			j.Legacy = append(j.Legacy, info)
		}

		j.Repo = repoInfoJSON{
			AgentMD:              agentMDExists,
			ClaudeMD:             claudeMDPathExists,
			ClaudeMDImportsAgent: claudeMDImportsAgentMD,
			ClaudeMDLinksAgent:   claudeMDLinksAgentMD,
		}

		if claudeMDPathExists && !claudeMDImportsAgentMD {
			j.Status = "warning"
			if claudeMDLinksAgentMD {
				j.Message = "CLAUDE.md links to AGENTS.md instead of importing it"
			} else if !agentMDExists {
				j.Message = "CLAUDE.md exists but does not reference AGENTS.md"
			}
		}

		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Installation Status")

	fmt.Println("Canonical installations")
	for _, loc := range canonicals {
		if _, err := os.Stat(loc.Path); err == nil {
			content, _ := os.ReadFile(loc.Path)
			ver := extractSkillVersion(content)
			fmt.Printf("  %s\n", loc.Name)
			fmt.Printf("    %s\n", loc.Path)
			if ver != "" {
				fmt.Printf("    Version: %s\n", ver)
			}
			fmt.Printf("    Status: ✓\n")
		} else {
			fmt.Printf("  %s\n", loc.Name)
			fmt.Printf("    %s\n", loc.Path)
			fmt.Printf("    Status: not installed\n")
			if loc.Name == "Agent Skills" {
				fmt.Printf("    Run: specsync install-skill --agents\n")
			}
		}
	}

	fmt.Println()
	fmt.Println("CLAUDE.md")
	if claudeMDPathExists {
		fmt.Printf("  %s\n", claudeMDPath)
		if claudeMDImportsAgentMD {
			fmt.Printf("  Imports AGENTS.md: yes ✓\n")
		} else if claudeMDLinksAgentMD {
			fmt.Printf("  Imports AGENTS.md: no (links instead of importing)\n")
			fmt.Printf("  Status: ⚠ Use @AGENTS.md instead of [AGENTS.md](AGENTS.md)\n")
		} else {
			fmt.Printf("  Imports AGENTS.md: no\n")
			fmt.Printf("  Status: ⚠ Should import AGENTS.md\n")
		}
	} else {
		fmt.Printf("  %s\n", claudeMDPath)
		fmt.Printf("  Status: not found (optional)\n")
	}

	fmt.Println()
	fmt.Println("Legacy installations")
	hasLegacy := false
	for _, loc := range legacy {
		if _, err := os.Stat(loc.Path); err == nil {
			fmt.Printf("  %s — exists (not used by current agents)\n", loc.Path)
			hasLegacy = true
		}
	}
	if !hasLegacy {
		fmt.Println("  None detected")
	}

	fmt.Printf("\nExternal Dependencies\n\n")
	printDependency(dep)
}

// printDependency prints one human-readable dependency-check line.
func printDependency(dep DependencyInfo) {
	if dep.Found {
		versionStr := ""
		if dep.Version != "" {
			versionStr = fmt.Sprintf(" v%s", dep.Version)
		}
		fmt.Printf("%s: ✓ Found (%s)%s\n", dep.Name, dep.Path, versionStr)
	} else {
		fmt.Printf("%s: not found — install with: openspec init\n", dep.Name)
	}
}

// doctorContext provides token analysis.
func doctorContext(asJSON bool) {
	fmt.Printf("SpecSync Token Impact Analysis\n\n")
	fmt.Printf("Profile Tokens:\n")
	fmt.Printf("  default:  ~280 tokens (router only)\n\n")
	fmt.Printf("Detailed documentation is available on-demand via\n")
	fmt.Printf("`specsync agent-help` rather than loaded by default.\n")
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
