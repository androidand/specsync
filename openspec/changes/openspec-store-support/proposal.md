# Support OpenSpec stores as a first-class spec root

## Why
OpenSpec 1.5 ships stores (beta): planning lives in its own git repo, and each code
repo carries one line — `store: <id>` in `openspec/config.yaml`. This removes the
single biggest adoption objection specsync faces, that planning markdown lands in
the code repo and in every PR diff.

specsync does not understand stores. Verified against a real store (openspec 1.5.0):

- Bare `specsync` in a store-configured code repo lists **zero** changes and exits 0.
  Silent empty result, no diagnostic.
- Run from inside the store, repo resolution falls through to the store's own
  `origin` and targets the **planning** repo. Issues get filed in the wrong place,
  silently.
- The same change synced from two different code repos targets whichever repo the
  shell happens to be in. A change has no way to declare where it belongs.

The last point is the root cause: nothing in a change names its target repo, so
`repo.go` can only ask the working directory. OpenSpec's own store docs do not
define this either — changes in a store are location-agnostic by design.

## What Changes
- Read `store:` from `openspec/config.yaml` so bare specsync resolves the store.
- Add `-store <id>` resolving through OpenSpec's machine registry.
- Add `targets` to a committed per-change `specsync.yml`; insert it into repo
  resolution ahead of git-remote auto-detection.
- Fan out one change to several target repos, cross-linking the resulting issues.
- Refuse an unscoped whole-store sync unless `-all` is passed.
- Extend `doctor` with store health: resolved, registered, behind remote, dirty,
  changes missing targets.

- Parse `specs/` deltas, which specsync never did, and add the Verify phase on
  top of them.

## Impact
- Affected specs: store-resolution, spec-verification
- Affected code: store.go, spec_delta.go, verify.go, repo.go, change.go,
  cmd/specsync/{root,verify_spec,doctor_store,main,traceability}.go
- Repo-local mode is unchanged: no `store:`, no `targets`, identical behaviour.
