# Tasks: never-fail-a-sync-silently

Task 1 is the bug. Tasks 4–6 are the size-limit symptom that exposed it, and are
worth less on their own — a pre-flight check that still exits 0 when it refuses has
not fixed anything.

- [ ] 1. Exit non-zero when any change fails to sync. A bulk run that syncs 44 of 45
       must not exit 0. Reproduced 2026-08-14: `specsync -change
       host-model-management-api` printed a GraphQL error, reported
       `0 created, 0 updated`, and exited **0**.
       Validation: a test forcing one per-change failure asserts a non-zero exit;
       `specsync -change <oversized>` exits non-zero.

- [ ] 2. Add a failure count to the summary — `27 created, 16 updated, 1 failed` —
       and list the failed slugs at the end of the run, not inline where they scroll
       past. In the original bulk run the error appeared 40 lines above a final line
       reading `27 created, 16 updated`.
       Validation: a bulk run with one failure prints the count and the slug in a
       trailing block.

- [ ] 3. Decide what counts as "failed" for the exit code (Open Question 4). A
       notes-only directory with no `proposal.md` is skipped rather than failed —
       `muse-glimmer-30b-gguf` in llama-skein was skipped silently and sat untracked
       for months, so skips need reporting too, but conflating them with errors
       would make the exit code noisy enough to ignore. Record the decision.
       Validation: `design.md` states the taxonomy — failed vs skipped vs no-op —
       and which of them affect the exit code.

- [ ] 4. Measure the rendered body before sending and refuse locally above GitHub's
       65,536-character limit. The message must name the change, the byte count, the
       limit, and the largest contributing section — `host-model-management-api` was
       71,175 bytes, of which `tasks.md` alone was 68,798.
       Validation: a fixture change over the limit is refused with all four facts in
       the message, and no network call is made.

- [ ] 5. Resolve whether the check is a hard refusal or a warning that still attempts
       (Open Question 1), and whether it also runs at authoring time rather than only
       at sync time (Open Question 2). Check whether `-dry-run` already renders the
       full body (Open Question 3) — if it does, surfacing the size there is nearly
       free.
       Validation: decisions recorded in `design.md`; `-dry-run` reports the body
       size for every change it would write.

- [ ] 6. State the remedy in the refusal message rather than only the problem. An
       oversized change is nearly always one whose `tasks.md` has accumulated
       completion evidence under finished sections. Point at
       `epic-scaffold-command` / `epic-and-subissue-projection` for the structural
       split; do not reimplement splitting here.
       Validation: the message names a next action, and a reader who has never seen
       this error can act on it without reading the source.

- [ ] 7. Regression-test the reported case end to end: a change whose rendered body
       exceeds the limit is refused with an actionable message and a non-zero exit,
       and the same change after splitting syncs cleanly.
       Validation: both halves of that assertion pass in one test.
