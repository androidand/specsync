# Cross-repo linked issues

Large features often span multiple GitHub repos (e.g. a backend service change + a frontend view + a shared component library migration). Each repo needs its own issue, but the issues should visibly reference each other so reviewers and agents always have context.

Today specsync is strictly 1:1: one slug → one issue in the auto-detected repo. There is no way to target a different repo or to record that two specs are related.

## Solution

**`-repo` flag on sync**: `specsync -change <slug> -repo owner/name` creates or updates an issue in an explicit repo instead of the git-remote-detected one. The ref cache key becomes `github:owner/name` (instead of `github`) so a single spec can accumulate refs across multiple repos without conflict.

**`specsync link <slug1> <slug2>`**: After both issues exist, this command writes `.specsync/links.json` in each change directory (recording the other's issue URL), then syncs both specs. The next push renders a `## Related` section in each issue body, pointing at the other issue by URL. Fully idempotent and re-runnable.

**`## Related` section**: Managed by specsync in the issue body (like `## Tasks`). Stripped on pull so it never pollutes the local proposal.md. Rebuilt from `links.json` on every push.

## Agent workflow for the target test prompt

```
# Agent has read the PR context and decided to create two linked follow-up issues

mkdir -p openspec/changes/atomic-design-widget-app
# write proposal.md and tasks.md for widget-app work
specsync -change atomic-design-widget-app -repo org/widget-app   # creates widget-app issue

mkdir -p openspec/changes/atomic-design-dashboard
# write proposal.md and tasks.md for dashboard cleanup
specsync -change atomic-design-dashboard                              # creates Dashboard issue

specsync link atomic-design-widget-app atomic-design-dashboard          # cross-links both
```

Both GitHub issues now reference each other and share enough context for any engineer (or agent) to find the sibling issue.

## Provider agnosticism

`-repo` and link semantics are in the Go package layer. The GitHub provider passes `--repo` to `gh`. Future providers (Linear, Jira) implement `SetRepo` or equivalent concept — the core `WorkProvider` interface stays the same.
