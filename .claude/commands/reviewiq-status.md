Show current finding statuses.

## Steps

1. Find the most recent state file in `.pr-review/reviews/`.
2. Read it and output a status table:

```
## Review Status (Round N)
| # | Severity | Status | Title | File |
|---|----------|--------|-------|------|
| 1 | CRITICAL | resolved | Retry without backoff | webhook.ts:42 |
| 2 | IMPORTANT | open | Missing input validation | api.ts:15 |

Open: X | Resolved: Y | Won't fix: Z | Retracted: W
Assessment: REQUEST CHANGES
```

Do NOT re-review or call any APIs. Just read and display the state file.
