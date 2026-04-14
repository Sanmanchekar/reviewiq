Incremental re-review of branch: $ARGUMENTS

The developer has pushed fixes. Follow the `check` command in `.pr-review/agent.md`.

## Steps

1. Load state from `.pr-review/reviews/` — you know exactly which findings are open and what SHA you last reviewed.
2. Get the incremental diff since `last_reviewed_sha` in state.
3. Read all currently changed files in full.
4. Load relevant skills from `.pr-review/skills/`.
5. For each finding in state:
   - **RESOLVED** → transition to `resolved` with note
   - **PARTIALLY FIXED** → transition to `partially_fixed` with note
   - **UNRESOLVED** → keep as `open`
6. Check for NEW issues introduced by the fixes — add as new findings.
7. Create a new review round in state, save to `.pr-review/reviews/pr-<N>.json`.
8. Output the status update table and updated summary.
