# Tasks

- [ ] 1. Reproduce with a test: an archived change (date-prefixed slug) whose
      issue marker still carries the pre-archive slug fails
      `ResolveLiveRefs`/`Find` with no local ref cache.

- [ ] 2. Fix `Find`'s (or `ResolveLiveRefs`'s) slug resolution to account for
      the archive rename — strip/retry the date prefix, and/or have
      `openspec archive`'s caller (or `Sync`) re-mark the issue under the
      new slug at archive time so drift can't accumulate.
      Validation: the regression test from task 1 passes.

- [ ] 3. Make `sync.yml`'s full-repo run (or `Sync` itself) refuse to create
      a new issue for an `Archived` change that fails to resolve, rather
      than silently creating a duplicate — this is exactly what happened
      with #139/#141.
      Validation: a fixture archived change with no resolvable issue and no
      cache does not create a new issue; it reports the unresolved state
      instead.

- [ ] 4. Confirm `changelog -resolve-refs` on a fresh checkout (no
      `.specsync/` anywhere) correctly attributes an archived change's
      commits to its issue.
