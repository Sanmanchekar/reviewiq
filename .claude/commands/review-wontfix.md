Mark finding as won't fix: $ARGUMENTS

Format: `<finding-id> [reason]`
Example: `/review-wontfix 2 acceptable risk at our scale`

## Steps

1. Load state from `.pr-review/reviews/`.
2. Parse finding ID and reason from `$ARGUMENTS`.
3. Read the finding — consider whether the developer's reasoning is sound.
4. If sound, transition to `wontfix` with the reason. If not, explain why and keep as `open`.
5. Record the exchange in the finding's discussion thread.
6. Recompute summary and save state.
