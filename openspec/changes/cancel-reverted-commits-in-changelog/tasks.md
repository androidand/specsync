# Tasks: cancel reverted commits in changelog

- [x] `Commit.RevertsHash` parsed from "This reverts commit <hash>." in `ParseCommit`
- [x] `cancelRevertPairs()` drops in-range pairs, newest revert first
- [x] Out-of-range revert target: revert commit still renders
- [x] Cancelled commits excluded from unlinked-commit failures
- [x] Tests: simple pair, revert-of-revert chain, out-of-range target, linked pair
