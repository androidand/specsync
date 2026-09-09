# Tasks

## 1. CLAUDE.md → @AGENTS.md import

- [x] Rewrite `CLAUDE.md` to import `AGENTS.md` via `@AGENTS.md` syntax
  - [x] Brief header identifying the repo as SpecSync-enabled
  - [x] `@AGENTS.md` import line
  - [x] Short pointer to the SpecSync skill (when to use it, how to get help)
  - [x] Keep under ~15 lines total (CLAUDE.md is always-loaded context)
- [x] Verify the new `CLAUDE.md` renders correctly (no orphaned sections)
- [ ] Update `sync-skill` Makefile target if `.claude/CLAUDE.md` is used

## 2. AGENTS.md — skill routing section

- [x] Add a concise "SpecSync Skill" section to `AGENTS.md`
  - [x] States that the `specsync` agent skill is the operational guide
  - [x] Directs to `specsync agent-help <command>` for command-specific details
  - [x] Keeps it brief — no command references or flag listings
  - [x] Places it after the existing "Documentation Style" section (or before it)

## 3. installskill.go — canonical targets and aliases

- [x] Replace `skillTargets` with two canonical targets:
  - [x] `claude-code` → `~/.claude/skills/specsync/`
  - [x] `agents` → `~/.agents/skills/specsync/`
- [x] Add `--codex` and `--opencode` as compatibility aliases mapping to `--agents`
- [x] Deduplicate: if `--agents --codex` are both set, write only once
- [x] `--all` installs to both canonical targets (Claude Code + Agent Skills)
- [x] Update help text to reflect new target descriptions
- [x] Update installation output to identify Agent Skills as the cross-harness standard

## 4. SKILL.md — broadened description

- [x] Update frontmatter `description` in `skills/specsync/SKILL.md`
  - [x] Covers the full SpecSync/OpenSpec lifecycle
  - [x] Uses clear, harness-portable language
  - [x] Stays within the 1024-character Agent Skills limit
  - [x] Uses directive language without `MUST` (harness-portable)
- [x] Run `sync-skill` Makefile target to propagate to all derived locations
- [x] Verify `TestSkillDrift` still passes

## 5. doctor.go — unified diagnostics

- [x] Add detection for `~/.agents/skills/specsync/SKILL.md` (canonical Agent Skills path)
- [x] Add detection for legacy `~/.codex/skills/specsync/SKILL.md`
- [x] Add detection for legacy `~/.config/opencode/skills/specsync/SKILL.md`
- [x] Add CLAUDE.md validation: check for `@AGENTS.md` import vs. Markdown link
- [x] Update `doctor install` to show canonical + legacy paths
- [ ] Update `doctor claude` to also check Agent Skills installation
- [x] Distinguish errors (skill missing), warnings (legacy install present), and info (version status)
- [x] Update `doctor.go` import detection logic

## 6. Tests

- [x] Test `--agents` flag installs to `~/.agents/skills/specsync/`
- [x] Test `--codex` flag installs to `~/.agents/skills/specsync/` (alias)
- [x] Test `--opencode` flag installs to `~/.agents/skills/specsync/` (alias)
- [x] Test `--agents --codex` writes only once (deduplication)
- [x] Test `--all` installs to both Claude Code and Agent Skills paths
- [x] Test `--claude-code` installs only to Claude Code path
- [x] Test skill destination paths use temporary home directories
- [x] Test `@AGENTS.md` import detection in CLAUDE.md
- [x] Test Markdown link `[AGENTS.md](AGENTS.md)` is flagged as incorrect
- [ ] Test doctor detects canonical Claude installation
- [ ] Test doctor detects canonical Agent Skills installation
- [ ] Test doctor detects legacy `~/.codex/skills/` installation
- [ ] Test doctor detects legacy `~/.config/opencode/skills/` installation

## 7. Dogfooding and completion

- [x] Run `make test` and verify all tests pass
- [x] Run `make vet`
- [x] Run `make sync-skill`
- [x] Run `specsync doctor` locally to verify output
- [x] Run `specsync doctor --json` to verify machine-readable output
- [x] Verify `TestSkillDrift` passes
- [ ] Sync the OpenSpec change
- [ ] Update `site/features.json` if this qualifies as user-facing (it does — installation UX and diagnostics)
- [ ] Ensure changelog/release note quality (title hygiene, no implementation minutiae)
