# release-tool-detection

## ADDED Requirements

### Requirement: Detect the project's release tool by filesystem evidence
specsync SHALL detect which release tool a project uses by probing for common
tools' marker files (release-please, changesets, release-it, semantic-release,
standard-version), reporting a custom flow or none when no marker matches, and
SHALL report the evidence it found. The detector is intentionally light; an
unrecognized tool is reported as custom rather than treated as an error.

#### Scenario: Changesets detected
- **WHEN** a `.changeset/` directory with a `config.json` exists
- **THEN** specsync reports changesets as the detected release tool
- **AND** lists the evidence paths it matched

#### Scenario: No release tool
- **WHEN** no release-tool markers are present
- **THEN** specsync reports that no release tool was detected
- **AND** its bump recommendation remains advisory

### Requirement: Report responsibilities and defer to the tool
specsync SHALL report which responsibilities the detected tool owns (version
bump, tag, changelog, publish) and SHALL NOT perform those responsibilities
itself.

#### Scenario: Deferring version bump and tagging
- **WHEN** a release tool is detected
- **THEN** specsync reports that the tool owns bumping and tagging
- **AND** specsync does not bump the version or create a tag

### Requirement: Never invoke the detected tool
Detection SHALL be limited to reading the filesystem; specsync SHALL NOT import
or execute the release tool.

#### Scenario: Detection is inert
- **WHEN** detection runs
- **THEN** no release-tool command is executed and no dependency on it is added

### Requirement: Position standard-version and changesets
specsync SHALL report `standard-version` as a legacy/ad-hoc workflow rather than
a recommended path, and MAY note that `changesets` aligns conceptually with
OpenSpec because both record release intent.

#### Scenario: Standard-version labelled legacy
- **WHEN** standard-version is detected
- **THEN** the report labels it as legacy/ad-hoc
