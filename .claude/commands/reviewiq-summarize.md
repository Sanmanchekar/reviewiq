Generate a PR summary suitable for the merge commit.

## Steps

1. Load state from `.pr-review/reviews/`.
2. Read the full diff and changed files.
3. Generate a concise summary covering:
   - What changed and why
   - Key design decisions
   - Findings that were addressed and how they were resolved
   - Any remaining NITs or known trade-offs
   - Anything future readers should know
4. Format for use as a merge commit message or PR description.
