# specsync Development

SpecSync uses OpenSpec and itself (dogfooding) to manage development.

## Quick Links

- **Full agent guidelines**: See [AGENTS.md](AGENTS.md)
- **SpecSync commands**: `specsync agent-help` or `specsync agent-help <command>`
- **Workflow patterns**: See [AGENTS.md#Working Paths](AGENTS.md#working-paths)
- **PR conventions**: See [AGENTS.md#Branches, Worktrees & PRs](AGENTS.md#branches-worktrees--prs)

## TL;DR

1. All code changes need an OpenSpec spec in `openspec/changes/<slug>/`
2. Before implementation: `specsync -dry-run -change <slug>`
3. During implementation: keep `tasks.md` current, sync with `specsync -change <slug>`
4. Before merging: ensure PR references the issue, CI passes (includes dogfooding checks)
5. When done: archive the change with `openspec archive <slug> -y`

## Repository Overview

- **OpenSpec changes**: `openspec/changes/` — All work tracked as specs
- **Implementation**: Changes reference GitHub issues synced by specsync
- **Dogfooding**: This repo dogfoods specsync; every issue is spec-generated
- **CI gates**: Changelog linking, archive hygiene, task accuracy enforced
- **Agent tools**: `specsync agent-help`, `specsync doctor`, `specsync changes -json`

## Where to Go Next

See **[AGENTS.md](AGENTS.md)** for:
- Principles and rules
- Security requirements
- Dogfooding standards
- Complete working paths
- PR and branch conventions
- Commit message standards
- Documentation style
