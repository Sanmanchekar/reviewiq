Apply the suggested fix for finding: $ARGUMENTS

Follow the `fix` command in `.pr-review/agent.md`.

## Steps

1. Load state from `.pr-review/reviews/` — find the finding by ID.
2. Read the current file state (don't assume it hasn't changed since the review).
3. Apply the suggested fix from the finding (or the refined version from discussion).
4. Self-check — re-read the file after editing and verify:
   - Syntax is valid
   - Logic is correct
   - Imports are present
   - No side effects on surrounding code
5. If the fix touches shared code, check callers with `git grep`.
6. Transition the finding to `resolved` with a note describing what was fixed.
7. Save state.
