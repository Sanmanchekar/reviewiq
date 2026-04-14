Deep dive into finding: $ARGUMENTS

Follow the `explain` command in `.pr-review/agent.md`.

## Steps

1. Load state from `.pr-review/reviews/` — find the finding by ID.
2. Read the file referenced by the finding in full.
3. Trace execution: what calls this code? What does it call? What state does it touch? Use `git grep -n <symbol>`.
4. Show concrete scenarios where the issue manifests.
5. If the developer disagrees, engage with their reasoning — if they're right, transition the finding to `retracted`.
6. Add the exchange to the finding's `discussion` array in state.
7. Save state.
