# Define each command once so it is always discoverable

## Why

A request for an overview of one feature's issues, branches, worktrees and pull requests
across four repositories could not be served with specsync, although specsync already
contains most of the answer. Checking why shows that the failure was discovery, and that
discovery fails because a command is described in many places by hand.

**What happened**

- `specsync status` was merged after the latest release, so the installed binary did
  not have it. Sixteen commits sat on the main branch with no release.
- Even on main, nothing points to it. `agent-help` has no entry for it, the skill never
  mentions it, and the skill description (create, update, reconcile, pull, scan, link,
  release impact) holds no word an agent would match to "status", "overview", "where is my
  work" or "recover after a crash". The agent reached for a different tool.
- `specsync -h` prints the flags of the `sync` subcommand, and `specsync` with no
  arguments syncs every change. There is no grouped list of what the tool can do.
- Run against a real checkout, `topology` found the change and issue but returned no branch
  or worktree, because it matches only one branch naming convention.

**Why it keeps happening: one command, many hand-written lists**

A command is currently declared or described in each of these, with nothing tying them
together:

1. `knownSubcommands`, a map of names;
2. the dispatch `switch` in `main()`, which has cases the map lacks (seven today);
3. `commandMetadata`, a hand-written table of about a thousand lines that drives
   `agent-help`;
4. the real flag definitions in each command's `flag.FlagSet`, **and again** as hand-written
   flag entries in `commandMetadata`, so every flag is described twice;
5. the README command tables;
6. the skill file and its three derived copies, plus the skill description;
7. the site's feature list.

The consequences are visible today: `topology` is registered but has no help entry; a
command, `relate`, is fully implemented and tested yet unreachable, and the test that
guards unreachable code allowlists it as "dead code" instead of failing; flag descriptions
can disagree with the flags. The existing drift test only checks that the copies of the
skill file match each other, not that the skill covers the commands the binary has.

Forgetting to document a command is therefore the default outcome, and checking for it
after the fact will always be one step behind. The fix is to make the omission impossible,
not to add another list to keep in step.

## What Changes

- **The description is the code that runs it.** Each command is one type implementing a
  small `Command` interface: its `Spec` (name, group, one-line summary, intent phrases,
  mutation flag, workflow position, safety rules, examples), a `Flags` method that defines
  its flags on a `flag.FlagSet`, and its `Run` method. Because the logic is the `Run`
  method, a command cannot exist without being declared, and cannot be declared without a
  complete description. This is the shape the `go` command uses for its own subcommands,
  using only the standard library. See `design.md`.
- **Registration is automatic and validated.** A command registers itself from `init()` in
  its own file; `Register` panics on an incomplete Spec, an undeclared group, a duplicate
  name or a flag without usage text, so an undocumented command cannot start, let alone
  ship.
- **Everything else is derived from that declaration, never written again:**
  - dispatch and the set of known subcommands (a command that is not declared cannot be
    dispatched);
  - the unknown-subcommand suggestions;
  - `agent-help`, with flag names, types, defaults and descriptions read from the command's
    own `flag.FlagSet`, so each flag is described exactly once;
  - the grouped overview printed by `help`, `-h` and bare `specsync`;
  - the README command table and the skill's command list and task index, generated with
    `go generate`, and the skill description's trigger phrases built from the groups'
    intent phrases.
- **The build refuses incompleteness.** Declaring a command with an empty summary, group,
  example or flag description fails a test. CI regenerates the generated docs and fails if
  they differ from what is committed, so a forgotten regeneration cannot merge.
- **No allowlisted dead commands.** With `Run` as a method there is no separate function to
  leave unwired; an unreferenced helper is reported by the linter's unused-code check, not
  exempted. `relate` is either given a declaration (and documentation) or deleted as part of
  this change.
- **A grouped front door and a task index.** `specsync help`, `-h` and bare `specsync` print
  commands grouped by intent (see what is going on, plan, sync, review and release,
  housekeeping) with a starting point per group. `specsync agent-help tasks` and a matching
  skill table map intents to commands.
- **Every harness gets the same facts.** The existing multi-agent skill installation renders
  from the same declarations, so a skill for any harness cannot describe different commands
  than the binary has.
- **`doctor` reports version skew:** commands the installed skill lacks, and an installed
  skill older than the binary.
- **Release the unreleased,** and add `specsync changelog -unreleased` so merged but untagged
  features are visible.

## Generality

The declarations are CLI-first, so the same source serves people (`help`, the README),
any agent harness (`agent-help`, a generated skill or instruction file) and tooling
(`agent-help -json`). Nothing here assumes a tracker, a language or a repository layout.

## Non-goals

- No interactive or full-screen interface, no model-written help text, no telemetry.
- Does not change what any command does, other than the bare invocation, below.
- Does not generate the marketing site's curated feature cards.

## Compatibility

Bare `specsync` with no flags currently syncs every change. Printing the overview instead
removes a sharp edge (`AGENTS.md` already says to always pass `-change`) but is a behaviour
change. Proposal: print the overview, and keep `specsync sync` and any invocation carrying
sync flags unchanged. Open question for the maintainers before it ships.

The migration touches about fifteen flag sets and the existing help entries. It is done
incrementally behind a snapshot of today's `agent-help -json` output, so descriptions are
preserved or deliberately improved, never silently lost.

## Impact

- New command declaration type and registry; `cmd/specsync/main.go` dispatch and
  `agent_help.go` reduced to derivations; generator and CI check; README and skill
  generated.
- Docs: README, the skill (and derived copies), `site/features.json`.

## Release note

Every command is now defined once and its help, overview, README table and skill are
generated from that definition, so a command can no longer exist without being documented.
`specsync` also shows a grouped overview, and `doctor` reports when your installed skill is
missing commands.
