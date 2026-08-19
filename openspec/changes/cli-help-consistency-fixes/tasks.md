# Tasks

## Fix dead relate/work-graph dispatch

- [x] Remove `relate`/`work-graph` from `knownSubcommands` in `main.go`
- [x] Remove `relate`/`work-graph` from the unknown-subcommand hint text
- [x] Remove the false `relate` documentation from README (subcommand list
      + `### relate` section)
- [x] Verify `specsync relate` now errors instead of running a live sync

## Fix agent-help/doctor flag-order parsing

- [x] Add `reorderFlagsFirst` helper in `main.go`
- [x] Use it in `runAgentHelp`'s `fs.Parse`
- [x] Use it in `runDoctor`'s `fs.Parse`
- [x] Verify `agent-help <cmd> -json` and `agent-help -json <cmd>` both work
- [x] Verify `doctor <cmd> -json` and `doctor -json <cmd>` both work

## Add missing agent-help entries

- [x] `doctor`
- [x] `epic`
- [x] `idea`
- [x] `ideas`
- [x] `archive`
- [x] `set-priority`
- [x] `note`
- [x] Verify `agent-help` overview lists all 25 subcommands

## Standardize dash style

- [x] `WORKFLOW.md` (`sync --dry-run` → `sync -dry-run`)
- [x] `README.md` (`ideas --json` → `ideas -json`)
- [x] `CLAUDE.md` (`changes --json` → `changes -json`)
- [x] `agent_help.go` example/tip strings (`--json` → `-json`)

## Validation

- [x] `go build ./...`
- [x] `go vet ./...`
- [x] `go test ./...`
