# Tasks

## Bootstrap

- [x] `pull` without `openspec/`: error names the command that creates it, or offers `-init`
- [x] `pull` writes a starter `tasks.md` so the new change validates

## Validate

- [ ] `validate -change <slug>` checks one change only
- [ ] Tests: unscoped run still reports every change

## PR link

- [ ] `pr-body` with no change: print the missing-link notice and the `Closes` line from the branch's issue number
- [ ] Warn once when `openspec/` is gitignored

## Discoverability

- [ ] `help` alias for `agent-help`
- [ ] `topology` with no store explains stores and how to register one
