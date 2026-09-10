While updating the specsync skill to document `adopt`, found the skill and
`doctor` were both pointing at a `--profile minimal|docs|full` flag on
`install-skill` that was never implemented — leftover from the unstarted
`add-skill-install-profiles` proposal (0/tasks checked). Any agent following
the skill's own "Install" section, or following `doctor`'s own token-size
recommendation, would hit `flag provided but not defined: -profile` and fail.

Fixed inline (small, same PR):
- `skills/specsync/SKILL.md` — dropped the fictional `--profile`
  examples/token-estimate table and the false claim that reference docs "may
  be installed alongside this skill" (they aren't; `docs/*.md` never ships
  past this repo — see below).
- `cmd/specsync/doctor.go` — the size-warning recommendation no longer
  suggests `--profile minimal`.
- `skills/specsync/docs/reference.md` / `docs/workflow.md` — documented the
  existing but previously-undocumented `scan -openspecs` (multi local
  `openspec/` dirs) and `scan -references` (OpenSpec's own
  `references:`/workset sibling-repo model) flags, and added an `adopt`
  workflow section.
- `site/features.json` — added an "Adopt an existing issue" card
  (`status: soon`, issue 160) and rebuilt `site/index.html`.

Also confirmed, while answering a related question this session: specsync has
no concept of a single external/shared spec repo that multiple code repos
point to instead of keeping their own local `openspec/`. What exists is (a)
pluggable spec *format* sources (OpenSpec vs. Beads, `pkg/spec.SpecSource`)
and (b) peer coordination between sibling repos that each still have their
own local `openspec/`, via `scan -openspecs`/`-references` and OpenSpec's own
`context`/`workset` commands. `declarative-repo-policies` (0% implemented)
is the only backlog proposal that touches multi-repo spec policy, and it's
about git-tracking policy for a repo's own specs, not remote sourcing.
