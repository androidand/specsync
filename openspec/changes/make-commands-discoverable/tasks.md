# Tasks

Done incrementally; each phase leaves the tool working and its output unchanged or deliberately improved.

## 1. Safety net first

- [x] 1.0a Add the source-level entry to the same test: parse the package, report any map, slice or `switch` outside a declaration holding three or more registered command names; confirm it is red for the current switch, `knownSubcommands`, `commandMetadata` and `knownConfusions`
- [x] 1.0 Write `TestEverySurfaceListsExactlyTheRegisteredCommands` first, against today's code, and confirm it fails naming `topology` and `relate`; keep it as the acceptance test for this change

- [x] 1.1 Snapshot today's `agent-help` and `agent-help -json` output for every command as a golden file
- [ ] 1.2 Snapshot the flag set of every command (name, default, usage) to detect silent loss during migration

## 2. One type per command

- [x] 2.1 Add the `Command` interface, `Spec`, typed `Group` constants and `Register` with validation that panics on an incomplete Spec, duplicate name, or a flag without usage
- [x] 2.2 Make registration automatic from each command's file via `init()`; order output by group then name
- [x] 2.3 Port commands one at a time into their own types holding their flag values; the old `switch` handles any command not yet ported
- [x] 2.4 When the last command is ported, delete the `switch`, `knownSubcommands` and `commandMetadata`
- [x] 2.5 Resolve `relate`: give it a declaration and documentation, or delete it with its tests; remove the dead-code allowlist entry
- [x] 2.5a Move the habit-word suggestions (`knownConfusions`) into a `Spec` field so no separate table remains
- [x] 2.6 Declare `status` (renamed from the unreleased `topology`) with a full specification
- [x] 2.7 Test: every registered command is listed by `help`, and every example parses against its own flag set

## 3. Derive, do not repeat

- [x] 3.1 Build `agent-help` from declarations, reading flags from each command's own `flag.FlagSet`
- [x] 3.2 Confirm the derived `agent-help` output matches the golden snapshot, or differs only by deliberate improvement
- [x] 3.3 Generate the README command table and the skill command list and task index with `go generate`
- [ ] 3.4 Build the skill description trigger phrases from the groups' intent phrases
- [ ] 3.5 Render each supported harness's skill from the same declarations (extend `install-skill`)

## 4. The build refuses incompleteness

- [ ] 4.1 Extend the parity test to every harness's rendered skill, and keep one entry per surface; test: every declaration has a non-empty summary, group, at least one example, and a description on every flag
- [ ] 4.2 CI step: regenerate the generated docs and fail on any difference
- [ ] 4.3 Extend the skill drift test so all derived copies are generated, not hand-kept

## 5. Front door, index, doctor, releases

- [ ] 5.1 Grouped overview for `help`, `-h` and bare `specsync`; sync flags still sync (decide the bare-invocation change with the maintainers)
- [ ] 5.2 `agent-help tasks` and the matching table in the skill
- [ ] 5.3 `doctor`: list commands the installed skill lacks; warn when the skill is older than the binary
- [ ] 5.4 `changelog -unreleased`
- [ ] 5.5 Cut a release that includes `status` and the work built on it

## 6. Tests and docs

- [x] 6.1 A fixture declaration with a missing example fails; a fixture with everything passes
- [ ] 6.2 Overview output is non-mutating (no files, no tracker calls)
- [ ] 6.3 Two harnesses' rendered skills list identical commands
- [ ] 6.4 README, `agent-help`, skill and `site/features.json` updated; `specsync changelog -fail-on-unlinked-commits`
