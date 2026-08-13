# Idea intake command

## Why

Ideas arrive faster than they can be planned, and any idea that isn't
captured somewhere durable is lost. The predecessor (skein's `idea`
command) had the right instinct — drop any thought, vague or elaborate,
into a funnel — but coupled capture to autonomous planning and
implementation. When the autonomous stage failed, the idea itself was lost
in the noise, which destroyed trust in the funnel entirely.

The lesson: **capture must be guaranteed and dumb; triage is a separate,
later, visible step.** The durable inbox already exists — GitHub Issues,
the same tracker specsync projects specs onto. An idea is just an issue at
the earliest lifecycle stage. That also makes the funnel org-friendly:
teammates and managers see ideas as ordinary issues, and no one is forced
into the full spec workflow to use it.

## What

### 1. `specsync idea`

`specsync idea "<text>"` (or stdin for long-form) creates a GitHub issue
in the current repo — or `-repo owner/name`, with a configurable default
repo so personal ideas can flow to a dedicated ideas repo regardless of
cwd. Behavior:

- Title derived mechanically (first line / first sentence, truncated);
  body is the idea text verbatim plus a capture timestamp. No AI in the
  capture path — capture never waits, never fails on model errors.
- Labeled `stage:intake`, joining the existing `stage:*` lifecycle
  (intake → active → complete → archived).
- Prints the issue URL and nothing else. Sub-second, single command.

### 2. `specsync ideas`

Lists open `stage:intake` issues for the repo (or `-repo`/default), so
"what have I captured that nobody has triaged?" is one command. This is
the rest-assured view: nothing lost, everything visible.

### 4. Site promotion

The idea command is a new user-facing capability, so this change also
updates `site/features.json` (a "Capture ideas" card under the plan group)
and regenerates `site/index.html` — per the repo rule that site updates
ship with the capability change, not as a follow-up.

### 3. Triage path (existing machinery)

An intake issue graduates by being pulled into a change
(`specsync pull -issue <n>`), which flips it into the normal spec
lifecycle. Closing with a reason is a legitimate disposition — a written
"why not" closes the loop on an idea rather than losing it.

## Non-goals

- No automated triage, classification, or scheduled steward runs — that is
  a follow-up change once capture has earned trust.
- No cross-repo/global idea search or context inference — the fuzzy
  "funnel for ALL thoughts regardless of context" problem is explicitly
  deferred; the default-repo config is the pragmatic bridge.
- No local idea storage — the tracker is the single source of truth.
