# Harden the npm binary installer

The npm wrapper currently exits successfully when the release binary cannot be
downloaded or extracted. npm then reports a successful installation even though
the `specsync` command cannot run. The downloaded archive is also not checked
against the release checksum file.

Make installation fail clearly when a supported platform cannot obtain a usable
binary, verify the archive before extraction, and test the installer without
real network access.

## Decisions

- Unsupported platforms keep a clear Go-install fallback.
- Supported platforms fail installation when download, verification, or
  extraction fails.
- Verify the selected archive against the release `checksums.txt` entry.
- Inject or isolate network/filesystem/process operations for deterministic tests.

## Non-goals

- Adding Windows binaries.
- Replacing npm with another distribution channel.
