# Tasks: Guide agents toward one branch, one worktree per issue

- [x] `agent-help` overview (text + JSON) states the branch/worktree-per-issue convention directly, not just "read AGENTS.md"
- [x] `agent-help pull` documents `-worktree`/`-worktree-dir` and adds a matching safety rule
- [x] Skill (`SKILL.md`, `docs/reference.md`, `docs/workflow.md`) mentions the convention and the flag, including the spec-first gap (no automation there yet)
- [x] `make sync-skill`; `go build`/`go vet`/`go test` clean
