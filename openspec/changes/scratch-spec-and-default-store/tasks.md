# Tasks

## 1. `init`

- [x] 1.1 Add `specsync init`: create `./openspec/changes/`, add `/openspec/` to `git rev-parse --git-path info/exclude`, idempotent, refuse if `openspec/` is tracked
- [x] 1.2 Tests: plain repo, linked worktree, run twice, `openspec/` tracked
- [x] 1.3 Make `pull -init` (from `smooth-first-use-in-a-new-repo`) call it; without `-init` nothing is created

## 2. Error text

- [x] 2.1 Name `specsync init` and `-store` in the missing-`openspec/` error (coordinate with `smooth-first-use-in-a-new-repo` item 1)

## 3. Default store

- [x] 3.1 Read `~/.config/specsync/config.yaml` / `SPECSYNC_DEFAULT_STORE` and add it last in `ResolveSpecRoot`
- [x] 3.2 Tests for the resolution order, including a local `openspec/` winning

## 4. Docs

- [x] 4.1 Update the specsync skill, `agent-help pull` and `agent-help init`
- [x] 4.2 Update the agent tooling that bootstraps worktrees (outside this repo) to use `specsync init` in repos without `openspec/`
