# concise-text

## ADDED Requirements

### Requirement: Lint titles against a standard
`specsync lint` SHALL check PR and commit titles against Conventional Commits and the configured
length, and issue titles against sentence case, outcome wording and the configured length, and
SHALL report the line, the rule, the reason and a suggestion.

#### Scenario: Title too long
- **WHEN** a PR title exceeds the limit
- **THEN** `lint` reports the rule and the limit

#### Scenario: Not Conventional Commits
- **WHEN** a PR title has no `type:` prefix under the conventional profile
- **THEN** `lint` reports it and shows the expected form

### Requirement: Lint headers and sentences
`lint` SHALL report headers outside the allowed set, headers longer than the limit, more headers
than allowed, sentences over the limit, and paragraphs over the line limit.

#### Scenario: Long header
- **WHEN** a header has more than three words
- **THEN** `lint` names it and the limit

### Requirement: Budget the visible length
`lint` SHALL count words in the visible part of an issue and in a PR body, excluding links and
generated sections, and report when the budget is exceeded.

#### Scenario: Over budget
- **WHEN** a PR body has 320 visible words and the budget is 200
- **THEN** `lint` reports the count and the budget

### Requirement: Render the PR body from the change
`specsync pr-body -template` SHALL render a body from the repository's template, or the default,
using the change's first Why sentence, What bullets, review focus, size, verification result and
closing references, and SHALL keep each section within its budget.

#### Scenario: Repo template exists
- **WHEN** the repository has a pull request template
- **THEN** its sections are used

### Requirement: Take the standard from repository config
specsync SHALL read the profile, limits, header set, filler list and template path from
`openspec/specsync.yml` and SHALL apply the defaults when absent.

#### Scenario: Plain profile
- **WHEN** the profile is `plain`
- **THEN** titles are not required to be Conventional Commits and length limits still apply

### Requirement: Advise, never rewrite
`lint` SHALL exit 0 by default, SHALL exit non-zero only with `-fail`, and SHALL NOT modify any
text.

#### Scenario: CI gate
- **WHEN** `lint -fail` finds a violation
- **THEN** it exits non-zero
