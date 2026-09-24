# Design: reject-unconsumed-cli-args

## Shadowed-flag matching strategy

Exact flag-name matching only — no typo-fuzzing (e.g. `-dryrun` is not
recognized as a near-miss of `-dry-run`). Typo detection is a separate,
fuzzier problem and conflating it with this check would let false positives
undermine trust in the new guard.

## Arity lookup

`shadowedFlags` walks the subcommand's `flag.FlagSet` via `fs.VisitAll` and
classifies each registered flag as boolean or value-consuming by checking
for the `IsBoolFlag() bool` method Go's own `flag` package uses internally
(`fs.Bool` values implement it; `fs.String`/`fs.Var` values generally don't).
This mirrors how `flag.Parse` itself decides whether a token is a bare flag
or a flag plus a following value token, so the scan doesn't misidentify a
value flag's argument (e.g. the `owner/repo` after `-repo`) as the first
positional token.
