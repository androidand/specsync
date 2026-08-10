# Add structured JSON output to key SpecSync commands

## Context

The current SpecSync skill teaches Claude to parse prose output from commands. This requires Claude to:

1. Read detailed prose descriptions
2. Understand implied structure
3. Parse human-readable text
4. Reconstruct state for decision-making

Better approach: Commands output structured JSON. Claude consumes deterministic data, not text parsing.

Currently only 3 of 20 commands emit JSON:
- `changes` (--json flag)
- `audit` (--json flag)
- `audit-tasks` (--json flag)

This change adds structured JSON output to 9 more high-value commands.

## Proposed Changes

### Commands to Add JSON Support

1. **sync** — current output is prose log; add `--json` for mutation result
2. **pull** — current output is prose log; add `--json` for creation result
3. **link** — current output is prose log; add `--json` for linking result
4. **scan** — already has `--json` (good precedent)
5. **trace** — current output is prose; add `--json` for graph structure
6. **validate** — current output is prose; add `--json` for validation result
7. **verify** — current output is prose; add `--json` for PR verification result
8. **spinoff** — current output is prose; add `--json` for child change creation result
9. **release-plan** — might already have JSON (check current code)

### JSON Output Schemas

Each command defines its JSON output structure:

#### sync

```json
{
  "change": "my-change",
  "issue": 42,
  "created": false,
  "updated": true,
  "mutations": {
    "title_updated": true,
    "body_updated": true,
    "labels_updated": true,
    "checklist_reconciled": true,
    "dependencies_synced": false
  },
  "reconciliation": {
    "tasks_before": 5,
    "tasks_checked": 3,
    "tasks_after": 5,
    "external_changes": 0
  },
  "board_state": {
    "synced": true,
    "human_override": false
  },
  "url": "https://github.com/owner/repo/issues/42"
}
```

#### pull

```json
{
  "change": "my-change",
  "issue": 42,
  "files_created": [
    "openspec/changes/my-change/proposal.md",
    "openspec/changes/my-change/tasks.md"
  ],
  "files_updated": [],
  "from_title": "Issue title",
  "from_body_length": 1234,
  "to_proposal_h1": "Issue title",
  "title_hygiene": {
    "flag": false,
    "suggestion": null
  }
}
```

#### link

```json
{
  "links": [
    {
      "from": "change-a",
      "to": "change-b",
      "file": "openspec/changes/change-a/links.md",
      "written": true
    },
    {
      "from": "change-b",
      "to": "change-a",
      "file": "openspec/changes/change-b/links.md",
      "written": true
    }
  ],
  "synced": false,
  "note": "Run specsync sync to push links to GitHub"
}
```

#### trace

```json
{
  "change": "my-change",
  "graph": {
    "commits": [
      {
        "hash": "abc123",
        "message": "feat: do something (#42)",
        "change": "my-change",
        "linked": true
      }
    ],
    "branches": [
      {
        "name": "feat/42-my-feature",
        "commit": "abc123",
        "change": "my-change"
      }
    ],
    "issues": [
      {
        "number": 42,
        "change": "my-change",
        "synced": true
      }
    ]
  }
}
```

#### validate

```json
{
  "valid": true,
  "changes_checked": 5,
  "errors": [],
  "warnings": [
    {
      "change": "my-change",
      "issue": "missing-tasks.md",
      "description": "Change has proposal but no tasks"
    }
  ]
}
```

#### verify

```json
{
  "prs_checked": 3,
  "issues": [
    {
      "number": 123,
      "state": "open",
      "branch": "feat/42-my-feature",
      "change": "my-change",
      "has_reference": false,
      "reference": null
    }
  ],
  "valid": false
}
```

#### spinoff

```json
{
  "parent": "my-change",
  "child": "my-change-followup",
  "files_created": [
    "openspec/changes/my-change-followup/proposal.md",
    "openspec/changes/my-change-followup/tasks.md"
  ],
  "issue": 43,
  "url": "https://github.com/owner/repo/issues/43"
}
```

#### release-plan

(Already has JSON support; verify and document)

```json
{
  "since": "v0.9.0",
  "until": "main",
  "changes": [
    {
      "slug": "my-change",
      "issue": 42,
      "committed": true,
      "shipped": false,
      "category": "feature"
    }
  ],
  "archive_candidates": [
    {
      "slug": "archived-change",
      "issue": 41,
      "missing_archive": true
    }
  ]
}
```

### Implementation Strategy

1. **Define structs** (in cmd/specsync/ and/or root library):
   - SyncResult
   - PullResult
   - LinkResult
   - TraceResult
   - ValidateResult
   - VerifyResult
   - SpinoffResult

2. **Add --json flag** to each command's flag set

3. **Populate results** during command execution

4. **Emit JSON** at end of each run* function:
   ```go
   if *jsonFlag {
     result, _ := json.MarshalIndent(result, "", "  ")
     fmt.Println(result)
   } else {
     // existing prose output
   }
   ```

5. **Document output** in `specsync agent-help` (see `add-agent-help-command`)

### Backwards Compatibility

- Default behavior unchanged (prose output)
- `--json` flag is opt-in
- Existing scripts continue to work
- No breaking changes to CLI interface

### Testing

For each command:
- [ ] Test --json flag produces valid JSON
- [ ] Test JSON output has expected schema
- [ ] Test JSON output is complete (no missing fields)
- [ ] Test dry-run also supports --json
- [ ] Test error cases emit appropriate JSON or prose errors

## Benefits for Claude Integration

1. **Deterministic**: Claude reads JSON, not prose interpretation
2. **Schema-aware**: Claude knows exact field names and types
3. **Reduced token use**: JSON is more compact than prose description
4. **Better automation**: Agents can parse structured output reliably
5. **Reduced duplicated documentation**: Skill can reference schema instead of teaching command behavior

## Related Changes

- `reduce-claude-skill-token-cost` — skill can say "use --json, see schema in agent-help"
- `add-agent-help-command` — documents JSON schemas for each command
- `add-claude-doctor-command` — may consume JSON output

## Release Notes

Added structured JSON output to 9 key SpecSync commands: sync, pull, link, trace, validate, verify, spinoff, and improved release-plan. Use the `--json` flag to get machine-readable output suitable for automation and AI agent integration. Existing prose output remains unchanged by default; JSON is opt-in.
