# Tasks

## Slice 1: Path template
- contract: One resolver turns the config, repo, issue and slug into a worktree path. The default is unchanged.
- budget: files<=8, lines<=300

- [x] 1.1 `worktrees.path` in the config: parse, validate placeholders and the `{slug}` or `{name}` rule
- [x] 1.2 A resolver for the path, with `{name}` as `<issue>-<slug>` or `<slug>`
- [x] 1.3 `start` uses the resolver; the Orca name follows the template
- [x] 1.4 Tests: default layout, nested layout, issue and no issue, bad template, Orca name

## Slice 2: Pull uses it
- after: 1
- contract: `pull -worktree` reads the `worktrees:` config and creates the same worktree as `start`.
- budget: files<=8, lines<=300

- [x] 2.1 `pull -worktree` takes the resolver, branch pattern and `dir` from the config
- [x] 2.2 `-worktree-dir` and `$SPECSYNC_WORKTREE_DIR` still win over the config `dir`
- [x] 2.3 Tests: pull then start finds one worktree; flag and env override; no config is unchanged

## Slice 3: Docs
- after: 2
- contract: The key is documented once, and the feature list shows it.

- [x] 3.1 `docs/flow-verbs.md` and the generated help name `worktrees.path`
- [x] 3.2 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`
