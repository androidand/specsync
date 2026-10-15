# Catch stale commit and request references in change documents

## Why

Change documents cite commits and change requests: "fixed in a1b2c3d", "see #123", "ships
with owner/repo#45". When a branch is rebased or its history regrouped, every cited commit
hash stops resolving or stops being on the branch, and the document keeps pointing at it.
In one real case a branch was rewritten twice in a day and every commit hash cited in the
planning documents went stale. It was noticed by chance, not by a tool, and a reader
following the references would have landed on nothing.

specsync already verifies that change requests reference their changes (`verify`). The
inverse, that what a change document references still exists and still means what it did,
is not checked, although it is cheap and deterministic: a commit either resolves in the
repository or it does not, and a change request either exists or it does not.

## What Changes

- **`specsync verify -refs [path...]`** scans the markdown of the change folders (and any
  paths given) for commit hashes and change-request references and reports:
  - a hash that does not resolve in the repository (`stale`);
  - a hash that resolves but is not reachable from the default branch or the change's branch
    (`orphaned`, typical after a history rewrite), with the commit subject so a human can
    find its successor;
  - a request or issue reference that does not exist, or whose state is merged or closed,
    reported as facts.
- **Precision over recall.** A hash is only considered when it is 7 to 40 hex characters,
  stands alone as a word, and is written as code or follows the word "commit", to avoid
  matching ordinary words. Unrecognised tokens are ignored, never flagged.
- **No guessing of successors.** The report shows the old reference's subject when
  available and lists nothing else; it never rewrites documents or pairs a new hash by
  similarity.
- **Exit status.** Advisory by default (exit 0); `-fail-on-stale` fails for projects that
  want a CI check.

## Generality

- Hash checks use only git, so they work in any repository. Request and issue checks go
  through the work provider and are skipped, with a status line, when it cannot answer.
- Works on any markdown, so it covers change documents, runbooks and release notes alike.

## Non-goals

- It does not check numbers, counts or claims written in prose. A figure that has gone stale
  cannot be detected mechanically; the remedy there is to cite the query that produces it.
- No automatic fixing and no document rewriting.

## Impact

- A reference scanner and resolver over existing git and provider calls; one flag on
  `verify`.
- Docs: README, `agent-help`, the skill, and `site/features.json`.

## Release note

`specsync verify -refs` finds commit hashes and request references in your change documents
that no longer resolve, for example after a rebase.
