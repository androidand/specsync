# Auto-update skills on binary upgrade

When `specsync` is upgraded (via npm or any package manager), installed skill files in agent directories should automatically refresh. Currently users must manually run `specsync install-skill --all` to get updated skill content, defeating token-reduction and feature improvements until they discover the need.

## Why

- **Token reduction lost**: v0.10.0 reduced SKILL.md 60% (222→80 lines), but inactive until `install-skill` runs
- **Silent stale docs**: Users keep running old skill versions, missing new commands and safety updates
- **Poor UX**: Upgrade gets no benefit until a magic command is discovered
- **Dogfooding failure**: specsync's own releases don't take effect until manual intervention

## What

On first run after upgrade, detect if installed skill versions are stale and refresh them:

1. Compare installed skill timestamps/versions against specsync binary version
2. If mismatch detected, offer to auto-update (or run silently in CI/automation)
3. Store metadata so each subsequent run knows it's up-to-date
4. Log a one-time notice that skills were refreshed

## Scope

- Detect stale skills in all 5 agent directories (.claude, .codex, .config/opencode, .copilot, .agents)
- Refresh on first post-upgrade run
- Make it silent or offer choice (TBD via implementation)
- No breaking changes to manual `install-skill` command
