# Tasks

- [x] 1. Add a dispatch-time guard: a leading arg that isn't a recognized subcommand and doesn't start with `-` errors out (exit 2) instead of silently reaching `runSync`'s flag parsing
- [x] 2. Add a small `knownConfusions` table (currently just `push` → `sync`) so the error names the right command and explains why push isn't an alias, without making it one
- [x] 3. Unit tests: unknown subcommand errors; `push` specifically errors with a message suggesting `sync` and explaining the reconcile behavior
- [ ] 4. Update FusionHub docs referencing `specsync push` to use the real invocation (`specsync` / `specsync -change <name>`)
