# Tasks: cancel reverted commits in changelog

- [ ] `Commit.RevertsHash` parsed from "This reverts commit <hash>." in `ParseCommit`
- [ ] `cancelRevertPairs()` drops in-range pairs, newest revert first
- [ ] Out-of-range revert target: revert commit still renders
- [ ] Cancelled commits excluded from unlinked-commit failures
- [ ] Tests: simple pair, revert-of-revert chain, out-of-range target, linked pair
