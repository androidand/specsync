# Tasks

- [ ] 1.1 Resolver: branch → change for `feat/<issue>-<slug>`, `<prefix>/<slug>` and a bare `<slug>`; nothing for the default branch, detached HEAD, archived or unknown changes
- [ ] 1.2 `set-stage <stage>` with one argument uses the resolved change
- [ ] 1.3 `note <text>` with one argument uses the resolved change
- [ ] 1.4 `archive` without `-change` uses the resolved change
- [ ] 1.5 Print the picked change and branch on stderr; keep explicit arguments authoritative
- [ ] 1.6 Tests, each watched failing with the resolver disabled
- [ ] 1.7 `agent-help`, README and `site/features.json`
