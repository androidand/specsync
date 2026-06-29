# Tasks

## Canonical skill file

- [x] Create `skills/specsync/SKILL.md` with agentskills.io frontmatter and verified CLI content

## In-repo project skill

- [x] Create `.claude/skills/specsync/SKILL.md` (copy of canonical, not symlink)

## Copilot always-on context

- [x] Create `.github/copilot-instructions.md` with concise specsync reference

## npm postinstall

- [x] Extend `npm/scripts/postinstall.js` to install skill to all known agent dirs
- [x] Add `"skills/"` to `files` array in `npm/package.json`

## CLI subcommand

- [x] Add `install-skill` subcommand to `cmd/specsync/main.go` dispatch
- [x] Implement `cmd/specsync/installskill.go` with embed, flag parsing, and dir logic
- [x] Embed `skills/specsync/SKILL.md` via `go:embed` in the install-skill file
- [x] Verify: `specsync install-skill --all` writes correct dirs and reports results
