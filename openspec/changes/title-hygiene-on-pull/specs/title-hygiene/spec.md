# title-hygiene

## ADDED Requirements

### Requirement: Clean issue titles on pull
specsync SHALL apply `shortenTitle()` to the issue title when pulling into an OpenSpec change, stripping parentheticals, backtick-enclosed tool names, and trailing detail words. The cleaned title becomes the `proposal.md` H1.

#### Scenario: Pull a verbose issue title
- **WHEN** `specsync pull -issue 3538` runs against an issue titled `"Design: resource-select flavor of the integration fields schema (credential → list resources → multi-create)"`
- **THEN** `proposal.md` H1 is `"Design: resource-select flavor of the integration fields schema"`
- **AND** the CLI prints `title cleaned: "Design: resource-select flavor of the integration fields schema (credential → list resources → multi-create)" -> "Design: resource-select flavor of the integration fields schema"`

#### Scenario: Pull a clean issue title
- **WHEN** `specsync pull -issue 4060` runs against an issue titled `"Portal 2026 Q3"`
- **THEN** `proposal.md` H1 is `"Portal 2026 Q3"` (unchanged)
- **AND** the CLI does not print a title-cleaning message

### Requirement: Report title cleaning to the user
When the issue title differs from the proposal H1, specsync SHALL report the before/after transformation so the user sees what was changed.

#### Scenario: Title was cleaned
- **WHEN** `shortenTitle()` returns a different title than the input
- **THEN** `PullResult.TitleCleaned` is true
- **AND** `PullResult.TitleBefore` contains the original title
- **AND** `PullResult.TitleAfter` contains the cleaned title
- **AND** the CLI prints the before/after in both dry-run and real mode

### Requirement: Idempotent cleaning
`shortenTitle()` SHALL be idempotent — applying it twice produces the same result as applying it once.

#### Scenario: Double cleaning
- **WHEN** `shortenTitle()` is applied to an already-cleaned title
- **THEN** the result is identical to the input
