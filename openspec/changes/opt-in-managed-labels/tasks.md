# Tasks

- [x] 1. Add `WorkItem.ManagedLabels bool`; `desiredLabels` (`github.go`)
      adds `specsync`/`stage:<stage>` only when true, `priority:<n>`
      unaffected.
      Validation: a fixture `WorkItem` with `ManagedLabels: false` (or
      unset) produces no `specsync`/`stage:` labels but still produces
      `priority:N` when `Priority > 0`; with `ManagedLabels: true` produces
      the previous full set.

- [x] 2. Add `Options.Labels bool` (default false); `Sync` (`sync.go`) sets
      `item.ManagedLabels = opts.Labels` after `WorkItemFor`.
      Validation: `Sync` with `Options{}` (zero value) produces a Push call
      with no managed labels beyond priority.

- [x] 3. Add `-labels` flag to the `sync` CLI command (`cmd/specsync/main.go`),
      wired to `Options.Labels`.
      Validation: `specsync -dry-run` without the flag previews no
      `specsync`/`stage:` labels; `specsync -dry-run -labels` previews them.

- [x] 4. Update existing tests asserting the old always-on default
      (`TestGitHubPushCreate`, `TestGitHubPushUpdateReconcilesLabels`) to
      set `ManagedLabels: true` explicitly.

- [x] 5. New tests: default sync omits `specsync`/`stage:` labels; a sync
      without `-labels` against an issue that already carries stale
      `specsync`/`stage:x` labels removes them (self-cleaning); `-labels`
      restores the full previous set.

- [x] 6. Document in README (the label behavior is currently undocumented
      as a standalone bullet — check and add/update) and `site/features.json`
      per AGENTS.md's dogfooding rule.
