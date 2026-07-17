# Tasks: advisory title suggestions

## Transform hardening
- [x] Make `shortenTitle()` a fixpoint — applying it twice equals applying it once
- [x] Guard against empty output: a title that cleans to nothing is returned unchanged
- [x] Keep backtick content, remove only the backtick characters
- [x] Leave unbalanced parens untruncated (balanced-paren guard)
- [x] Drop the trailing detail-word blacklist (mangled legitimate titles, broke idempotency)

## Advisory surface (warn, never mutate)
- [x] `ItemResult.TitleSuggestion` set when sync pushes an H1 that could be tighter
- [x] `PullResult.TitleSuggestion` set when the pulled issue title could be tighter
- [x] Sync pushes the proposal H1 verbatim; pull writes the issue title verbatim
- [x] No suggestions for archived changes (`changes/archive/` is immutable)
- [x] `runSync()` and `runPull()` print `title could be tighter: "..."` (dry-run and real mode)

## Tests
- [x] `shortenTitle()` table test: real messy titles, clean pass-through, unbalanced input, empty guard (`title_test.go`)
- [x] Idempotency pinned across messy, clean, and degenerate inputs
- [x] Sync pushes verbatim and surfaces `TitleSuggestion`
- [x] Pull keeps the H1 verbatim and surfaces `TitleSuggestion`

## Docs
- [x] Skill file (+ mirrors): warn-never-rewrite contract, "H1 = WHAT, not HOW" convention
