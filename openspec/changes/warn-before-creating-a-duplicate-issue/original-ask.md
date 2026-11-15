# Say when a sync is about to open a new issue, and what it might duplicate

## Why

`specsync -change <slug>` opens a new issue whenever the change has no ref yet. It never looks at
what is already open in the target repo, and its output gives the reader no way to notice.

Reproduced while working an incident: a change was written for remediation work, synced, and opened
a new issue — while the issue that had tracked that same work for two days sat open in the same
repo. The duplicate was caught by a human reading the board, not by the tool.

Two things made it easy to walk into.

**1. `-dry-run` uses the same word as a real run.** The preview ends:

```
  created  https://github.com/<owner>/<repo>/issues/0  [github:owner/repo] (my-change)
specsync: 1 created, 0 updated
```

`created` is also what an update-path preview prints for the *other* branch of the same `if`, so the
line scans as routine projection output. The one detail that distinguishes a create — `issues/0`,
the placeholder for a number that does not exist yet — is the least conspicuous thing on the line.
Nothing says *this will open a new issue*.

**2. Nothing looks for prior art.** `sync.go` decides with `created := !hadRef` — purely "do we have
a ref cached for this change". The CLI already has `scan` (find related work) and `adopt` (bind a
change to an existing issue), so the pieces exist; nothing routes you to them at the one moment they
matter, which is immediately before a create.

## What

### Make the create path unmistakable in a dry run

On the create path, `-dry-run` should name what it is about to do rather than describe the result in
the past tense. Something that cannot be mistaken for an update — a distinct verb, and the absence
of a real issue number stated rather than implied.

### Offer prior art before creating

Before a create, list open issues in the target repo and surface the plausible matches, with the
`adopt` command spelled out:

```
  NEW ISSUE  (no issue exists yet)  [github:owner/repo] (my-change)
    2 open issues may already cover this:
      #128  Widget sync starves under load          (shared: widget, sync)
      #131  Widget totals double-counted            (shared: widget)
    → bind instead with:  specsync adopt -issue 128 -change my-change
```

Matching should be on **shared distinctive words**, not title similarity. This matters and is worth
stating plainly: in the case that prompted this, the two titles shared exactly one word. Two people
describing the same problem rarely phrase it alike, and a similarity score high enough to avoid false
positives would have scored that pair near zero. Shared jargon — a subsystem, a source system, a
component — is a far better signal than string distance.

This is advisory. It prints and proceeds; it does not block, and it does not need to be right. A
list that is wrong costs the reader two seconds. A duplicate costs a cleanup.

## Non-goals

- Blocking or refusing a create. Duplicate detection is a heuristic and must never gate a sync.
- Changing `adopt` or `scan`. Both work; they are simply not reachable from where the mistake happens.
- Semantic or embedding-based matching. A stopword list and a word intersection is enough for a hint,
  and it stays offline.
