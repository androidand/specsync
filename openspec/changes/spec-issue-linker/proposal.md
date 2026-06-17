# Spec ↔ issue linker

## Why

A spec and its issue can originate from either side: an issue created first (by a
human or an orchestrator) that later gains a spec, or a spec authored first that
needs an issue. specsync must resolve "which issue is this change" robustly
across both, without inventing a parallel identity scheme.

## What

Add a `Linker` abstraction that resolves a change to its issue ref by consulting,
in priority order:

1. the current git branch name (issue-linked branches encode the issue number),
2. the `<!-- specsync:change=<slug> -->` marker in issue bodies,
3. the local `.specsync/` ref cache,
4. an external relation source (e.g. an MCP that knows issue↔branch↔repo links).

The first hit wins; the result is cached. This makes issue-first and spec-first
flows converge on the same link.

## Scope

- `Linker` interface + chained resolver.
- Branch-name resolver (configurable pattern).
- Wire the resolver into the sync engine ahead of provider `Find`.
