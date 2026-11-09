# Smooth first use of specsync in a repo that has no openspec/

## Context

Observed while linking a bug fix and a cleanup to GitHub issues in a repo that
had never used OpenSpec (a product repo, `acme/app`). Using specsync did not come
naturally: each step below cost a detour. Facts only; commands as run with
specsync from an npm global install on 2026-10-27.

## Observed

1. **No `openspec/` folder: `pull` fails and does not say what to do.**
   `specsync pull -issue 12` printed
   `no openspec/ directory at <path> (pass -openspec, -store, or declare store: in openspec/config.yaml)`.
   It does not create the folder and does not name the command that does
   (`openspec init --tools none`). The user expected specsync to create it.
   After `openspec init --tools none .` the same pull worked.
2. **A freshly pulled change fails `validate`.**
   `pull` wrote `proposal.md` and `specsync.yml` but no `tasks.md`.
   `specsync validate` then reported `tasks.md: missing (change folder requires tasks.md)`.
   The wrapper `wt start` printed the same as a failure right after its own pull.
3. **`validate` cannot be scoped to one change.** It has `-json`, `-openspec`
   and `-store` only. In another repo it reported an unrelated change
   (`v2-button-parity/proposal.md: missing`) as a failure of the current
   work.
4. **A PR with no matching change gets no issue link, quietly.**
   `wt pr prep` printed `CHANGE none matched; PR will not carry a specsync reference`
   and carried on as ready. `pr-body` needs `-change`, so there was nothing
   to generate. The issue link had to be written by hand (`Closes owner/repo#N`).
5. **A gitignored `openspec/` is silent.** In that repo `.gitignore` lists
   `openspec/`. `pull` and `wt save` reported success, but the change can
   never reach the PR or teammates. Nothing said so.
6. **Discoverability.** `specsync help` answers `unknown subcommand "help"`
   (the real entry is `agent-help`). `specsync topology` in a repo with no
   store prints only `No changes resolved.` with no hint about stores.

## Proposed changes

- `pull` (and the no-arg sync) in a repo without `openspec/`: say exactly how
  to create it, or offer to (`-init`, with a prompt, never silently).
- `pull` also writes a starter `tasks.md`, so a new change validates.
- `validate -change <slug>` checks one change; the default stays repo-wide.
- When a PR has no change, `pr-body`/`wt pr prep` should say plainly that
  the issue link is missing and print the `Closes` line for the issue found
  from the branch name.
- Warn once when the `openspec/` directory is gitignored.
- `help` is an alias of `agent-help`; `topology` with no store explains what
  a store is and how to register one.

## Not in scope

- Creating issues from specs (the board and duplicate checks live in the
  backlog tool, not here).
