# Fix epic so its documented command line works

## Why

Every example of `specsync epic` puts the title first:

```
specsync epic "Feature X" --repo owner/planning --child owner/backend#12 --child frontend-slug
```

Go's flag parser stops at the first positional token, so in that form `--repo`, `--child` and
`--dry-run` are never parsed. The argument guard catches it and the command refuses to run, so
following the docs never works. An agent building a real epic gave up and built it by hand.

A second defect surfaced while checking the fix: a `--child` slug resolved to the wrong issue. The
change's local ref cache held a stray ref for another repository, and slug resolution took the
alphabetically first `github:` ref instead of the target the change's `specsync.yml` declares. The
same resolution backs `specsync link`.

## What

- `epic` accepts flags anywhere: before, between or after the title words. A bare `--` ends flag
  parsing, so a title word may start with a dash.
- `epic` refuses a title that contains one of its own flags. zsh does not word-split an unquoted
  `$args`, so `-child a -child b` can arrive as one argument; it used to become part of the title
  and the epic got no children. Words after `--` are taken as text without the check.
- A change slug given to `epic` or `link` resolves to the ref for a target its `specsync.yml`
  declares. When none of them has a ref, a stray ref for another repo is not used: `link` reports
  that the change has no synced ref and `epic` syncs it first. A change with no declared targets
  keeps the old choice.

## Non-Goals

- Interspersed flags for other commands. They keep flags-first parsing and the shadowed-flag guard.
- Changing ref choice for the other `firstRef` callers (sync, topology, spinoff, issue state).

## Release note

`specsync epic "Title" --child …` now works as documented, and a child slug always wires the
change's own issue rather than a stray cached ref.
