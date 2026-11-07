# Tasks

## 1. Shared naming

- [ ] 1.1 Extract branch and worktree-path derivation from `runPullWithWorktree` into one helper; make `pull -worktree` use it
- [ ] 1.2 Make `topology`'s branch matcher the inverse of that helper, with a round-trip test

## 2. Branch inference

- [ ] 2.1 Resolve the change from the current branch (`feat/<issue>-<slug>`); error with "no change given" on the default branch or a non-matching branch
- [ ] 2.2 Wire inference into `set-stage`, `archive`, `note` and the default sync; an explicit change argument always wins
- [ ] 2.3 `set-stage <stage>` with a single stage-name argument infers the change; test that a slug equal to a stage name still needs `-stage` to be unambiguous
- [ ] 2.4 Test that inference fails on `main` (watch the test go red with inference forced on)

## 3. `specsync start`

- [ ] 3.1 Register `start` in the dispatcher and `agent-help`
- [ ] 3.2 Refuse a change with no linked issue, printing the sync command to run
- [ ] 3.3 Fetch and branch from the remote default branch
- [ ] 3.4 Resolve the Orca executable (`$ORCA_CLI_COMMAND`, `orca-ide` on Linux, then `orca`); `-launcher auto|orca|git`
- [ ] 3.5 Orca launcher: `orca worktree create` with `--name`, `--branch`, `--base-branch`, `--issue`, passing `-agent` and `-prompt` through
- [ ] 3.6 Git launcher: reuse the `pull -worktree` path
- [ ] 3.7 Reuse an existing worktree for the change instead of creating another
- [ ] 3.8 Set stage `active`; sync only with `-sync`
- [ ] 3.9 `-dry-run` prints branch, path and the exact launcher command
- [ ] 3.10 Tests with a fake runner for both launchers, including the "orca on Linux is the screen reader" case

## 4. Rescope related work

- [ ] 4.1 Narrow `openspec/changes/in-flight-work-claims/` to squash-landing detection and cross-worktree file overlap; sync it

## 5. Docs and site

- [ ] 5.1 README: layering in "Why", a `start` section, the `orca.yaml` recipe
- [ ] 5.2 `site/features.json`: `start` and branch inference
- [ ] 5.3 specsync skill: replace the manual worktree steps with `specsync start`

## 6. Verification

- [ ] 6.1 Try the recipe end to end with Orca on this repo: `start` → hook sets `active` → archive hook derives the stage
- [ ] 6.2 `go test ./...`, `gofmt`, `openspec validate`, `specsync changelog -fail-on-unlinked-commits`
