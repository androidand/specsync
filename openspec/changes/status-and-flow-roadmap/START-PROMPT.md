# Start prompt for the implementing agent

Copy everything below the line.

---

You are implementing a planned set of changes in the specsync repository. It is a Go project that uses only the standard library.

## Goal

Make specsync tell people and agents where work stands (`status`, `overview`), keep issues and PRs short, and give one repeatable flow (`next`, `new`, `start`, `save`, `gates`, `pr`, `update`, `cleanup`), configured per repo.

It must stay general: any tracker, any agent harness, any repo layout. Do not hard-code anyone's conventions. Conventions belong in `openspec/specsync.yml`.

## Read first, in this order

1. `AGENTS.md` and `CLAUDE.md`. They define the workflow, dogfooding and public-repo rules.
2. `openspec/changes/status-and-flow-roadmap/proposal.md` and `tasks.md`. They hold the order, dependencies and rules.
3. The change you are about to build: `proposal.md`, `design.md` if present, `specs/*/spec.md`, `tasks.md`.
4. For the foundation: `openspec/changes/make-commands-discoverable/design.md`.

## Ground rules

- **Public repo.** No home paths, private addresses, credentials or internal repo names in code, docs or tests. Use placeholders such as `acme/api`. `TestNoPrivateDataInPublicRepo` must pass.
- **Tests first.** Write the failing test, watch it fail for the right reason, then write the code.
- **Small slices.** One slice per PR. Stay inside the slice's `budget`. If a slice grows, stop and split it.
- **Standard library only.** No new dependencies.
- **Read-only stays read-only.** `status`, `overview`, `next`, `flow`, `lint` and the dry runs must not write. Add a test for each.
- **One definition per command.** Each command is a type with its `Spec`, `Flags` and `Run`. Never hand-write command docs, tables or lists. Generate them.
- **Asserted facts only.** Relations and status come from git, the provider or specsync's own files. Never infer from names or similarity.
- **Safety.** Never run commands from a repo config unless trusted. Never force-push a shared branch. Never remove dirty or unpushed work.
- **No real syncs.** Use `-dry-run`. Do not create issues, push to a shared remote or release without asking the maintainer.
- **No attribution trailers** such as `Co-Authored-By` in commits.
- **Commits** reference the change's issue (`#n`) once issues exist. Before each PR run `go test ./...`, `go vet ./...`, `gofmt -l .` and `specsync changelog -fail-on-unlinked-commits`.
- **Keep `tasks.md` accurate.** Tick items as they are done.
- **Style.** Keep new docs, issues and PR text short. Short headers, short sentences.

## Start here

**Step 0. Prepare.**
- Work in a new worktree on a new branch, not on `main`.
- Run `specsync -dry-run -change <slug>` for each change listed in the roadmap. Show the maintainer the result. Ask before syncing.
- Ask the maintainer to decide the open questions in the roadmap `tasks.md` (item 0.3). Do not guess.

**Step 1. Foundation: `make-commands-discoverable`.**
1. Write `TestEverySurfaceListsExactlyTheRegisteredCommands` and its source-level check first, against the current code.
2. Run it. It must fail, naming `topology` (missing from `agent-help`, the README and the skill) and `relate` (implemented but unreachable), and the duplicate lists (`knownSubcommands`, the dispatch `switch`, `commandMetadata`, `knownConfusions`). Show the output to the maintainer.
3. Snapshot today's `agent-help` output as a golden file.
4. Add the `Command` interface, `Spec`, `Register` and validation. Port commands one at a time. Keep the old `switch` for commands not yet ported.
5. Delete the duplicate lists when the last command is ported. The parity test turns green.

Stop after step 1. Report: what changed, test results, anything surprising, and the next change you recommend. Wait for the maintainer before starting `status-pr-and-worktree-state`.

## If you are unsure

Prefer asking over guessing. If a spec and the code disagree, say so and propose which to change. If a slice does not fit its budget, split it and tell the maintainer.
