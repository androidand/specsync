# Safety Rules & Best Practices

## Core Rules

1. **Always dry-run before GitHub writes**
   ```bash
   specsync -dry-run -change <slug>
   ```
   This shows the exact GitHub commands and issue body without making changes.

2. **Always pass `-change` when one change is in scope**
   - Without `-change`, specsync syncs **every change**
   - Forgetting this is the most common mistake
   ```bash
   # Right
   specsync -change my-change

   # Wrong (syncs all changes!)
   specsync
   ```

3. **Confirm git remote points to the right repo**
   - specsync auto-detects the repo from `git remote`
   - On unsure machines, pass `-repo owner/name` explicitly
   ```bash
   specsync -dry-run -change my-change -repo androidand/specsync
   ```

4. **Never commit `.specsync/` cache directories**
   - `.specsync/` is auto-generated and machine-local
   - Add to `.gitignore` if not already

5. **Never put credentials or secrets in issue bodies**
   - Specs and issues are public-facing
   - Use environment variables, vaults, or secure config

6. **Always include issue references in PR bodies**
   - Use `specsync pr-body -change <slug>` to generate the right reference
   - `Part of #N` while work remains
   - `Closes #N` only when all tasks complete

7. **Use `adopt` to bind an existing change to an existing issue — never hand-edit the marker**
   - See [Adopting an existing issue](#adopting-an-existing-issue) below

## Adopting an existing issue

A change and its issue can both already exist independently — someone files the
tracker issue by hand while someone else writes the OpenSpec change — with
nothing linking them. Hand-editing the `<!-- specsync:change=<slug> -->` marker
into the issue body and hoping the next sync finds it has a real race: marker
discovery goes through the tracker's search index, which lags behind writes by
seconds to minutes. A sync inside that window finds nothing and creates a
*duplicate* issue — precisely the outcome the marker was meant to prevent. This
happened for real: a hand-added marker raced GitHub's index, a sync seconds
later created a duplicate issue, and a later sync then wrote to the closed
duplicate, reporting success.

```bash
specsync adopt -issue <N> [-change <slug>] [-dry-run] [-force]
```

`adopt` writes `.specsync/refs.json` *before* touching the issue body, so the
link holds immediately regardless of indexing. It refuses when the change is
already bound to a different issue, or the issue already carries a marker for
a different change, unless `-force`.

Two related guards protect `sync` itself, and both can now reject a flow that
previously appeared to "succeed" — because it was succeeding while writing to
the wrong issue:

- **Ambiguous marker match**: if marker search returns more than one issue,
  `sync` stops and names `adopt` instead of silently picking the first hit.
- **Closed issue**: `sync` refuses to write to a closed issue unless `-force`
  — a closed issue is the strongest available signal that a cached ref is
  stale. (This guard only applies to a plain content push; `-close-completed`'s
  own reopen/defer logic is unaffected.)

## Task State Reference

specsync recognizes four task states in `tasks.md`:

- `[ ]` — todo (unchecked)
- `[x]` — done (checked)
- `[~]` — dropped (superseded, removed, won't implement)
- `[>]` — moved (relocated to another change, task linked elsewhere)

Progress calculation: only `[ ]` and `[x]` count as live tasks.

On GitHub issues, specsync shows: `+N added · X done · Y dropped · Z moved`

## Workflow Order

1. Create change: `openspec/changes/<slug>/` with `proposal.md` and `tasks.md`
2. Preview sync: `specsync -dry-run -change <slug>`
3. Real sync: `specsync -change <slug>` (creates GitHub issue)
4. Implement: Commit code, check off tasks in `tasks.md`
5. Update tracker: `specsync -change <slug>` (reconciles task state)
6. Complete: All tasks checked → `specsync -change <slug>` → `openspec archive <slug>`

## Stage Meanings

- **backlog**: Ready to start, but not started
- **active**: Currently being worked
- **blocked**: Waiting on external dependency (blocked, but not abandoned)
- **in-review**: Under review (don't assign new work)
- **complete**: All tasks done, ready to archive
- **archived**: No longer active (in `changes/archive/`)

## Board Reconciliation Safety

specsync uses three-way merge to prevent clobbering human edits:

1. Read local stage from `.specsync/metadata.json`
2. Query GitHub for current status
3. Compare against last-synced state in `.specsync/board.json`
4. If human moved card on board: **skip update, preserve human edit** ✅
5. If local changed, remote didn't: **push update to board** ✅
6. If both changed: **log conflict, skip, await human review** ⚠️

**Never fight the board.** If you see `StatusSkipped: human moved the card on the board`, respect it—don't retry.

## Provider Safety

specsync auto-detects the work provider (GitHub or Beads):

- **GitHub** (default): Always assumed unless `.beads/` directory exists AND `bd` is on PATH
- **Beads** (auto-detect only): Requires `.beads/` directory + `bd` CLI installed

If auto-detection chooses unexpectedly:
```bash
specsync -provider github -dry-run -change <slug>
```

Pass `-provider` explicitly rather than letting mistakes create items in the wrong tracker.

## Title Hygiene

specsync never rewrites titles. Instead, it prints suggestions:

```
title could be tighter: "Migrate to Postgres 17 pgx/v6 driver (rewrite ~450 call sites)"
→ Suggestion: "Migrate to Postgres 17"
```

**Good titles** answer "what will be different?" without implementation detail.

- ✅ "Migrate to Postgres 17"
- ✅ "Fix login SSO reuse"
- ❌ "Migrate to Postgres 17 pgx/v6 driver (rewrite ~450 call sites)"

Edit the proposal H1 yourself if you agree. specsync will use your version on the next sync.

## Dogfooding Enforcement

This repo's CI enforces:

1. **Archive hygiene**: Completed changes moved to `changes/archive/`
2. **Changelog linking**: Every commit references its change's issue (`(#N)`)
3. **Task dogfooding**: Task checklist status matches code changes
4. **Structure validation**: All changes follow spec format

These gates prevent quality decay.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Syncing every change by accident | Always pass `-change <slug>` |
| Writing to GitHub without dry-run | Always use `-dry-run` first |
| Stale task checklists | Keep `tasks.md` current while implementing |
| Missing PR issue references | Use `specsync pr-body -change <slug>` in PR body |
| Forgetting to archive completed changes | `openspec archive <slug> -y` at end |
| Putting secrets in issues | Never; use env vars or vaults |
| Hand-editing the marker into an issue body | Use `specsync adopt -issue <N> -change <slug>` instead |
