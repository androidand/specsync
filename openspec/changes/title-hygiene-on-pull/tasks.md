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
