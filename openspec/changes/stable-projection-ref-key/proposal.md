# Stabilize the projection ref-cache key so sync never duplicates an issue

## Why

Two-way sync silently created a **duplicate issue** in a real run, defeating the
whole point of the ref cache. Reproduced against `ExopenGitHub/ExoKit`:

1. A change was created by `specsync pull -issue 15` (issue-first flow). `pull`
   constructs a repo-scoped provider, so the ref was cached under the key
   `"github:ExopenGitHub/ExoKit"` (see `GitHubProvider.Name()` in `github.go:51`).
2. A later `specsync -slug <change>` was run **without `-repo`**. That provider
   auto-detects the repo via the git remote but leaves `p.repo` empty, so
   `Name()` returns the bare `"github"`.
3. `syncOne` (`sync.go:68`) looks up `refs[prov.Name()]` — `refs["github"]` — which
   is absent, so `hadRef=false` and it calls `Push(existing=nil)`.
4. `Push` (`github.go:118`) defends against duplicates with `Find`, which matches
   **only by the body marker** `<!-- specsync:change=<slug> -->`. Issue #15 had no
   marker (`pull` does not write the marker back into the source issue), so `Find`
   returned nil and `Push` **created a new issue (#16)** instead of updating #15.

Two independent gaps line up to produce the duplicate:

- **The ref-cache key is unstable.** For the *same repository*, an auto-detected
  provider (`"github"`) and an explicit/`pull`-constructed one (`"github:owner/repo"`)
  key the cache differently. A ref saved under one is invisible under the other, so
  `hadRef` is wrongly `false`.
- **A `pull`-linked issue is un-rediscoverable.** `pull` binds a change to an
  existing issue but never persists the identity marker into that issue, so once the
  cache key is missed, `Find` cannot recover the link and the create path fires.

The ref cache is documented as "purely an optimization … rebuilt via the provider's
Find" (`cache.go:11`). That contract only holds if (a) the key is stable and (b) the
marker actually exists on linked issues. Both were violated.

## What Changes

- **The GitHub provider resolves its repo once and keys the cache canonically.**
  `Name()` always returns `"github:owner/repo"` for the concrete target repo,
  whether the repo came from `-repo` or from git-remote auto-detection. Auto-detected
  and explicit-repo providers pointing at the same repo now share one cache key, so a
  ref saved by `pull` is found by a later `sync`.
- **Ref lookup is backward-compatible and self-healing.** `syncOne` resolves a
  change's ref by the canonical key and, failing that, by the legacy bare-provider
  key (`"github"`), migrating a hit to the canonical key on the next save. Existing
  `refs.json` caches keep working without a manual edit.
- **`pull` writes the identity marker into the source issue.** Pulling issue `#N`
  edits the issue body to carry `<!-- specsync:change=<slug> -->`, so the link is
  durable: even if the cache is deleted, a later `sync` rediscovers `#N` via `Find`
  and updates it instead of creating a duplicate. (Honors `-dry-run`: preview only.)

### Out of scope / explicitly deferred
- Providers other than GitHub — Beads keys refs by its own provider name and is
  unaffected; an equivalent guarantee arrives with each provider.
- De-duplicating issues that were *already* created by this bug — this change
  prevents new duplicates; cleaning up existing pairs stays a manual/`link` task.
- Changing the marker format or the `Find` search query.

## Capabilities

### New Capabilities
- `stable-projection-identity` — a change's projection is identified by a repo-stable
  cache key plus a durable on-issue marker, so `sync` updates the existing issue
  across `pull`/`push` and `-repo`/auto-detect combinations and never creates a
  duplicate.

## Impact

- `github.go`: `NewGitHubProvider` resolves the remote repo at construction (or
  lazily on first use) so `Name()` yields `"github:owner/repo"` consistently;
  `repoFlag()` unchanged behaviorally.
- `sync.go` (`syncOne`) / `cache.go`: canonical-key lookup with a legacy-key
  fallback and migrate-on-save.
- `pull.go`: after fetching, push the marker into the source issue body (idempotent
  upsert), guarded by `-dry-run`.
- Stays stdlib-only and shells out to `gh`. Extends the ref-cache model from
  `two-way-reconcile` and the `github:owner/repo` keying introduced with
  `cross-repo-linked-issues`; depends on no in-flight change.
