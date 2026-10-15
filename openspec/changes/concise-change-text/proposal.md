# Keep issue and PR text short, with a standard

## Why

Issues and PRs grow long. Headers multiply. Sentences run on. Reviewers skim, miss the point or give up.

specsync writes issue bodies and PR fragments. It can keep them short and the same everywhere.

## Standard

Defaults. All configurable.

- **PR and commit titles:** Conventional Commits. `type(scope): subject`. Imperative. 72 characters at most.
- **Issue titles:** the outcome, in sentence case. 60 characters at most. No tool or file names.
- **Headers:** from a short set. `Why`, `What`, `Out of scope`, `Related`. Three words at most. Five headers at most.
- **Sentences:** 20 words at most. Paragraphs 4 lines at most. Prefer bullets.
- **Length:** the visible part of an issue, 150 words. A PR body, 200 words. Links and generated parts do not count.
- **No filler.** A short list of phrases to avoid. Configurable.

Sources: Conventional Commits 1.0, git's 50/72 rule, Keep a Changelog (already used for the changelog).

## What

- `specsync lint`: check a change's proposal, and optionally PR text, against the standard. Advisory. Shows line, rule, reason and a suggestion. `-fail` for CI.
- `specsync pr-body -template`: render the PR body from the change. Sections: Summary, Changes, Review focus, Size, Verification, Closes. Each is short.
- Per-repo choice in `openspec/specsync.yml`: `text.profile`, limits, header set, filler list, and a PR template path. A repo's own `.github/pull_request_template.md` is used when present.
- The synced issue keeps its visible part short. The rest stays in collapsed sections, as today.

## Out of scope

- No rewriting of text. It reports and suggests.
- No model or grammar checking. Rules are counts and lists.
- It builds on `advisory-title-suggestions`. It does not replace it.

## Related

- `flow-verbs`: `pr open` uses the generated body.
- `pr-size-advisory`: the Size section.
- `review-map-for-prs`: the Review focus section.
- `ways-of-working-and-next`: the config file.

## Release note

Add `specsync lint` and a short PR body template. Titles follow Conventional Commits. Issues and PRs stay brief and consistent.
