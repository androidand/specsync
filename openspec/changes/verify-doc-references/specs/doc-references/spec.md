# doc-references

## ADDED Requirements

### Requirement: Detect commit references that do not resolve
`specsync verify -refs` SHALL scan the markdown of the change folders and any given paths
for commit hashes and SHALL report each hash that does not resolve in the repository as
`stale`, with file and line.

#### Scenario: Rebased branch
- **WHEN** a document cites a commit hash that no longer exists after a rebase
- **THEN** the hash is reported as stale with its file and line

### Requirement: Detect commits that are no longer on the branch
specsync SHALL report a hash that resolves but is not reachable from the default branch or
the change's branch as `orphaned`, showing the commit's subject.

#### Scenario: History regrouped
- **WHEN** a cited commit still exists but only on a discarded line of history
- **THEN** it is reported as orphaned with its subject

### Requirement: Match references precisely
specsync SHALL consider a token a commit hash only when it is 7 to 40 hexadecimal
characters, stands alone as a word, and is written as inline code or follows the word
"commit", and SHALL ignore other tokens without reporting them.

#### Scenario: Ordinary hex-looking word
- **WHEN** a document contains the word `defaced` outside code
- **THEN** it is not treated as a hash and nothing is reported

### Requirement: Check request and issue references through the provider
specsync SHALL check `#n` and `owner/repo#n` references through the work provider and report
those that do not exist and the state of those that do, and SHALL skip this check with a
status line when the provider cannot answer.

#### Scenario: Provider unavailable
- **WHEN** the provider's tool is missing
- **THEN** hash checks still run
- **AND** a status line says request checks were skipped

### Requirement: Never rewrite or guess
specsync SHALL NOT modify any document and SHALL NOT propose replacement references by
similarity.

#### Scenario: Stale hash with a similar commit present
- **WHEN** a stale hash has a commit with a similar subject on the branch
- **THEN** only the stale hash is reported
- **AND** no replacement is suggested

### Requirement: Advisory by default
`verify -refs` SHALL exit 0 when it finds problems unless `-fail-on-stale` is given, in
which case it SHALL exit non-zero.

#### Scenario: CI gate
- **WHEN** `-fail-on-stale` is set and a stale hash exists
- **THEN** the command exits non-zero
