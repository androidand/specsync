# Add `specsync agent-help` command for CLI-generated guidance

## Context

The current SpecSync skill embeds detailed documentation (222 lines) to teach Claude how to use each command. This documentation is static, manually maintained, and duplicated with AGENTS.md and WORKFLOW.md.

Better approach: Let the CLI be self-documenting. Agents can query the CLI for:

- What a command does
- Whether it mutates state
- When to use it in a workflow
- What safety rules apply
- What JSON output looks like
- Related commands
- Example usage

This eliminates the need to teach the full manual to Claude—agents can ask for what they need.

## Proposed Changes

### New Subcommand: `specsync agent-help`

#### Basic Usage

```bash
# Get help for a specific command
specsync agent-help sync
specsync agent-help pull
specsync agent-help scan

# Get high-level guidance
specsync agent-help

# Machine-readable format
specsync agent-help sync --json
```

#### Output Format (Human-Readable)

```
# sync

Synchronize an OpenSpec change with GitHub Issues.

## Behavior

Reads the local change from openspec/changes/<slug>/ and projects it to GitHub
Issues. Updates the issue title, body, labels, and checklist based on the spec.
Optionally reconciles task state back from the issue into tasks.md.

Mutations: Yes (by default; use -dry-run to preview)
Workflow position: After local spec work; before final completion

## Usage

specsync [-dry-run] [-change <slug>]

## Flags

-change <slug>
  (required) Sync only this change; without it, syncs every change
-dry-run
  Preview the sync without writing to GitHub
-reconcile (default true)
  Merge task state from GitHub back into tasks.md
-close-completed (default false)
  Auto-close the issue when all tasks are checked

## Safety

- Always use -dry-run first
- Always pass -change when one change is in scope
- Confirm git remote points to the right repo

## JSON Output

With --json, emits:

```json
{
  "change": "my-change",
  "issue": 42,
  "created": false,
  "updated": true,
  "url": "https://github.com/owner/repo/issues/42"
}
```

## Related Commands

- specsync pull — issue → spec (inverse)
- specsync changes — list all changes with status
- specsync verify — check PR references

## When To Use

Use after implementing a change and updating tasks.md locally. Always dry-run
first to preview the GitHub mutation. Use after completing work to update the
issue with final status.
```

#### JSON Output Format

```json
{
  "command": "sync",
  "description": "Synchronize an OpenSpec change with GitHub Issues",
  "mutates": true,
  "workflow": {
    "position": "after-implementation",
    "related_before": ["scan", "pull"],
    "related_after": ["changes", "verify"]
  },
  "flags": [
    {
      "name": "change",
      "type": "string",
      "required": true,
      "description": "Sync only this change; without it, syncs every change"
    },
    {
      "name": "dry-run",
      "type": "boolean",
      "required": false,
      "default": false,
      "description": "Preview the sync without writing to GitHub"
    },
    {
      "name": "reconcile",
      "type": "boolean",
      "required": false,
      "default": true,
      "description": "Merge task state from GitHub back into tasks.md"
    }
  ],
  "safety_rules": [
    "Always use -dry-run first",
    "Always pass -change when one change is in scope",
    "Confirm git remote points to the right repo"
  ],
  "json_output": {
    "enabled": true,
    "schema": {
      "change": "string",
      "issue": "number",
      "created": "boolean",
      "updated": "boolean",
      "url": "string"
    }
  },
  "examples": [
    "specsync -dry-run -change my-change",
    "specsync -change my-change",
    "specsync sync --json"
  ]
}
```

### Implementation Details

#### CLI Architecture

Add new Go file: `cmd/specsync/agent_help.go`

```go
func runAgentHelp(args []string) {
  // Parse args and --json flag
  // Look up command metadata from registry
  // Render as markdown or JSON
}
```

#### Command Metadata

Create a registry mapping commands to metadata:

```go
var commandMetadata = map[string]AgentCommandHelp{
  "sync": {
    Description: "...",
    Mutates: true,
    ...
  },
  ...
}
```

Define `AgentCommandHelp` struct with:
- Description (string)
- Mutates (bool)
- Workflow information
- Flag definitions
- Safety rules
- JSON output schema
- Related commands
- Examples

#### Coverage

Implement metadata for all 20 subcommands:

- sync, pull, link, scan, trace
- release-plan, changelog
- changes, set-stage, set-priority, note, spinoff
- pr-body, verify, relate, work-graph
- audit, audit-tasks, validate
- install-skill

### Integration with Skill

The reduced skill (see `reduce-claude-skill-token-cost`) references this command:

```markdown
For detailed guidance on any command, run:

  specsync agent-help <command>
  specsync agent-help <command> --json  # for machine-readable format

Examples:
  specsync agent-help sync
  specsync agent-help pull
  specsync agent-help scan
```

### Benefits

1. **Single source of truth**: Command metadata lives in the CLI, not duplicated in skill
2. **Always current**: CLI help auto-updates with new features
3. **Machine-readable**: Agents can parse `--json` output for intelligent dispatch
4. **Progressive disclosure**: Agents ask for details only when needed
5. **Better UX**: Users can discover features without reading massive manuals

## Related Changes

- `reduce-claude-skill-token-cost` — references this command
- `add-json-output-to-key-commands` — works with JSON output from this command
- `add-claude-doctor-command` — uses metadata to diagnose configuration

## Release Notes

Added `specsync agent-help` command for CLI-generated agent guidance. Agents can now query SpecSync for structured command documentation, enabling reduced prompt sizes and improved automation. Use `specsync agent-help <command>` for human-readable help or `specsync agent-help <command> --json` for machine-readable format.
