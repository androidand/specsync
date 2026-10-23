# links-md Specification

## Purpose
TBD - created by archiving change fix-links-append-section. Update Purpose after archive.
## Requirements
### Requirement: Appended links are recorded as related links
specsync SHALL record appended entries as related links: when the last section
heading of an existing `links.md` is not `## Related`, it writes a `## Related`
heading before the appended entries, so that `parseLinksMD` reads them as related
links. It SHALL NOT change any existing byte of the file.

#### Scenario: File ends with a dependency section
- **WHEN** `links.md` ends with `## Blocked by` listing `owner/repo#2`, and
  `specsync link` records `owner/repo#5`
- **THEN** `owner/repo#5` is parsed as a related link
- **AND** `## Blocked by` still lists only `owner/repo#2`

#### Scenario: File already ends with Related
- **WHEN** the last section of `links.md` is `## Related`
- **THEN** the new entry is appended without another heading

