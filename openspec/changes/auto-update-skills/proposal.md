# Auto-update skills on binary upgrade

When `specsync` is upgraded (via npm or any package manager), installed skill files in agent directories should automatically refresh. Currently users must manually run `specsync install-skill --all` to get updated skill content, defeating token-reduction and feature improvements until they discover the need.

## Why

- **Token reduction lost**: v0.10.0 reduced SKILL.md 60% (222→80 lines), but inactive until `install-skill` runs
- **Silent stale docs**: Users keep running old skill versions, missing new commands and safety updates
- **Poor UX**: Upgrade gets no benefit until a magic command is discovered
- **Dogfooding failure**: specsync's own releases don't take effect until manual intervention

## What

Auto-update skills during explicit maintenance operations:

1. **`specsync install-skill`** — installing/upgrading skills automatically uses the new binary version
2. **`specsync doctor`** — diagnoses skill health and auto-fixes stale versions (pass `--skip-skill-update` to skip)
3. **Version detection** — embed semantic version in SKILL.md marker (`<!-- specsync-skill-version: X.Y.Z -->`)
4. **Comparison** — compare installed versions against binary version and update if stale

## Scope

- Detect stale skills in all 5 agent directories (.claude, .codex, .config/opencode, .copilot, .agents)
- Update only on explicit commands: `install-skill` and `doctor`
- Clear messaging: show what was updated and to which version
- No hidden mutations: work commands (sync, pull, scan) unchanged
- Doctor includes `--skip-skill-update` flag for read-only diagnostics in CI
