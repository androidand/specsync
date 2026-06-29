# Tasks: Multi-provider sync (fan-out)

- [ ] Make `-provider` repeatable (and config-resolvable); default to a single `github` when unset
- [ ] Change `Sync` to take a set of providers; loop projection + reconcile over each
- [ ] Reconcile inbound from every provider into `tasks.md` via the shared monotonic union (no provider↔provider path)
- [ ] Aggregate `Result` across providers (per-change, per-provider created/updated + reconcile flips)
- [ ] Per-provider dry-run output
- [ ] Failure isolation: one provider's error must not corrupt another's refs or abort the whole run silently
- [ ] Tests: a change fans out to two fake providers; inbound union from both; refs coexist in `refs.json`; one provider failing leaves the other's ref intact
- [ ] Docs: the fan-out model + the star-not-mesh / single-source-of-truth invariant
