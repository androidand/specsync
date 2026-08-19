# Design: link existing issues by reference

Extends `cross-repo-linked-issues` (which shipped `specsync link <slug> <slug>`
and `-repo`). Covers only what changes when a `link` argument is an issue
reference rather than a slug.

## Argument classification

`link` already takes a list of arguments; each is now classified with the same
rules `resolveEntry` (change.go) uses for `links.md` entries, plus a bare-number
form:

| Argument | Meaning |
|---|---|
| `owner/repo#N` | issue N in an explicit repo |
| `https://github.com/owner/repo/issues/N` | same, by URL |
| `#N` or `N` | issue N in the `-repo` repo, else the auto-detected one |
| anything else | a local change slug (must exist under `changes/`) |

Slugs and references may be mixed. Reference forms are deliberately the *same*
shorthands `links.md` already accepts, so there is one syntax to learn, not two.

## Two paths, one `## Related` renderer

The slug path is unchanged: write `links.md`, sync, and `WorkItemFor` re-renders
the body (including `## Related`). The reference path never touches disk:

1. resolve the issue via the existing `IssueReader.Get` (fetches the current body);
2. upsert a `## Related` section listing the *other* arguments' issue URLs;
3. push the edited body via the provider's issue-edit path.

Both paths must produce a byte-identical `## Related` block, so the rendering in
`WorkItemFor` (`sync.go`, the `body + "\n\n## Related\n\n" + …` append using
`refLabel`) is factored into a shared helper:

```
upsertRelatedSection(body string, links []Ref) string
```

It finds an existing `## Related` heading and replaces the block up to the next
`##` heading (or end of body); if none exists it appends one. This is what makes
re-running `link` idempotent on a hand-authored issue body — the section is
replaced, never duplicated — and guarantees the spec-rendered and
reference-edited forms can't drift. `WorkItemFor` calls the same helper instead of
its inline append.

## Cross-repo targeting

Each reference carries its repo: `owner/repo#N` and URLs name it explicitly; a
bare `#N` uses `-repo` if given, else the auto-detected remote. specsync builds
one provider per distinct repo (`NewGitHubProviderWithRepo`, the existing
`github:owner/repo` keying) and edits each issue through the provider bound to its
repo. One `link` call therefore spans repos, which is the case that motivated this.

## What it deliberately does not do

- **No issue creation.** A reference must resolve to an existing issue; an
  unresolved reference is an error, not a create. `link` cross-references; it does
  not author work.
- **No local artifact for reference links.** Pure-reference links write no change
  directory and no `links.md` — the cross-reference exists only in the GitHub
  issue bodies. (A slug argument still writes its `links.md` exactly as today.)
- **No unlinking.** This change only adds or refreshes `## Related`. Removing a
  relationship is a separate, later addition once a real need appears.

## The one mutation, and dry-run

Editing an issue body is a write — but `link` already writes issue bodies for the
slug path, so this adds no new *kind* of side effect, only a new way to name the
target. `link -dry-run` extends to references: it prints, per issue, the `gh issue
edit` it would run and the `## Related` block it would write, and makes no GitHub
calls — the same dry-run discipline the rest of the tool follows.

## Boundaries

- Stdlib-only; shells out to `gh` through the existing provider. No new dependency.
- GitHub-shaped reference forms; other providers gain an equivalent when they
  exist (`pluggable-providers`). The classification and the `Ref`-based renderer
  stay provider-agnostic in the core.
