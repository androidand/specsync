# Design

## Row shape

Additive fields on `ChangeTopology`. Every block is optional and omitted when its
source is unavailable.

```json
{
  "slug": "add-field-forms",
  "pr": {
    "number": 42, "url": "https://example.test/acme/web/pull/42",
    "state": "open", "draft": false,
    "review": "REVIEW_REQUIRED", "mergeable": "MERGEABLE",
    "checks": {"pass": 9, "fail": 0, "pending": 0},
    "size": {"files": 8, "additions": 167, "deletions": 19, "commits": 7}
  },
  "local": {
    "dirty": 0, "untracked": 1,
    "upstream": "origin/feat/add-field-forms", "ahead": 2, "behind": 0,
    "base": "main", "behindBase": 2,
    "size": {"files": 10, "additions": 214, "deletions": 36, "commits": 9},
    "lastCommit": "2026-10-13T09:12:00Z", "volatilePath": false
  },
  "attention": ["unpushed", "behind-base"]
}
```

## Decisions

**1. Fixed attention vocabulary, derived by documented rules.** An agent and a
human must read the same flags. Rules are plain predicates over the facts above
(`ahead > 0` is `unpushed`, no upstream is `no-upstream`, and so on). No scoring,
no weighting. The list is part of the spec so it cannot drift silently.

**2. Request facts through the provider; GitHub does one `gh` call per repo, matched by head branch.** The `where` script that
motivated this already does this and caches for ten minutes; per-PR calls are
avoided because a feature may have dozens of branches. If the head branch is
absent from the first 100 results, the row simply has no `pr` block; a status
line says the list was truncated.

**3. Two commands: `status` and `overview`.** The earlier name, `topology`, describes the data
structure (a join across stores), not what a person wants. It is unreleased, so it is renamed
outright with no alias.

- `status` is relative to where you stand: resolve the current worktree to its branch, then to its
  change through naming or the request that closes the issue, and show it with everything related.
  If no change is bound, show the branch's request and local facts and say there is no change.
- `overview` is global and scopable. It is specified in `work-relations-and-overview`.

Two commands with distinct jobs read better than one command with many scopes. The names answer
"where am I?" and "what is going on?". Alternatives considered: `where` (a separate skill, about
location), `board` (already the project board), `work`, `list`.

**4. Size is computed locally.** `local.size` is `git diff --shortstat` and a commit count against the merge base, so the size of any branch is known without a tracker. `pr.size`, when a provider supplies it, is shown alongside and may differ for pushed versus local commits. The size advisory uses `local.size`.

**5. Behind-base needs a base.** Base is the PR's base branch when a PR exists,
otherwise the repository default branch from `git symbolic-ref
refs/remotes/origin/HEAD`. If neither resolves, the field is omitted, not
guessed.

**6. `volatilePath` reuses the claims proposal's definition** (system temp
directories). It is a fact about the path, not a judgement, and is the cheapest
signal for "this work can vanish".

## Freshness and sync advice

Facts (all read-only, from git and the provider):

- `behindUpstream`, `ahead`, `diverged` (both non-zero) for the branch's own upstream.
- `behindBase` and `baseMovedAt` (time of the newest base commit not in the branch).
- `overlap`: files changed on the branch and also on the base since the merge base.
- `conflicts`: paths that would conflict, from `git merge-tree --write-tree <base> <head>`. It
  computes the merge in the object store and never touches the index or worktree. Omitted when git
  is too old; the row then says the check was unavailable.
- `shared`: the branch has commits by more than one author.
- `reviewStarted`: the request has any review or comment (provider fact).

Advice is a mechanical table over those facts. First match wins:

| Facts | Suggestion |
|---|---|
| dirty tree | commit or stash first; nothing else is suggested |
| only `behindUpstream` | fast-forward the branch |
| `diverged` | rebase local commits onto the upstream; no force push needed |
| `behindBase`, `shared` | merge the base; never force |
| `behindBase`, `reviewStarted` | merge the base (keeps review comments attached); configurable |
| `behindBase`, otherwise | rebase onto the base; force push with lease if already pushed |
| up to date | nothing |

The strategy and the after-review behaviour come from `sync:` in `openspec/specsync.yml`, with the
defaults above. Every suggestion lists exact commands and says if a force push is involved. The
commands are text; applying them is the job of `specsync update` (see `flow-verbs`).

## Risks

- `gh` rate limits on large workspaces. Mitigated by the single call per repo, the
  TTL cache, and `-state=off`.
- `mergeable` is often `UNKNOWN` right after a push because GitHub computes it
  lazily. It is reported as-is; no retry loop.
- A force-pushed or rebased branch changes `ahead/behind` meaning. The numbers are
  correct for the current refs; we do not try to explain history rewrites.

## Open questions

- Should `size` include generated or lockfile paths? Proposal: report what `gh`
  reports; filtering is a reviewer concern.
- Should `cleanup` also require the remote branch to be deleted? Proposal: no,
  keep it to "PR merged and tree clean" and let the human decide.
