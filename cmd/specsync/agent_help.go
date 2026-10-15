package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
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

// AgentCommandArg is a positional argument.
type AgentCommandArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// AgentCommandWorkflow describes when and how to use a command in the workflow.
type AgentCommandWorkflow struct {
	Position      string   `json:"position"`
	RelatedBefore []string `json:"related_before,omitempty"`
	RelatedAfter  []string `json:"related_after,omitempty"`
}

// AgentCommandHelp is the complete help metadata for a command.
type AgentCommandHelp struct {
	Command     string                 `json:"command"`
	Description string                 `json:"description"`
	Mutates     bool                   `json:"mutates"`
	Args        []AgentCommandArg      `json:"args,omitempty"`
	Workflow    AgentCommandWorkflow   `json:"workflow"`
	Flags       []AgentCommandFlag     `json:"flags"`
	SafetyRules []string               `json:"safety_rules"`
	JSONOutput  map[string]interface{} `json:"json_output,omitempty"`
	Examples    []string               `json:"examples"`
}

// agentHelpCmd prints documentation for every command, derived from the
// command declarations.
type agentHelpCmd struct{ jsonFlag *bool }

func init() { Register(&agentHelpCmd{}) }

func (*agentHelpCmd) Spec() Spec {
	return Spec{
		Name:     "agent-help",
		Group:    GroupHousekeeping,
		Summary:  "Show how to use any specsync command, for people and agents.",
		Workflow: AgentCommandWorkflow{Position: "any"},
		Safety:   []string{"Read-only"},
		Examples: []string{
			"specsync agent-help",
			"specsync agent-help sync",
			"specsync agent-help sync -json",
		},
	}
}

func (cm *agentHelpCmd) Flags(fs *flag.FlagSet) {
	cm.jsonFlag = fs.Bool("json", false, "emit machine-readable JSON output")
}

func (cm *agentHelpCmd) Run(_ context.Context, args []string) error {
	cm.run(args)
	return nil
}

func (cm *agentHelpCmd) run(args []string) {
	fs := flagSetFor(cm)
	jsonFlag := cm.jsonFlag
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

	help, ok := helpFor(command)
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
	commands := make([]string, 0, len(registry))
	for cmd := range registry {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	return commands
}

// helpFor returns the help entry for a command, derived from its declaration.
func helpFor(name string) (AgentCommandHelp, bool) {
	c, ok := lookupCommand(name)
	if !ok {
		return AgentCommandHelp{}, false
	}
	return helpFromCommand(c), true
}

// renderAgentHelpOverview renders high-level guidance.
func renderAgentHelpOverview(asJSON bool) {
	if asJSON {
		overview := map[string]interface{}{
			"description": "SpecSync CLI for managing OpenSpec changes and GitHub synchronization",
			"subcommands": len(availableCommands()),
			"commands":    availableCommands(),
			"tips": []string{
				"Use `specsync agent-help <command>` for detailed help on any command",
				"Use `specsync agent-help <command> -json` for machine-readable output",
				"All output formats support -json for automation",
				"Always use -dry-run before making changes",
				"One issue, one branch, one worktree: before starting work on a change, create a branch named after its issue (e.g. feat/<n>-<slug>) — a dedicated worktree (`git worktree add ../worktrees/<slug> -b feat/<n>-<slug>`) keeps concurrent changes from colliding in the same working directory. `specsync pull -issue <n> -worktree` does this for you on the issue-first path.",
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
			if help, ok := helpFor(cmd); ok {
				fmt.Printf("  %-20s %s\n", cmd, help.Description)
			}
		}

		fmt.Print(`
## Tips

- Always use -dry-run before making changes
- Many commands support -json for machine-readable output
- See agent-help <command> for full details and flags
- If the project has an AGENTS.md, read it for local workflow patterns

## Branch & worktree per issue

Before starting work on a change, create a branch named after its issue
(e.g. feat/42-my-change) — do not commit directly to the main branch or mix
two issues' work in one branch. A dedicated worktree keeps concurrent
changes from colliding in the same working directory:

  git worktree add ../worktrees/my-change -b feat/42-my-change

On the issue-first path, 'specsync pull -issue 42 -worktree' does this for
you (see 'specsync agent-help pull'). Clean up when merged:
'git worktree remove ../worktrees/my-change'.
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
