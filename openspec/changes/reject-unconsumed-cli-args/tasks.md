# Tasks: reject-unconsumed-cli-args

Task 1 is the exact defect that caused the 2026-09-24 incident (81 live issues
created on `ExopenGitHub/portal` from an unrelated invocation). Task 2 is the
independent, more general defect that made `-dry-run` silently do nothing. Task 5
is the regression test proving both are actually fixed.

- [ ] 1. Add a shared `rejectUnconsumedArgs(fs *flag.FlagSet)` helper (fails loudly,
       naming the exact leftover argument(s), if `fs.NArg() > 0`) and call it
       immediately after `fs.Parse(args)` in every subcommand whose flag set takes
       no legitimate positional arguments: `runSync`, `runAudit`, `runAuditTasks`,
       `runValidate`, `runArchive`, `runIdeas`, `runIdea` (idea text is the one
       expected positional — special-case it), and any others found during the
       audit. Reproduced 2026-09-24: `specsync -repo ExopenGitHub/portal
       <issue-url> -dry-run` silently ignored the URL as a stray argument, left
       `-change` unset (meaning "sync everything"), and proceeded as a live full
       sync.
       Validation: `specsync -repo owner/repo https://example.com/foo -dry-run` (or
       any subcommand + stray non-flag arg) now exits non-zero with an error
       naming the stray argument, and makes no provider calls.

- [ ] 2. Detect flags shadowed by an earlier positional token — the independent bug
       that made `-dry-run` have zero effect regardless of task 1. Before
       `fs.Parse`, walk the subcommand's registered flags via `fs.VisitAll` to know
       each flag's arity, then scan the raw `args` for the first true positional
       token; any subsequent token matching a registered flag name is "shadowed."
       Fail loudly listing the shadowed flag name(s) instead of silently keeping
       their zero value. Apply this to every subcommand, including ones that
       legitimately take positional arguments (`link`, `set-stage`, `set-priority`,
       `note`, `spinoff`), since the shadowing bug applies to them too.
       Validation: `specsync note some-slug "text" -dry-run` currently silently
       writes for real (dry-run shadowed by the two positionals); after the fix it
       either parses correctly or fails loudly naming `-dry-run` as shadowed —
       either outcome beats "silently does the opposite of what was asked."

- [ ] 3. Add a pre-flight summary printed before any real (non-dry-run) write in
       `runSync`: resolved target repo, resolved spec-source directory, and count
       of changes about to be synced (`target: X — spec source: Y (N changes)`).
       Validation: a real `sync` run's first line of output states the repo, the
       spec source path, and the change count before any provider call.

- [ ] 4. Decide the shadowed-flag scan's matching strategy (Open Question 1: exact
       name only, no typo-fuzzing) and confirm the arity-lookup approach (Open
       Question 2: `fs.VisitAll` to distinguish boolean vs value-consuming flags).
       Record both decisions.
       Validation: a `design.md` note states the decisions; the task-2
       implementation matches them.

- [ ] 5. Regression test reproducing the 2026-09-24 incident's exact argument
       shape end to end: `specsync -repo <repo> <url> -dry-run` run against a
       fixture directory whose `openspec/changes/` holds changes unrelated to
       `<repo>` must fail with a clear "unexpected argument" error, make zero
       provider calls (assert via a call-counting fake provider), and exit
       non-zero.
       Validation: the test fails against the pre-fix code and passes after tasks
       1–2 land.

- [ ] 6. Sweep the rest of `cmd/specsync/main.go` for any other place that calls
       `fs.Parse` and ignores `fs.NArg()`/leftover args, beyond the list in task 1
       — confirm the audit was exhaustive rather than covering only the reported
       cases.
       Validation: every `run*` function in the file either has no free-form
       positional arguments and calls `rejectUnconsumedArgs`, or documents (in a
       comment) exactly which positional arguments it expects and why leftover
       args beyond those still fail loudly.
