# title-hygiene

## ADDED Requirements

### Requirement: Titles are never rewritten
specsync SHALL treat titles as author content in both directions: `pull` writes the issue title verbatim as the `proposal.md` H1, and `sync` pushes the proposal H1 verbatim as the tracker item title. No command modifies a title automatically.

#### Scenario: Pull a verbose issue title
- **WHEN** `specsync pull -issue 7` runs against an issue titled `"Migrate to Prisma 7 \`prisma-client\` generator (rewrite ~450 imports)"`
- **THEN** the `proposal.md` H1 is that title verbatim

#### Scenario: Sync a verbose proposal H1
- **WHEN** `specsync` syncs a change whose H1 is `"Migrate to Prisma 7 \`prisma-client\` generator (rewrite ~450 imports)"`
- **THEN** the tracker item title is that H1 verbatim

### Requirement: Unwieldy titles get an advisory suggestion
When `shortenTitle()` would tighten a title, specsync SHALL surface the suggestion to the user — `ItemResult.TitleSuggestion` on sync, `PullResult.TitleSuggestion` on pull, and a printed `title could be tighter: "..."` line in both dry-run and real mode — without modifying the proposal or the tracker item.

#### Scenario: Verbose title synced
- **WHEN** `specsync` syncs a change whose H1 is `"Migrate to Prisma 7 \`prisma-client\` generator (rewrite ~450 imports)"`
- **THEN** `ItemResult.TitleSuggestion` is `"Migrate to Prisma 7 prisma-client generator"`
- **AND** the CLI prints `title could be tighter: "Migrate to Prisma 7 prisma-client generator"`

#### Scenario: Clean title synced
- **WHEN** `specsync` syncs a change whose H1 is `"Portal 2026 Q3"`
- **THEN** `ItemResult.TitleSuggestion` is empty and no title message is printed

#### Scenario: Archived change synced
- **WHEN** `specsync` syncs a change under `changes/archive/`, whatever its title
- **THEN** `ItemResult.TitleSuggestion` is empty — the archive is immutable by convention, so "edit the proposal H1" is not actionable

### Requirement: The suggestion transform is conservative
`shortenTitle()` SHALL strip balanced parenthetical asides and backtick characters (keeping the text inside backticks), collapse whitespace, and trim trailing punctuation. It SHALL NOT remove words by blacklist. It SHALL leave unbalanced-paren input untouched, and SHALL return the original title when cleaning would produce an empty string.

#### Scenario: Unbalanced parens
- **WHEN** `shortenTitle()` is applied to `"Fix smiley :( in parser"`
- **THEN** the title is returned unchanged

#### Scenario: Title that would clean to nothing
- **WHEN** `shortenTitle()` is applied to `"(everything in parens)"`
- **THEN** the original title is returned unchanged and no suggestion is reported

### Requirement: Idempotent cleaning
`shortenTitle()` SHALL be idempotent — applying it twice produces the same result as applying it once — so repeated pull/sync round-trips cannot erode a title.

#### Scenario: Double cleaning
- **WHEN** `shortenTitle()` is applied to an already-cleaned title
- **THEN** the result is identical to the input
