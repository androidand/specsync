# Cap the length of generated slugs

## Context

`slugify` (`pull.go:485`) turns a title into kebab-case with no length limit at
all — it emits one hyphen-joined token per word for the entire input. Every
generated change name is therefore as long as whatever it was derived from.

Observed in real use: pulling ExopenGitHub/portal#4331, whose title is

> Multi-entity integrations: "Create all" has no selection UI, and company-name
> display is inconsistent across pages

produced the directory

```
openspec/changes/multi-entity-integrations-create-all-has-no-selection-ui-and-company-name-display-is-inconsistent-across-pages
```

98 characters, which then has to be typed for every `-change` flag, shows up in
every path in every log line, and had to be renamed by hand immediately. Compare
against the slugs this repo actually uses — `add-planning-scan`,
`add-worktree-support`, `add-agent-help-command` — all short, verb-led, and
hand-picked. The generated ones do not resemble them.

Three call sites are affected, in increasing severity:

- `pull.go:93` — issue title. Long titles are common; a descriptive issue title
  is good practice and is punished here.
- `epic.go:83` — epic title, prefixed `epic:`. Same problem.
- `spinoff.go:92` — **task text**. This is the worst case: task lines are full
  sentences, often with trailing detail, so spinoff slugs are longer still.

There is no test for `slugify` anywhere in the repo.

## Why not just pass `-change`

`pull` already accepts `-change`, and `slugFromMarker` can read a slug out of
the body, so a workaround exists. But the default path is what agents and
newcomers hit first, it produces something nobody would choose by hand, and
`spinoff` has no equivalent escape hatch for its per-task slugs.

## Proposed change

Cap generated slugs at a sane length, truncating on a word boundary rather than
mid-word, and never leaving a trailing hyphen.

Suggested shape, to be confirmed by whoever picks this up:

- A single `maxSlugLen` constant (proposal: 48 characters — long enough for
  `add-agent-help-command`-style names with room to spare, short enough to type).
- Truncate at the last hyphen at or before the cap, so words stay whole. If the
  first word alone exceeds the cap, hard-truncate it rather than emit nothing.
- Apply inside `slugify` so all three call sites benefit, rather than at each
  call site.
- `epic.go` prefixes `epic:` before the slug — decide whether the cap covers the
  prefix or only the slug portion, and be explicit about it.

## Collision risk

Truncation can make two distinct titles collide. Worth deciding deliberately:

- `pull` already handles an existing change directory somehow — check what it
  does today and whether truncation makes that path more likely, rather than
  assuming it is fine.
- A numeric suffix on collision (`-2`) is the obvious fallback, but only if the
  existing behaviour does not already cover it.

## Out of scope

Changing the slugs of changes that already exist. This only affects newly
generated ones.
