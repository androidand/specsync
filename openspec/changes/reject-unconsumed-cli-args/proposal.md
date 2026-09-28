# Reject unconsumed CLI arguments instead of silently dropping them

## Why

Reproduced 2026-09-24 against a real, shared GitHub repo (`ExopenGitHub/portal`):

```
$ specsync -repo ExopenGitHub/portal https://github.com/ExopenGitHub/portal/issues/4304 -dry-run
```

Intent: pull/inspect a single issue. Actual result: specsync ran a full **live**
`sync` of every local OpenSpec change under the current directory (`~/dev/specsync`'s
own ~80-item internal backlog) and created all of them as real issues on
`ExopenGitHub/portal` — a repo those changes have nothing to do with. `-dry-run` had
no effect whatsoever; every issue was actually created. Cleanup required manually
deleting issues #4370–#4450.

Two independent defects compounded:

1. **`runSync` never checks for leftover positional arguments.** `resolveSubcommand`
   already dispatches a bare `-repo ...` invocation (no subcommand word) to `sync`,
   which is correct — but the URL positional argument that followed had nowhere to
   go. `runSync` calls `fs.Parse(args)` and then proceeds unconditionally; it never
   inspects `fs.NArg()`. The stray URL was silently discarded, `-change` stayed
   unset, and an unset `-change` means "sync **every** change" — turning a mistyped
   single-issue command into a full destructive sync with no error, warning, or
   pause of any kind.
2. **`-dry-run` was silently never parsed at all**, independent of defect 1. Go's
   stdlib `flag` package stops parsing at the very first non-flag token. Because the
   URL came *before* `-dry-run` on the command line, `flag.Parse` stopped at the URL
   and never saw `-dry-run` — it kept its zero value (`false`). This is a generic,
   well-known footgun of `flag`, and it means **any** flag placed after **any**
   positional-looking token on **any** specsync subcommand — not just `sync` — is
   silently dropped without so much as a warning. `main.go` already documents this
   exact mechanism in a comment on `resolveSubcommand` (guarding against a
   *mistyped subcommand word* reaching this trap) but the guard was never extended
   to stray positional arguments *within* an otherwise-valid flag set.

Either defect alone would have been a near-miss. Together they turned a typo'd
invocation into 81 live writes against a shared repository with zero feedback that
anything unusual was happening — the tool's own stdout looked exactly like a normal
successful bulk sync.

## What Changes

- **`sync` (and every other subcommand whose flag set takes no free-form positional
  arguments) fails loudly on any leftover argument.** After `fs.Parse(args)`, check
  `fs.NArg()`; if non-zero, exit non-zero with an error naming the exact unconsumed
  argument(s) and stating plainly that any flags appearing after them (Go's `flag`
  package stops parsing at the first non-flag token) were **not applied** — do not
  let the run proceed as if nothing were wrong. Audit every subcommand in
  `cmd/specsync/main.go` (`sync`, `audit`, `audit-tasks`, `validate`, `archive`,
  `ideas`, and any others) to classify which legitimately accept positional
  arguments (`link`, `set-stage`, `set-priority`, `note`, `spinoff` via `-from`) and
  add the same `NArg()` guard to every one that does not.
- **Detect flags shadowed by an earlier positional token, across every subcommand —
  including ones that legitimately take positional arguments.** Before `fs.Parse`,
  scan the raw `args` for any token matching a flag registered on that subcommand's
  `FlagSet` (`-name` / `--name` / `-name=value`) that appears *after* the first
  non-flag token. If found, fail loudly listing the shadowed flag name(s) instead of
  silently keeping their zero value — this is the generic form of defect 2 and would
  independently have caught this incident even if defect 1 had not existed.
- **Print an unmissable pre-flight summary before any real (non-dry-run) write.**
  Every real `sync` run should state, before doing anything: the resolved target
  repo, the resolved spec-source directory, and the number of changes about to be
  synced (`target: ExopenGitHub/portal — spec source: ~/dev/specsync/openspec (81
  changes) — 0 already linked, 81 new`). A human skimming this line would have
  caught "81 new" against a repo they'd only ever pushed one issue to.
- **Regression test reproducing this incident's exact shape**: `specsync -repo
  <repo> <url> -dry-run` run against a directory whose `openspec/changes/` holds
  unrelated changes must fail with a clear "unexpected argument" error, make **zero**
  provider calls, and exit non-zero.

## Capabilities

### Modified Capabilities

- CLI argument parsing (`cmd/specsync/main.go`): every subcommand rejects
  unconsumed positional arguments and shadowed flags instead of silently
  discarding them.
- `sync`'s real-run output: gains a pre-flight target/count summary before any
  write.

## Non-Goals

- **Not** reworking argument parsing onto a different library (e.g. `cobra`,
  `kong`). The fix is a validation pass around the existing stdlib `flag` usage,
  not a framework migration — that's a much larger, separate change.
- **Not** making `-repo` scope *which* local `openspec/` directory is read. That
  remains cwd-based by design (documented in `resolveRoot`); the incident's actual
  bug was that a mismatch between the two produced *no signal at all*, not that the
  mismatch itself is disallowed. A deliberate cross-repo `sync` (e.g. `scopedTargetRepos`)
  is a legitimate, existing use case and must keep working.
- **Not** adding a confirmation prompt before every real sync. That would make
  routine, correct usage (CI, scripted syncs) annoying or outright broken. The
  pre-flight summary is informational, not a blocking prompt.

## Open Questions

- **Should the shadowed-flag scan be exact-name matching only, or also catch
  near-miss typos** (e.g. `-dryrun` instead of `-dry-run`)? Leaning exact-match
  only for this change — typo detection is a separate, fuzzier problem
  (`advisory-title-suggestions` territory) and conflating them risks false
  positives that make the new guard itself untrustworthy.
- **Does the shadowed-flag scan need to know each flag's arity** (boolean vs
  value-consuming) to correctly identify "the first non-flag token"? Yes — e.g.
  `-repo ExopenGitHub/portal` is two tokens for one flag; the scan must walk the
  `FlagSet`'s registered flags (via `fs.VisitAll`) to know which names consume a
  following value, mirroring how `flag.Parse` itself walks arguments, or it will
  misidentify a flag's value token as "the first positional."

## Impact

- `cmd/specsync/main.go`: `runSync` and every other flag-parsing `run*` function
  gains an unconsumed-argument / shadowed-flag check immediately after
  `fs.Parse`.
- A new shared helper (e.g. `rejectUnconsumedArgs(fs *flag.FlagSet, rawArgs
  []string)`) used by every subcommand, so the fix can't silently regress on the
  next new subcommand the way it silently never covered `sync` this time.
- `runSync`'s real-run path gains a one-line pre-flight summary before the first
  provider write.
- New regression test(s) under `cmd/specsync/` reproducing the 2026-09-24
  incident's argument shape.
