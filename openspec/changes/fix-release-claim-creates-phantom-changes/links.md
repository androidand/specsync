# Links

## Related

- `in-flight-work-claims` (this repo) — the feature that added `release`/`claim`. This
  change fixes the guard on the commands that feature introduced; it does not touch what
  claim projects onto the tracker.
- `close-state-three-way-merge` (this repo) — shares the `mutableChange` guard path.
  Both changes rely on the same load-first, fail-on-missing-change contract.
