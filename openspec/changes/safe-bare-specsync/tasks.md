# Tasks

- [x] 1.1 No arguments and `-h`/`-help`/`--help`/`help` print grouped commands from the registry and exit 0
- [x] 1.2 Refuse a writing sync with neither `-change` nor `-all`; allow `-dry-run`
- [x] 1.3 Reword the unknown-subcommand error
- [x] 1.4 Tests: bare and help print usage and never call `gh`; an unscoped sync is refused without calling `gh`; `-all` and `-change` still sync; `-dry-run` still previews
- [x] 1.5 README, skill and agent-help no longer describe bare `specsync` as a sync
