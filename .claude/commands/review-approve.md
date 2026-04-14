Final check — any blockers remaining?

## Steps

1. Load state from `.pr-review/reviews/`.
2. Check all findings:
   - List any CRITICAL or IMPORTANT findings still `open` or `partially_fixed` — these are blockers.
   - List any NITs still open — non-blocking.
3. Read all changed files one final time to catch anything new.
4. Output:

If blockers remain:
```
BLOCKED — the following findings must be addressed:
  [CRITICAL] Finding 1: <title>
  [IMPORTANT] Finding 2: <title>
```

If clear:
```
APPROVE — no remaining blockers. Safe to merge.
```

5. Update assessment in state to `APPROVE` if no blockers. Save state.
