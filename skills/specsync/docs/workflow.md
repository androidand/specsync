# Workflow Patterns

## One Issue, One Branch, One Worktree

Before starting *any* change below — spec-first or issue-first — put it on
its own branch, named after the issue once one exists (`feat/<n>-<slug>`; use
a working name and rename once `sync`/`pull` mints the issue if starting
spec-first). Don't commit two unrelated changes to the same branch, and don't
work directly on the main branch. A dedicated worktree avoids collisions when
more than one change is in flight at once:

```bash
git worktree add ../worktrees/my-change -b feat/42-my-change
# ... implement here ...
git worktree remove ../worktrees/my-change   # once merged
```

On the issue-first path, `specsync pull -issue 42 -worktree` sets both up for
you in one step (see `pull` in [reference.md](reference.md)). There's no
equivalent flag for spec-first yet — branch/worktree by hand as above.

## Spec-First (Plan → Issue)

When starting from a spec idea:

1. Create the change directory:
   ```bash
   mkdir -p openspec/changes/my-change
   ```

2. Write proposal and tasks:
   ```bash
   # openspec/changes/my-change/proposal.md
   # openspec/changes/my-change/tasks.md
   ```

3. Preview the issue:
   ```bash
   specsync -dry-run -change my-change
   ```

4. Create the GitHub issue:
   ```bash
   specsync -change my-change
   ```

5. Implement and sync updates:
   ```bash
   # ... write code, check off tasks ...
   specsync -change my-change
   ```

## Issue-First (Issue → Spec)

When starting from an existing GitHub issue:

1. Pull the issue into a local change:
   ```bash
   specsync pull -issue 42 [-change my-change]
   ```
   Creates `proposal.md` and `tasks.md` locally.

2. Refine the spec:
   - Edit `proposal.md` to clarify intent
   - Refine `tasks.md` with actual implementation tasks
   - Add scope notes to the body

3. Preview and sync:
   ```bash
   specsync -dry-run -change my-change
   specsync -change my-change
   ```

4. Proceed with implementation

## Adopting an Already-Existing Pair

When a change and its issue both already exist independently — someone filed
the tracker issue by hand while someone else wrote the OpenSpec change — and
nothing links them:

1. Bind them explicitly:
   ```bash
   specsync adopt -issue 3691 -change my-change
   ```
   Writes `.specsync/refs.json` *before* touching the issue body, so the link
   holds immediately.

2. Never hand-edit the `<!-- specsync:change=<slug> -->` marker into the issue
   body and hope the next sync finds it — marker search lags writes by
   seconds to minutes, and a sync inside that window creates a *duplicate*
   issue instead of finding the real one.

3. If `sync` reports an ambiguous-marker or closed-issue error, that is the
   guard doing its job (not a bug): resolve it with `specsync adopt` rather
   than `-force`, unless you're certain the cached ref is right.

## Implementation Phase

During active work:

1. Check off tasks as you complete them:
   ```bash
   # openspec/changes/my-change/tasks.md
   - [x] Task 1
   - [ ] Task 2
   - [x] Task 3
   ```

2. Sync to update the GitHub issue:
   ```bash
   specsync -change my-change
   ```
   specsync auto-reconciles task state:
   - GitHub checkboxes that were checked locally remain checked
   - Local task additions appear on GitHub
   - The monotonic union means lagging remotes never uncheck work

3. Capture discoveries as you go:
   ```bash
   specsync note -change my-change "Found edge case in X, needs follow-up"
   ```
   These appear in the GitHub issue under "Discoveries" section.

## Multi-PR Implementation

For phased changes across multiple PRs:

1. Ensure `tasks.md` accurately reflects incomplete work:
   ```bash
   # Still has unchecked tasks
   - [ ] Phase 2: optimize database queries
   ```

2. Get the right PR reference:
   ```bash
   specsync pr-body -change my-change
   # Output: Part of #42
   ```

3. Use in PR body:
   ```bash
   gh pr create --title "Phase 0: initial schema" --body "$(specsync pr-body -change my-change)\n\nImplements..."
   ```

4. When final PR completes all tasks:
   ```bash
   # All tasks checked
   specsync -change my-change
   # Now issue will auto-close (if -close-completed was used on original sync)
   ```

## Completion & Archival

When all work is done:

1. Ensure all tasks are checked:
   ```bash
   # openspec/changes/my-change/tasks.md
   - [x] Task 1
   - [x] Task 2
   ```

2. Final sync:
   ```bash
   specsync -change my-change
   ```
   Issue stage transitions to `complete` automatically.

3. Archive the change:
   ```bash
   openspec archive my-change -y
   ```
   Moves to `changes/archive/` and issues get `stage:archived` label.

## Cross-Linking Changes

When related changes need to reference each other:

1. Link them locally:
   ```bash
   specsync link my-change related-change
   ```
   Creates `links.md` in each, with "## Related" section.

2. Sync both changes:
   ```bash
   specsync -change my-change
   specsync -change related-change
   ```
   GitHub issues now have mutual "## Related" sections.

## Coordinating Cross-Repo Work with an Epic

The canonical scenario: "We want feature X. It needs Y added to the backend repo and Z to use it in the frontend repo." One command mints the coordination issue and wires every child to it, in one step:

```bash
specsync epic "Feature X: cross-repo widgets" --repo owner/planning \
  --child owner/backend#12 \
  --child frontend-widget-view   # a local change slug in this repo's openspec tree
```

This creates one `type:epic` issue in `owner/planning`, attaches `owner/backend#12` and the frontend change's issue to it (syncing the frontend change first if it has no issue yet), and edits each child's body with a `## Related` backlink to the epic. Re-running with the same title and children converges instead of duplicating — safe to run again after adding another `--child`.

Once `epic-and-subissue-projection` lands, the same command attaches children as native GitHub sub-issues instead of the `## Related` fallback — no change to how you call it. Once `issue-dependency-sync` lands, add `--blocked-by owner/backend#12` on a child to record real dependency direction between children of the same epic.

## Finding Related Work

Before starting new work:

1. Search by file/path:
   ```bash
   specsync scan src/auth/ "SSO login"
   ```

2. Get JSON for analysis:
   ```bash
   specsync scan -json src/auth/ | jq '.'
   ```

3. Review existing changes:
   ```bash
   specsync changes --json | jq 'group_by(.stage)'
   ```

4. Scanning across sibling repos checked out locally (not a remote fetch):
   ```bash
   specsync scan -openspecs ../backend/openspec,../frontend/openspec src/auth/
   specsync scan -references src/auth/   # also show declared openspec/config.yaml references
   ```
   specsync has no concept of one shared spec repo that other projects pull
   from instead of keeping their own `openspec/` — every directory named here
   must be a real local checkout.

## Handling Blockers

When work is blocked:

1. Mark the change as blocked:
   ```bash
   specsync set-stage my-change blocked
   ```

2. Add reason in tasks.md:
   ```bash
   ## Tasks
   - [ ] Task 1
   - [ ] Task 2 (blocked: waiting for @person to review design)
   ```

3. Sync to update the issue:
   ```bash
   specsync -change my-change
   ```

4. Resume when unblocked:
   ```bash
   specsync set-stage my-change active
   specsync -change my-change
   ```

## Release Workflow

Before cutting a release:

1. Ensure all merged changes are archived:
   ```bash
   specsync release-plan
   ```
   Shows changes since last release and any missing archives.

2. Generate release notes:
   ```bash
   specsync changelog -since v1.0.0 -until v1.1.0
   ```

3. Append to CHANGELOG.md and tag:
   ```bash
   git tag v1.1.0
   git push origin v1.1.0
   ```
