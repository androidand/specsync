# Tasks

Depends on `status-pr-and-worktree-state`; benefits from `work-relations-and-overview`.

## 1. Provider

- [ ] 1.1 Parse `-sessions-cmd` and `SPECSYNC_SESSIONS_CMD`; empty value disables for one run
- [ ] 1.2 Run once with timeout (default 5 s) and an output size cap; discard stderr
- [ ] 1.3 Validate the JSON array; ignore unknown fields; require `cwd`
- [ ] 1.4 Never read the provider from repository, change or cache files

## 2. Join and show

- [ ] 2.1 Join entries to rows by longest worktree path prefix; count unmatched entries in a status line
- [ ] 2.2 Add `sessions` and `resume` to rows; default `resume` to `cd <worktree>`
- [ ] 2.3 Read the last non-empty line of `discoveries.md` into `lastNote`
- [ ] 2.4 Show a compact `resume` column in `status`

## 3. Tests

- [ ] 3.1 Provider fixture script: match by prefix, unmatched entry, most recent wins
- [ ] 3.2 Failure modes: missing command, non-zero exit, timeout, invalid JSON, oversized output
- [ ] 3.3 A repo file naming a provider is ignored and executes nothing
- [ ] 3.4 `lastNote` present, absent and empty cases
- [ ] 3.5 Nothing is written to disk by a run with a provider

## 4. Docs

- [ ] 4.1 Implement each new command or flag once, as a `Command` type per `make-commands-discoverable`; README, `agent-help`, the overview and the skill are generated, never hand-written
- [ ] 4.2 Example adapter script in the docs, marked optional and agent-specific
- [ ] 4.3 Update the skill and `site/features.json`
- [ ] 4.4 Run `specsync changelog -fail-on-unlinked-commits`
