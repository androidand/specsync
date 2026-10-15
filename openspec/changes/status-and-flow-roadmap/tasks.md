# Tasks

Each item is a change with its own tasks and slices. Tick it when its last slice is merged.

## Step 0: Prepare

- [ ] 0.1 Run `specsync -dry-run -change <slug>` for every change below and show the maintainer
- [ ] 0.2 After approval, sync the changes so each has an issue to reference in commits
- [ ] 0.3 Decide the open questions: bare `specsync` behaviour, default workspace root, epic chain shown by default

## Step 1: Foundation

- [ ] 1.1 `make-commands-discoverable`: parity test red first, then the `Command` type, then port commands, then delete the duplicate lists

## Step 2: Facts

- [ ] 2.1 `status-pr-and-worktree-state`

## Step 3: Relations (parallel with 4 and 6 once step 2 is done)

- [ ] 3.1 `work-relations-and-overview`
- [ ] 3.2 `status-resume-context`

## Step 4: Config

- [ ] 4.1 `ways-of-working-and-next`

## Step 5: Flow

- [ ] 5.1 `worktree-cleanup`
- [ ] 5.2 `flow-verbs`
- [ ] 5.3 `concise-change-text`

## Step 6: Leaves (any order, after step 2)

- [ ] 6.1 `pr-size-advisory`
- [ ] 6.2 `plan-changes-as-slices`
- [ ] 6.3 `review-map-for-prs`
- [ ] 6.4 `verify-doc-references`

## Step 7: Release

- [ ] 7.1 Cut a release that includes `status` and `overview`
- [ ] 7.2 Run `specsync changelog -unreleased` and check nothing user-facing is missing
