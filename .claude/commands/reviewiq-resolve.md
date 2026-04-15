Mark finding as resolved: $ARGUMENTS

Format: `<finding-id> [note]`
Example: `/reviewiq-resolve 1 backoff added with jitter`

## Steps

1. Load state from `.pr-review/reviews/`.
2. Parse finding ID and optional note from `$ARGUMENTS`.
3. Transition the finding to `resolved` with the note.
4. Recompute summary and save state.
5. Output: `Finding <N>: open → resolved — <note>`
6. If all CRITICAL/IMPORTANT findings are now resolved, note that the PR may be ready to merge.
