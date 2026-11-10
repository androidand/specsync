# Tasks

## Slice 1: Lint
- contract: `lint` reports title, header, sentence and length findings. It never edits.
- budget: files<=10, lines<=400

- [x] 1.1 Rules: title format and length, header set (including the names specsync writes or reads; slice headings exempt) and length, sentence and paragraph limits, filler list
- [x] 1.2 Word budget for the visible part, excluding links and generated sections
- [x] 1.3 `specsync lint [-change] [-pr|-body-file] [-fail]`
- [x] 1.4 Reuse `advisory-title-suggestions` for titles
- [x] 1.5 Tests per rule, advisory exit 0, `-fail` exit non-zero

## Slice 2: PR body template
- after: 1
- contract: The body renders short, from the change and the repo template.
- budget: files<=8, lines<=300

- [ ] 2.1 `pr-body -template` with sections Summary, Changes, Review focus, Size, Verification, Closes
- [ ] 2.2 Use `.github/pull_request_template.md` when present
- [ ] 2.3 Per-section budgets
- [ ] 2.4 Tests: default template, repo template, budgets

## Slice 3: Config and docs
- after: 1
- contract: A repo can choose its profile and limits.

- [ ] 3.1 `text:` section in `openspec/specsync.yml`; validated by `doctor`
- [x] 3.2 Declare `lint` once as a `Command` type; docs are generated
- [ ] 3.3 Update `site/features.json`; run `specsync changelog -fail-on-unlinked-commits`

## Slice 4: Dogfood
- after: 1
- contract: specsync's own proposals and PRs follow the standard and CI keeps them there.

- [ ] 4.1 Run `lint` on specsync's open proposals; condense those over budget, and list in specsync's own `text.headers` the extra names it uses on purpose (for example `Non-goals`, `Impact`) so the check passes without loosening the default

Depends on `ways-of-working-and-next` for the `text:` config.
- [ ] 4.2 Add a CI step that lints changed proposals and PR titles, advisory first, then `-fail`
