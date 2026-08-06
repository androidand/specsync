# Tasks

- [ ] `specsync idea` subcommand: text arg or stdin, mechanical title derivation, verbatim body + timestamp, `stage:intake` label, prints issue URL
- [ ] `-repo` flag + `default_ideas_repo` config key (config file, env override) for cwd-independent capture
- [ ] Ensure `stage:intake` label is created on first use (same pattern as existing stage labels)
- [ ] `specsync ideas` list subcommand: open intake issues for repo/default-repo, oldest first
- [ ] `specsync pull -issue <n>` on an intake issue transitions the label intake → active on next sync (verify, adjust if needed)
- [ ] Docs: README section + skill file update (capture → triage → pull lifecycle)
- [ ] Tests: title derivation edge cases (one word, multiline, unicode), label creation idempotence, default-repo resolution order
