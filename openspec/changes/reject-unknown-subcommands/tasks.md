# Tasks

- [ ] 1. Add `push` to the recognized subcommand dispatch in `main.go`, routed to `runSync`
- [ ] 2. Add a dispatch-time guard: a leading arg that isn't a recognized subcommand and doesn't start with `-` errors out (exit 2) instead of silently reaching `runSync`'s flag parsing
- [ ] 3. Unit tests: unknown subcommand errors; `push` behaves identically to bare/`sync` invocation, including flags after it
- [ ] 4. Update FusionHub docs referencing `specsync push` to also show `-slug` (still recommend scoping, even though `push` is now safe)
