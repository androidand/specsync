# Tasks

## 1. Scan and resolve

- [ ] 1.1 Scan change folders and given paths for hashes (7 to 40 hex, standalone, code or "commit" context) and request references
- [ ] 1.2 Resolve hashes with git; classify as stale or orphaned by reachability from the default and change branches
- [ ] 1.3 Check `#n` and `owner/repo#n` through the provider; skip with a status line when unavailable
- [ ] 1.4 Reuse the reference parser shared with `verify` and the relations graph

## 2. Output

- [ ] 2.1 Report file, line, reference, classification and the commit subject for orphaned hashes
- [ ] 2.2 Advisory exit 0 by default; `-fail-on-stale` exits non-zero

## 3. Tests

- [ ] 3.1 Fixture repo: resolving hash, stale hash, orphaned hash after a rewrite
- [ ] 3.2 Precision: hex-looking words, hashes in prose, and code spans
- [ ] 3.3 Provider unavailable still runs hash checks
- [ ] 3.4 No file is modified and no replacement is suggested

## 4. Docs

- [ ] 4.1 Declare the flag once in the command registry; README, `agent-help` and the skill are generated from it
- [ ] 4.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
