# CLI help and flag-parsing consistency fixes

## Context

A user asked whether specsync's `-flag` vs `--flag` help text was internally
consistent, and whether all documented commands actually work. Auditing the
CLI surfaced one real safety bug and a few smaller correctness/consistency
gaps in the `agent-help`/`doctor` machinery, on top of confirming that Go's
`flag` package genuinely treats `-x`/`--x` identically (not a bug).

## Issues Found

### 1. `specsync relate` silently ran a live `sync` instead of erroring

`relate`/`work-graph` were still listed in `knownSubcommands` and the
unknown-subcommand hint text in `cmd/specsync/main.go`, and README documented
`specsync relate -change my-feature` as a safe read-only example with no
`-dry-run`. But `d07979d0` had deliberately removed the dispatch `case
"relate"` (pending the not-yet-implemented `work-graph` change), leaving the
command fall through to `default: runSync(rest)`. Following the README
example verbatim, without `-dry-run`, would trigger a real GitHub sync
instead of printing a graph.

### 2. `agent-help <cmd> --json` / `doctor <cmd> --json` ignored `--json` in that order

Go's `flag.Parse` stops at the first positional token, so a flag written
after the subcommand name was silently dropped — the exact order the tool's
own help text told people to use (`agent-help <command> --json`). Only
`agent-help --json <command>` (flag-first) actually worked.

### 3. 9 real subcommands missing from `agent-help`

`doctor`, `epic`, `idea`, `ideas`, `archive`, `set-priority`, `note` all
dispatch and work, but `agent-help <cmd>` for any of them returned "unknown
command," and the top-level `agent-help` overview only listed 16 of the ~25
real subcommands.

### 4. Mixed `-flag`/`--flag` style in docs and help text

`WORKFLOW.md`, `README.md`, and `CLAUDE.md` each had one example using
`--flag` where the surrounding docs consistently use `-flag`. `agent-help`'s
own embedded examples and tips mixed both styles for the same flag
(`-dry-run` in one tip, `--json` in the next).

## Proposed Changes

1. Remove `relate`/`work-graph` from `knownSubcommands` and the
   unknown-subcommand hint in `main.go`; drop the now-false `relate`
   documentation from README. `relate.go` is left in place, unwired, for
   whenever the `work-graph` change actually ships.
2. Add a `reorderFlagsFirst` helper and use it in `runAgentHelp` and
   `runDoctor` so `-json`/`--json` parses correctly regardless of position
   relative to the subcommand name.
3. Add `commandMetadata` entries for `doctor`, `epic`, `idea`, `ideas`,
   `archive`, `set-priority`, `note` in `agent_help.go`.
4. Standardize on single-dash (`-flag`) in `WORKFLOW.md`, `README.md`,
   `CLAUDE.md`, and every example/tip string in `agent_help.go`.

## Impact

**Severity**: Medium (item 1 is a real footgun for anyone following the
README's own `relate` example; the rest are correctness/discoverability
cleanups with no functional risk).

- **Affected**: `cmd/specsync/main.go`, `cmd/specsync/doctor.go`,
  `cmd/specsync/agent_help.go`, `README.md`, `WORKFLOW.md`, `CLAUDE.md`
- **No breaking changes**: no flag or subcommand behavior changes for any
  command that was already working correctly; `relate`/`work-graph` were
  already effectively broken (silently doing the wrong thing), so removing
  them from the menus is a strict improvement, not a regression.

## Release Notes

Fixed `specsync relate` silently running a live sync instead of erroring
(it was already unwired; now it fails loudly like any other unknown
subcommand). Fixed `agent-help`/`doctor` ignoring `--json` when placed after
the subcommand name. Added `agent-help` entries for `doctor`, `epic`,
`idea`, `ideas`, `archive`, `set-priority`, and `note`. Standardized CLI
help text and docs on single-dash flag style throughout.
