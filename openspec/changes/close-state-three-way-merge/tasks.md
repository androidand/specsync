# Tasks: closing is a review decision, not a spec-push side effect

## Merge base (`provider.go`, `sync.go`)
- [x] Add `Ref.BaseClosed *bool` — the open/closed state specsync last asserted
- [x] Keep `nil` distinct from `false` ("no claim" vs "specsync left it open")
- [x] Preserve the base across pushes when a provider asserts nothing this run
- [x] `pull` carries the base forward instead of building a fresh ref that drops it —
      and must NOT seed it from the observed remote state (that would license
      undoing a close specsync never made)

## Reopen gate (`github.go`)
- [x] Reopen only when the base says specsync closed it
- [x] Leave the item alone when the base is open or absent
- [x] Do not adopt an external close as the new base
- [x] Record the base on close, on create-and-close, and when remote already matches
- [x] Body and label updates still happen either way — only state is deferred

## Workflow
- [x] Drop `-close-completed` from both steps of `.github/workflows/sync.yml`
- [x] Comment why, so it isn't "helpfully" restored

## Tests
- [x] Reopen gate table: base true reopens; base false and base nil defer (`specsync_test.go`)
- [x] Close records base true (`specsync_test.go`)
- [x] Round trip: close → base persisted to refs.json → new work → reopen (`close_state_merge_test.go`)
- [x] Externally closed issue is left alone, content still synced, base not adopted
- [x] Without `-close-completed`, neither close nor reopen is called
- [x] Rewrote `TestGitHubPushReopensManagedActiveIssue`, which encoded the old requirement
- [x] Re-pull carries the base forward (verified failing before the fix)
- [x] Pull records the task base from what it wrote; task-less pull keeps the prior one
- [x] End-to-end: an uncheck on the issue propagates after a pull (verified failing before the fix)
- [x] Full suite green

## Deferred (see proposal Non-Goals)
- [x] Surface the deferral in run output — needs the board's plan-shaped reporting — *done: added Ref.CloseSkipped to carry the reason from Push, ItemResult.CloseSkipped to surface it, and printBoardPlan renders "issue left closed (<reason>)"*
