# npm installer requirements

## Requirement: Successful install means usable binary

On a supported platform, npm installation SHALL fail unless the matching
specsync binary is downloaded, verified, extracted, and made executable.

### Scenario: Release asset is unavailable

- **WHEN** the binary download returns a non-success response
- **THEN** installation exits non-zero
- **AND** the error identifies the release asset and manual fallback

### Scenario: Archive checksum differs

- **WHEN** the downloaded archive does not match `checksums.txt`
- **THEN** installation exits non-zero before extraction
- **AND** no binary is installed

### Scenario: Valid release artifact

- **WHEN** the archive checksum matches and extraction succeeds
- **THEN** the launcher finds an executable specsync binary
- **AND** temporary files are removed

## Requirement: Unsupported platform guidance

An unsupported OS or architecture SHALL produce a concise message identifying
supported alternatives and the `go install` fallback.
