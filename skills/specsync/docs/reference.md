# Command Reference

## sync

Synchronize an OpenSpec change with GitHub Issues.

```
specsync [-dry-run] [-change <slug>] [-reconcile] [-close-completed] [-repo owner/name]
```

**Flags:**
- `-change <slug>` — Sync only this change (default: all changes)
- `-dry-run` — Preview without GitHub calls
- `-reconcile` (default true) — Merge task state back from GitHub
- `-close-completed` (default false) — Auto-close when all tasks checked
- `-repo owner/name` — Override auto-detected repo
- `-project owner/number` — GitHub Projects board
- `-provider` — github (default) or beads

## pull

Pull an issue into a local OpenSpec change.

```
specsync pull -issue <N> [-change <slug>] [-dry-run] [-repo owner/name]
```

**Flags:**
- `-issue <N>` (required) — Issue number to pull
- `-change <slug>` — Override auto-derived change name
- `-dry-run` — Preview without writing files
- `-repo owner/name` — Override auto-detected repo

## link

Cross-link two or more OpenSpec changes.

```
specsync link [-dry-run] <change1> <change2> [<change3>...]
```

Creates `links.md` in each change with references to others. Arguments can be change slugs or issue references (`#N`, `owner/repo#N`, URLs).

## scan

Scan for existing work in a code area or topic.

```
specsync scan [-json] <path...> [topic words]
```

Positional arguments split automatically:
- **Paths**: contain `/`, `*`, `?`, `[`; start with `.`; or exist as files/directories
- **Topics**: joined into search keywords

## changes

List all OpenSpec changes with status.

```
specsync changes [--stage <stage>] [--json]
```

**Stages**: backlog, active, blocked, in-review, complete, archived

## set-stage

Set the workflow stage of a change.

```
specsync set-stage <slug> <stage>
```

**Stages**: backlog, active, blocked, in-review, complete, archived, auto

## release-plan

Inspect release impact and generate release notes.

```
specsync release-plan [-json] [-since <ref>] [-until <ref>]
```

## changelog

Generate release notes from changes and commits.

```
specsync changelog [-resolve-refs] [-since <ref>] [-until <ref>]
```

## pr-body

Generate correct PR body fragment for a change.

```
specsync pr-body -change <slug>
```

Returns `Part of #N` while work remains, `Closes #N` when complete.

## verify

Verify PR references to their change issues.

```
specsync verify
```

Scans open PRs and warns if missing issue references.

## spinoff

Spin off emergent work from a discovery.

```
specsync spinoff -from <slug> -task <N> [-kind bug|followup|task] [-change <slug>]
specsync spinoff -from <slug> -text "<discovery>" [same flags]
```

## validate

Validate OpenSpec change structure.

```
specsync validate
```

## audit

Audit changes for structural issues.

```
specsync audit [-json]
```

## audit-tasks

Audit that task status matches code changes (dogfooding).

```
specsync audit-tasks [-json]
```

## trace

Trace dependencies and relationships of a change.

```
specsync trace [-change <slug>] [-json]
```

## install-skill

Install the SpecSync skill.

```
specsync install-skill [--claude-code] [--all] [--profile minimal|docs|full]
```

## agent-help

Get CLI-generated help for commands.

```
specsync agent-help [<command>] [--json]
```

## doctor

Diagnose Claude Code integration and token usage.

```
specsync doctor [claude|install|context|skill] [--json]
```
