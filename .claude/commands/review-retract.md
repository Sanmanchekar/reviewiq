Retract finding (agent was wrong): $ARGUMENTS

Format: `<finding-id> [reason]`
Example: `/review-retract 3 ORM handles parameterization`

## Steps

1. Load state from `.pr-review/reviews/`.
2. Parse finding ID and optional reason from `$ARGUMENTS`.
3. Verify the finding exists and is currently open.
4. Transition the finding to `retracted` with the reason as the note.
5. Recompute the summary (total open, assessment).
6. Save state.
7. Output: `Finding <N>: open → retracted — <reason>`
