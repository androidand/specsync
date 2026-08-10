# Tasks for pr-issue-traceability

- [x] 1.1 Provider-level reference syntax: `ReferenceLine(item)` returning
  `Part of #N` (GitHub) / `Related to #N` (GitLab). Never emit a closing
  keyword here — closing stays stage-derived.
- [x] 1.2 `specsync pr-body -change <slug>`: print the reference line plus an
  optional `-body-file` merge, idempotently (running twice must not stack
  duplicate lines). Exit non-zero if the change has no synced ref.
- [x] 1.3 Decide `Closes #N` eligibility: emit it ONLY when every task in the
  change is checked, i.e. the same predicate `-close-completed` uses. Unit
  tests must cover the phased case — a partially complete change must get
  `Part of`, never `Closes`.
- [x] 1.4 `specsync verify`: warn on an open PR whose head branch equals a
  change id but whose body has no reference to that change's issue.
- [x] 1.5 Docs: SKILL.md + WORKFLOW.md state "PRs reference, completion
  closes", with the phased-change rationale. Agents read SKILL.md, and this
  failure mode is entirely a forgetting problem.
- [x] 1.6 Regression test from the reported incident: a change with 4 phases
  where phase 0 lands — assert the generated body says `Part of #N` and that
  the issue remains open after a simulated merge.
