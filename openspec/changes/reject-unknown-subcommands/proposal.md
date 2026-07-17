# CLI dispatch: reject unrecognized leading arguments instead of silently falling through to sync

## Why

`cmd/specsync/main.go`'s dispatch `switch` matches known subcommands (`pull`, `link`,
`scan`, `trace`, `release-plan`, `changelog`, `install-skill`, `changes`, `set-stage`,
`set-priority`, `sync`) explicitly, and falls to `default: runSync(args)` for anything
else — including typos and words that aren't subcommands at all, like `push`.

`runSync` builds its `flag.FlagSet` and calls `fs.Parse(args)`. Go's `flag` package
stops parsing at the first non-flag argument. If that first argument is an unrecognized
word like `push`, every flag after it — including `-slug` and, critically, `-dry-run` —
is silently never parsed and keeps its zero value.

This was discovered live: `specsync push -slug some-change -dry-run` executed a real,
unscoped sync of *every* change in the repo and wrote to GitHub, despite `-dry-run`
being passed. FusionHub's docs (AGENTS.md, `.claude/tools/openspec.md`, etc.) already
tell agents to run `specsync push` in several places, so this isn't a hypothetical typo
— it's an established (if inaccurate) usage pattern that currently silently discards
every flag typed after it.

## What changes

1. **Safety net**: any leading argument that isn't a recognized subcommand and doesn't
   start with `-` is rejected with a clear error and non-zero exit, instead of being
   silently swallowed by `runSync`'s flag parsing. This catches *any* typo'd subcommand
   name, not just `push`.
2. **`push` is deliberately NOT made an alias for sync.** Considered it, decided against:
   the docs that say "specsync push" were almost certainly written by an agent
   imitating git's vocabulary, not a deliberate human choice — and "push" is actually
   the wrong mental model here. The default sync action reconciles tracker checkbox
   state back into `tasks.md` before writing out (`-reconcile` defaults to `true`), so
   it isn't one-way the way git's push is. Baking a less-accurate name permanently into
   the tool to match a probably-mistaken doc phrase isn't worth the permanent surface
   area (two names for one action, forever, for every future reader to wonder about).
   Instead: `push` fails with an error that says "did you mean sync?" plus the one-line
   reason why — discoverable for git habit, without entrenching the wrong model.

## Non-goals

- No change to `runSync`'s actual behavior or flags — only dispatch-time validation.
- No change to any other subcommand's flag parsing.
- No alias for `push`. Fix the docs that say it, not the tool.
