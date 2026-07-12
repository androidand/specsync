# Keep skill artifacts synchronized

The completion feature updated `cmd/specsync/SKILL.md` but not the canonical
`skills/specsync/SKILL.md`. The npm release workflow packages the canonical
copy, so the next release would omit `-close-completed` and `stage:complete`
from the installed agent skill.

Make `skills/specsync/SKILL.md` the single authored source, regenerate every
derived copy, and add an automated drift check to CI or tests.

## Non-goals

- Rewriting the skill beyond correcting current CLI and lifecycle behavior.
- Changing skill installation locations.
