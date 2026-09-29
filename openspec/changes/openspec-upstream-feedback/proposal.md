# Report the OpenSpec defects found while adding store support

## Why
Building store support surfaced three defects and one gap in OpenSpec 1.5.0,
all reproducible. They cost real debugging time and will cost every other
adopter the same, and the gap is one specsync is about to work around with a
private format — better to propose the standard first.

No behaviour of specsync changes here, so this change carries no spec delta.
It is tracked as a change rather than as loose issues because the fourth item
gates a design decision in openspec-store-support.

## What Changes
Nothing in this repository. Four reports filed upstream.

## Impact
- Affected specs: none — this is coordination work, not a behaviour change
