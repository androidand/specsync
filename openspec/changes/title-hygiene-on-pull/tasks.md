# Tasks: title hygiene on pull

## Pull path
- [ ] Apply `shortenTitle()` on the issue title in `Pull()` before `splitBody()`
- [ ] Add `TitleCleaned`, `TitleBefore`, `TitleAfter` to `PullResult`
- [ ] Pass cleaned title to `splitBody()` instead of raw issue title
- [ ] Report when title was cleaned in both dry-run and real mode

## CLI output
- [ ] `runPull()` prints `title cleaned: "before" -> "after"` when `TitleCleaned` is true
- [ ] Dry-run shows the same output for review before committing

## Tests
- [ ] Test `shortenTitle()` with messy titles from real issues
- [ ] Test that clean titles pass through unchanged (idempotent)
- [ ] Test pull with verbose issue title produces cleaned proposal H1

## Docs
- [ ] Update specsync skill file: note that titles are cleaned on pull
- [ ] Update worktree-workflow skill: remove the "auto-strips" note from agent instructions (now handled by specsync)
- [x] AGENTS.md: add a Dogfooding section — link commits to issues, require reading `specsync changelog` output before completing a change, keep `site/features.json` in sync (done ahead of this change's implementation; tracked here since it's the same underlying problem)

## Related but out of scope here

- The 0.8.0 `CHANGELOG.md` section's ugliest entries (raw title + commit hash)
  are from `looseEntry()`, not `ReleaseNote()` — commits shipped without a
  linked issue, not unclean titles. Title hygiene fixes the entries that *do*
  reach `ReleaseNote()`; it does not fix unlinked commits. That's a process
  fix (commit messages must reference their change's issue), tracked as its
  own change: `openspec/changes/enforce-changelog-commit-linking/`.
