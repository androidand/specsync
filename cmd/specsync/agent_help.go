package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// AgentCommandFlag represents a command-line flag.
type AgentCommandFlag struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description"`
}

// AgentCommandWorkflow describes when and how to use a command in the workflow.
type AgentCommandWorkflow struct {
	Position      string   `json:"position"`
	RelatedBefore []string `json:"related_before,omitempty"`
	RelatedAfter  []string `json:"related_after,omitempty"`
}

// AgentCommandHelp is the complete help metadata for a command.
type AgentCommandHelp struct {
	Command      string                `json:"command"`
	Description  string                `json:"description"`
	Mutates      bool                  `json:"mutates"`
	Workflow     AgentCommandWorkflow  `json:"workflow"`
	Flags        []AgentCommandFlag    `json:"flags"`
	SafetyRules  []string              `json:"safety_rules"`
	JSONOutput   map[string]interface{} `json:"json_output,omitempty"`
	Examples     []string              `json:"examples"`
}

// commandMetadata is the registry of all command help.
var commandMetadata = map[string]AgentCommandHelp{
	"sync": {
		Command:     "sync",
		Description: "Synchronize an OpenSpec change with GitHub Issues.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "after-implementation",
			RelatedBefore: []string{"scan", "pull", "changes"},
			RelatedAfter:  []string{"changes", "verify", "release-plan"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "change",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "Sync only this change; without it, syncs every change",
			},
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Preview the sync without writing to GitHub",
			},
			{
				Name:        "reconcile",
				Type:        "boolean",
				Required:    false,
				Default:     true,
				Description: "Merge task state from GitHub back into tasks.md",
			},
			{
				Name:        "close-completed",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Auto-close the issue when all tasks are checked",
			},
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "auto-detect",
				Description: "Target repo as owner/name (default: auto-detect from git remote)",
			},
		},
		SafetyRules: []string{
			"Always use -dry-run first to preview changes",
			"Always pass -change when one change is in scope (without it, syncs every change)",
			"Confirm git remote points to the right repo",
		},
		Examples: []string{
			"specsync -dry-run -change my-change",
			"specsync -change my-change",
			"specsync sync -json -change my-change",
		},
	},
	"pull": {
		Command:     "pull",
		Description: "Pull an issue into a local OpenSpec change.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "issue-first-start",
			RelatedBefore: []string{"changes", "scan"},
			RelatedAfter:  []string{"sync", "link"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "issue",
				Type:        "number",
				Required:    true,
				Description: "Issue number to pull (required)",
			},
			{
				Name:        "change",
				Type:        "string",
				Required:    false,
				Description: "Override auto-derived change slug",
			},
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Preview what would be written without touching disk",
			},
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "auto-detect",
				Description: "Target repo as owner/name",
			},
		},
		SafetyRules: []string{
			"Use -dry-run to preview before pulling",
			"Verify the issue number is correct",
		},
		Examples: []string{
			"specsync pull -issue 42 -dry-run",
			"specsync pull -issue 42",
			"specsync pull -issue 42 -json",
		},
	},
	"link": {
		Command:     "link",
		Description: "Cross-link two or more OpenSpec changes.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "plan",
			RelatedBefore: []string{"scan", "changes"},
			RelatedAfter:  []string{"sync"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Preview links without writing files",
			},
		},
		SafetyRules: []string{
			"At least 2 changes required",
			"Use -dry-run to preview",
			"Links are append-only (existing links not removed)",
		},
		Examples: []string{
			"specsync link my-change other-change -dry-run",
			"specsync link my-change other-change",
			"specsync link change-a change-b change-c",
		},
	},
	"scan": {
		Command:     "scan",
		Description: "Scan for existing work in a code area or topic.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "plan",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{"pull", "link"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"At least one path or topic word is required",
			"Flags must come before positional arguments",
		},
		Examples: []string{
			"specsync scan cmd/specsync/ 'label creation'",
			"specsync scan cmd/specsync/ -json",
			"specsync scan openspec/changes/ reconcile",
		},
	},
	"trace": {
		Command:     "trace",
		Description: "Trace dependencies and relationships of a change.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "debugging",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "change",
				Type:        "string",
				Required:    false,
				Description: "Trace only this change (default: all)",
			},
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync trace",
			"specsync trace -change my-change",
			"specsync trace -json",
		},
	},
	"changes": {
		Command:     "changes",
		Description: "List all OpenSpec changes with status.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "planning",
			RelatedBefore: []string{},
			RelatedAfter:  []string{"pull", "scan", "link"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "stage",
				Type:        "string",
				Required:    false,
				Description: "Filter by stage (backlog, active, blocked, in-review, complete, archived)",
			},
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync changes",
			"specsync changes --stage backlog",
			"specsync changes -json",
		},
	},
	"set-stage": {
		Command:     "set-stage",
		Description: "Set the workflow stage of a change.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "workflow",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{"sync"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "stage",
				Type:        "string",
				Required:    true,
				Description: "New stage (backlog, active, blocked, in-review, complete, archived, auto)",
			},
		},
		SafetyRules: []string{
			"Use 'auto' to let specsync derive stage from tasks",
		},
		Examples: []string{
			"specsync set-stage my-change active",
			"specsync set-stage my-change blocked",
			"specsync set-stage my-change complete",
		},
	},
	"release-plan": {
		Command:     "release-plan",
		Description: "Inspect release impact and generate release notes.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "release",
			RelatedBefore: []string{"changelog"},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
			{
				Name:        "since",
				Type:        "string",
				Required:    false,
				Description: "Git reference for start of range (e.g., v0.9.0)",
			},
			{
				Name:        "until",
				Type:        "string",
				Required:    false,
				Description: "Git reference for end of range (default: main)",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync release-plan",
			"specsync release-plan --since v0.9.0",
			"specsync release-plan -json",
		},
	},
	"changelog": {
		Command:     "changelog",
		Description: "Generate release notes from OpenSpec changes and commits.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "release",
			RelatedBefore: []string{"release-plan"},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "resolve-refs",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Resolve issue references in commits",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
			"Requires git history",
		},
		Examples: []string{
			"specsync changelog",
			"specsync changelog -resolve-refs",
		},
	},
	"pr-body": {
		Command:     "pr-body",
		Description: "Generate the correct PR body fragment for a change.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "implementation",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "change",
				Type:        "string",
				Required:    true,
				Description: "Change slug",
			},
		},
		SafetyRules: []string{
			"Always include the output in your PR body",
			"Part of or Closes depends on task completion",
		},
		Examples: []string{
			"specsync pr-body -change my-change",
		},
	},
	"verify": {
		Command:     "verify",
		Description: "Verify PR references to their change issues.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "pre-merge",
			RelatedBefore: []string{"pr-body"},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{},
		SafetyRules: []string{
			"Run before merging a PR",
			"All open PRs should have issue references",
		},
		Examples: []string{
			"specsync verify",
		},
	},
	"audit": {
		Command:     "audit",
		Description: "Audit OpenSpec changes for structural issues.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "validation",
			RelatedBefore: []string{},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync audit",
			"specsync audit -json",
		},
	},
	"audit-tasks": {
		Command:     "audit-tasks",
		Description: "Audit that task status matches code changes (dogfooding).",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "validation",
			RelatedBefore: []string{},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Enforced by CI",
			"Keeps this repo's dogfooding honest",
		},
		Examples: []string{
			"specsync audit-tasks",
			"specsync audit-tasks -json",
		},
	},
	"validate": {
		Command:     "validate",
		Description: "Validate OpenSpec change structure.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "validation",
			RelatedBefore: []string{},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
			"Run before syncing",
		},
		Examples: []string{
			"specsync validate",
			"specsync validate -json",
		},
	},
	"spinoff": {
		Command:     "spinoff",
		Description: "Spin off emergent work from a discovery.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "implementation",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{"sync", "link"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "from",
				Type:        "string",
				Required:    true,
				Description: "Parent change slug",
			},
			{
				Name:        "task",
				Type:        "number",
				Required:    false,
				Description: "Task number to extract and mark as moved",
			},
			{
				Name:        "text",
				Type:        "string",
				Required:    false,
				Description: "Free-form discovery text",
			},
			{
				Name:        "kind",
				Type:        "string",
				Required:    false,
				Description: "Label kind (bug, followup, task)",
			},
		},
		SafetyRules: []string{
			"Either -task or -text required",
			"Creates a new change locally",
			"Link the changes with specsync link after",
		},
		Examples: []string{
			"specsync spinoff -from my-change -task 3",
			"specsync spinoff -from my-change -text 'Found edge case in X'",
		},
	},
	"install-skill": {
		Command:     "install-skill",
		Description: "Install the SpecSync Claude Code skill.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "setup",
			RelatedBefore: []string{},
			RelatedAfter:  []string{},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "claude-code",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Install for Claude Code",
			},
			{
				Name:        "all",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Install for all supported agents",
			},
			{
				Name:        "profile",
				Type:        "string",
				Required:    false,
				Default:     "minimal",
				Description: "Installation profile (minimal, docs, full)",
			},
		},
		SafetyRules: []string{
			"Requires write access to ~/.claude/ or similar",
		},
		Examples: []string{
			"specsync install-skill --claude-code",
			"specsync install-skill --all --profile minimal",
			"specsync install-skill --claude-code --profile docs",
		},
	},
	"doctor": {
		Command:     "doctor",
		Description: "Diagnose skill installation, environment, and context health.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "anytime",
			RelatedBefore: []string{},
			RelatedAfter:  []string{"install-skill"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
			{
				Name:        "skip-skill-update",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Skip auto-updating skill files before diagnosing",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync doctor",
			"specsync doctor install -json",
			"specsync doctor context -json",
		},
	},
	"epic": {
		Command:     "epic",
		Description: "Create a coordination issue and wire children to it.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "plan",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{"sync"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "auto-detect",
				Description: "Target repo for the epic issue, as owner/name",
			},
			{
				Name:        "child",
				Type:        "string",
				Required:    false,
				Description: "A child to attach: local change slug, owner/repo#N, bare #N, or issue URL (repeatable)",
			},
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Print what would happen without creating or editing any issue",
			},
		},
		SafetyRules: []string{
			"Use -dry-run to preview before creating or editing issues",
			"Re-running with the same title converges onto the existing epic instead of duplicating it",
		},
		Examples: []string{
			"specsync epic 'Q3 auth rework' -child my-change -child other-org/other-repo#42 -dry-run",
			"specsync epic 'Q3 auth rework' -child my-change",
		},
	},
	"idea": {
		Command:     "idea",
		Description: "Capture a free-text idea as a stage:intake issue.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "capture",
			RelatedBefore: []string{},
			RelatedAfter:  []string{"ideas", "pull"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "config ideas_repo, $SPECSYNC_IDEAS_REPO, or current repo",
				Description: "Repo to create the idea issue in, as owner/name",
			},
		},
		SafetyRules: []string{
			"Creates a real GitHub issue — there is no -dry-run",
		},
		Examples: []string{
			"specsync idea 'Consider caching the trace graph'",
			"echo 'Consider caching the trace graph' | specsync idea",
		},
	},
	"ideas": {
		Command:     "ideas",
		Description: "List open stage:intake issues for the repo.",
		Mutates:     false,
		Workflow: AgentCommandWorkflow{
			Position:      "review",
			RelatedBefore: []string{},
			RelatedAfter:  []string{"pull"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "config ideas_repo, $SPECSYNC_IDEAS_REPO, or current repo",
				Description: "Repo to list idea issues from, as owner/name",
			},
			{
				Name:        "json",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Emit machine-readable JSON output",
			},
		},
		SafetyRules: []string{
			"Read-only operation",
		},
		Examples: []string{
			"specsync ideas",
			"specsync ideas -json",
		},
	},
	"archive": {
		Command:     "archive",
		Description: "Archive a completed change: final push, close, then retention.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "after-complete",
			RelatedBefore: []string{"changes", "sync"},
			RelatedAfter:  []string{"verify"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "change",
				Type:        "string",
				Required:    true,
				Description: "Change to archive",
			},
			{
				Name:        "repo",
				Type:        "string",
				Required:    false,
				Default:     "auto-detect",
				Description: "Target repo as owner/name",
			},
			{
				Name:        "retain",
				Type:        "string",
				Required:    false,
				Description: "Retention policy: move (keep) or prune (delete)",
			},
			{
				Name:        "force",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Archive even when tasks are unchecked",
			},
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Print the plan without making changes",
			},
		},
		SafetyRules: []string{
			"Use -dry-run to preview before archiving",
			"Refuses unchecked-task changes unless -force is set",
		},
		Examples: []string{
			"specsync archive -change my-change -dry-run",
			"specsync archive -change my-change -retain move",
		},
	},
	"set-priority": {
		Command:     "set-priority",
		Description: "Set or unset a change's numeric priority override.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "plan",
			RelatedBefore: []string{"changes"},
			RelatedAfter:  []string{"changes"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "openspec",
				Type:        "string",
				Required:    false,
				Default:     "openspec",
				Description: "Path to the openspec/ directory",
			},
		},
		SafetyRules: []string{
			"Takes positional args: <change> <1-100|unset>, not flags",
		},
		Examples: []string{
			"specsync set-priority my-change 80",
			"specsync set-priority my-change unset",
		},
	},
	"note": {
		Command:     "note",
		Description: "Append a discovery line to a change's discoveries.md.",
		Mutates:     true,
		Workflow: AgentCommandWorkflow{
			Position:      "implementation",
			RelatedBefore: []string{},
			RelatedAfter:  []string{"spinoff"},
		},
		Flags: []AgentCommandFlag{
			{
				Name:        "openspec",
				Type:        "string",
				Required:    false,
				Default:     "openspec",
				Description: "Path to the openspec/ directory",
			},
			{
				Name:        "dry-run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Show what would be written without modifying files",
			},
		},
		SafetyRules: []string{
			"Takes positional args: <change> <text>, not a -text flag",
		},
		Examples: []string{
			"specsync note my-change 'Found edge case in X' -dry-run",
			"specsync note my-change 'Found edge case in X'",
		},
	},
}

// runAgentHelp handles the agent-help command.
func runAgentHelp(args []string) {
	fs := flag.NewFlagSet("agent-help", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "emit machine-readable JSON output")
	if err := fs.Parse(reorderFlagsFirst(args)); err != nil {
		fail(err)
	}

	remaining := fs.Args()
	var command string
	if len(remaining) > 0 {
		command = remaining[0]
	}

	if command == "" {
		// Show high-level guidance
		renderAgentHelpOverview(*jsonFlag)
		return
	}

	help, ok := commandMetadata[command]
	if !ok {
		fmt.Fprintf(os.Stderr, "specsync agent-help: unknown command %q\n", command)
		fmt.Fprintf(os.Stderr, "Available commands: %s\n", strings.Join(availableCommands(), ", "))
		os.Exit(2)
	}

	if *jsonFlag {
		data, err := json.MarshalIndent(help, "", "  ")
		if err != nil {
			fail(err)
		}
		fmt.Println(string(data))
	} else {
		renderAgentHelpMarkdown(help)
	}
}

// availableCommands returns all available command names.
func availableCommands() []string {
	commands := make([]string, 0, len(commandMetadata))
	for cmd := range commandMetadata {
		commands = append(commands, cmd)
	}
	// Sort for deterministic output
	for i := 0; i < len(commands)-1; i++ {
		for j := i + 1; j < len(commands); j++ {
			if commands[i] > commands[j] {
				commands[i], commands[j] = commands[j], commands[i]
			}
		}
	}
	return commands
}

// renderAgentHelpOverview renders high-level guidance.
func renderAgentHelpOverview(asJSON bool) {
	if asJSON {
		overview := map[string]interface{}{
			"description": "SpecSync CLI for managing OpenSpec changes and GitHub synchronization",
			"subcommands": len(commandMetadata),
			"commands":    availableCommands(),
			"tips": []string{
				"Use `specsync agent-help <command>` for detailed help on any command",
				"Use `specsync agent-help <command> -json` for machine-readable output",
				"All output formats support -json for automation",
				"Always use -dry-run before making changes",
			},
		}
		data, _ := json.MarshalIndent(overview, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println(`# SpecSync Commands

SpecSync manages OpenSpec changes and keeps them in sync with GitHub Issues.

## Commands

For help on any command, use:

  specsync agent-help <command>
  specsync agent-help <command> -json       # for JSON output

Available commands:`)
		for _, cmd := range availableCommands() {
			if help, ok := commandMetadata[cmd]; ok {
				fmt.Printf("  %-20s %s\n", cmd, help.Description)
			}
		}

		fmt.Print(`
## Tips

- Always use -dry-run before making changes
- Many commands support -json for machine-readable output
- See agent-help <command> for full details and flags
- Read AGENTS.md for workflow patterns
`)
	}
}

// renderAgentHelpMarkdown renders human-readable help.
func renderAgentHelpMarkdown(help AgentCommandHelp) {
	fmt.Printf("# %s\n\n", help.Command)
	fmt.Printf("%s\n\n", help.Description)

	fmt.Print("## Behavior\n\n")
	if help.Mutates {
		fmt.Println("Mutations: Yes (use -dry-run to preview)")
	} else {
		fmt.Println("Mutations: No (read-only)")
	}
	fmt.Printf("Workflow position: %s\n", help.Workflow.Position)
	if len(help.Workflow.RelatedBefore) > 0 {
		fmt.Printf("Related (before): %s\n", strings.Join(help.Workflow.RelatedBefore, ", "))
	}
	if len(help.Workflow.RelatedAfter) > 0 {
		fmt.Printf("Related (after): %s\n", strings.Join(help.Workflow.RelatedAfter, ", "))
	}

	if len(help.Flags) > 0 {
		fmt.Print("\n## Flags\n\n")
		for _, flag := range help.Flags {
			req := ""
			if flag.Required {
				req = " (required)"
			}
			def := ""
			if flag.Default != nil && flag.Default != "" && flag.Default != false {
				def = fmt.Sprintf(" (default: %v)", flag.Default)
			}
			fmt.Printf("**-%s** (%s)%s%s\n", flag.Name, flag.Type, req, def)
			fmt.Printf("  %s\n\n", flag.Description)
		}
	}

	if len(help.SafetyRules) > 0 {
		fmt.Print("## Safety Rules\n\n")
		for _, rule := range help.SafetyRules {
			fmt.Printf("- %s\n", rule)
		}
		fmt.Println()
	}

	if len(help.Examples) > 0 {
		fmt.Print("## Examples\n\n")
		for _, example := range help.Examples {
			fmt.Printf("```\n%s\n```\n\n", example)
		}
	}
}
