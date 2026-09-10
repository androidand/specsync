# Commit a change's stage so a shared plan agrees with itself

## Why
specsync advertises workflow stages as "committed as metadata rather than
external labels" — the argument for keeping planning state in the repo instead
of in tracker labels. It is not true. `Stage` is written to
`.specsync/metadata.json`, and the whole `.specsync/` directory is gitignored:
`git ls-files` finds zero tracked files under it.

Repo-local and solo, nobody notices. It breaks the moment planning is shared:
in a store, every teammate's clone has its own private idea of what stage each
change is in, and a stage set by one person is invisible to everyone else.

Surfaced while implementing openspec-store-support, which hit the same defect
for `targets` and resolved it by declaring them in a committed per-change
`specsync.yml`. Stage belongs in the same file.

## What Changes
- Read `stage` and `priority` from the committed per-change `specsync.yml`.
- Keep reading `.specsync/metadata.json` as a fallback, and migrate on the next
  write, so existing local state is not lost.
- Keep the projection cache (`refs.json`, `board.json`, `deps.json`) gitignored:
  those are caches, and committing them would leak projection ids.

## Impact
- Affected specs: change-state
- Affected code: store.go, change.go, the set-stage and set-priority commands
