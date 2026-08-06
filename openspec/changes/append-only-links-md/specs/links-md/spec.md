# links-md

## ADDED Requirements

### Requirement: Never rewrite authored links.md content
specsync SHALL treat `links.md` as append-only. When recording a link, it SHALL
preserve every existing byte of the file — prose, headings, sequencing notes,
`## Blocked by` / `## Blocks` sections, and entries specsync did not write — and
add only the entries not already recorded. specsync SHALL NOT remove an entry from
`links.md` on the basis that a ref is absent from the set it is recording;
removal is the author's edit to make.

#### Scenario: Authored prose and dependency order survive a link
- **WHEN** `links.md` contains sequencing prose and a `## Blocked by` section, and
  `specsync link my-change owner/repo#99` runs
- **THEN** the prose and the `## Blocked by` section are unchanged
- **AND** `- owner/repo#99` is added to the file

#### Scenario: A re-pull does not flatten the file
- **WHEN** a change with an authored `links.md` is re-pulled and the issue's
  `## Related` section lists two URLs
- **THEN** the authored content is unchanged
- **AND** only URLs not already recorded are added

#### Scenario: A locally removed entry is not resurrected by link
- **WHEN** the author deletes an entry from `links.md` and runs `specsync link`
  for an unrelated pair
- **THEN** the deleted entry is not written back by that invocation

### Requirement: Deduplicate by resolved ref
specsync SHALL decide whether a ref is already recorded by resolving the file's
existing entries, not by string-matching them. A full issue URL, its
`owner/repo#N` shorthand, and a sibling slug whose ref cache resolves to that
issue SHALL all count as the same link.

#### Scenario: Shorthand is not added beside its URL
- **WHEN** `links.md` already contains `- https://github.com/owner/repo/issues/42`
  and specsync records the ref for issue 42
- **THEN** no `- owner/repo#42` line is added

#### Scenario: Provider key spelling does not split one link in two
- **WHEN** the recorded ref carries a bare `github` provider and the file's entry
  resolves to the qualified `github:owner/repo` form for the same issue
- **THEN** they count as one link and no duplicate entry is added

#### Scenario: A sibling slug already counts as recorded
- **WHEN** `links.md` contains `- other-change` and that sibling's ref cache
  resolves to `owner/repo#42`, and specsync records the ref for issue 42
- **THEN** the file is unchanged
- **AND** the rendered `## Related` section lists that issue once

### Requirement: Recording an already-linked ref writes nothing
When every ref being recorded is already present, specsync SHALL leave
`links.md` byte-for-byte unchanged and perform no write, so repeat runs produce no
diff. When there are no refs to record and no `links.md` exists, specsync SHALL
NOT create one.

#### Scenario: Second link run is a no-op on disk
- **WHEN** `specsync link` runs twice on the same arguments
- **THEN** the second run leaves `links.md` identical to after the first

#### Scenario: A change with no links stays clean
- **WHEN** a spinoff records no parent link
- **THEN** no `links.md` is created

### Requirement: Preserve the plain-list shape for new files
When no `links.md` exists, specsync SHALL write a plain list of `- owner/repo#N`
entries (falling back to the bare URL when the shorthand cannot be derived), with
no section heading. The append-only rule SHALL NOT impose structure on changes
that have none.

#### Scenario: From-scratch output is a bare list
- **WHEN** specsync records two refs for a change with no `links.md`
- **THEN** the file is exactly the two `- owner/repo#N` lines

#### Scenario: An unterminated last line is not glued to the new entry
- **WHEN** the existing `links.md` does not end in a newline
- **THEN** the appended entry starts on its own line
