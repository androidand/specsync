# Tasks

## 1. Issue states

- [x] 1.1 Add a batched `IssueStates(refs)` to the provider interface; GitHub implements it with one `gh api graphql` call, others return `unknown`
- [x] 1.2 Test first: one call for N refs; a missing issue yields `unknown` for that ref only

## 2. changes command

- [x] 2.1 Test first: without `-issues` no provider call is made and output matches a golden snapshot
- [x] 2.2 Add `-issues` columns and the `-json` fields `issue`, `issueState`, `issueStateReason`
- [x] 2.3 Compute `closed-unarchived` and `complete-open` (needs at least one task); nothing else
- [x] 2.3a Print the count of changes with no cached ref; test: fresh checkout checks nothing and says so
- [x] 2.4 Add `-drift`: filter and non-zero exit

## 3. Docs

- [x] 3.1 Regenerate `agent-help` and the README command table; add a clean-up example to AGENTS.md
