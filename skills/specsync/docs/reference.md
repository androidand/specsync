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

## epic

Mint (or converge onto) a coordination epic issue and wire cross-repo children to it — the "feature X needs Y in the backend repo and Z in the frontend repo" workflow.

```
specsync epic <title> [--repo owner/name] [--child <slug|owner/repo#N|url>]... [--dry-run]
```

**Flags:**
- `--repo owner/name` — Repo the epic issue itself lives in (default: auto-detect from git remote). Only bare `#N` children resolve against it; `owner/repo#N` and URL children ignore it.
- `--child <slug|owner/repo#N|url>` — A child to attach; repeatable, may span different repos. A local change slug with no synced issue yet is synced automatically before being attached.
- `--dry-run` — Preview without creating or editing any issue.

Creates one issue labeled `type:epic` and `specsync`, keyed by an identity marker derived from the normalized title — re-running with the same title converges onto the same issue instead of duplicating it. Each child is wired to the epic (and the epic to each child) via a managed `## Related` section, upserted idempotently, until native GitHub sub-issue attachment (`epic-and-subissue-projection`) lands. Combine with `--blocked-by`/`--blocks` (once `issue-dependency-sync` lands) to record real dependency direction between children.

The epic's body is fully regenerated on every run — it is not a spec, so there is no `proposal.md` to preserve, and a manual edit to the issue body is overwritten on the next `specsync epic` call.

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
