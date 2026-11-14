# Stamp local builds with the real version

## Why

`make build` and `make install` produce a binary whose version is `dev`, so a
checkout build cannot be told from another or from a release. A hand-rolled
`-ldflags` build that stamps a bare commit id (`specsync b0d6565`) is easy to
get wrong and masks which release the binary is based on.

## Proposed Changes

- The Makefile stamps `main.version` from `git describe --tags --always --dirty`
  (leading `v` dropped), so a build on a release tag reports `0.15.0` and one
  after it reports `0.15.0-3-gabc1234`. `VERSION=...` overrides it.
- Outside a git checkout the stamp is skipped and the binary stays `dev`.
- `make version` prints the stamp that would be used.

## Release note

`make build` and `make install` now report the real version, such as
`0.15.0-3-gabc1234`, instead of `dev`.
