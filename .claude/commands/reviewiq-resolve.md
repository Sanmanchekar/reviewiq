Resolve all findings and approve PR: $ARGUMENTS

$ARGUMENTS: PR link (`https://github.com/owner/repo/pull/42`), PR number (`42`), or branch (`feature/xyz`).

Goes through ALL review rounds, verifies every finding is resolved, and approves the PR.

## 1. Load All State

Read `.pr-review/pr-<N>/state.json` and all `round-*/report.md` files.

If no state exists:
  "No review found. Run /reviewiq-full or /reviewiq-pr first."

Show current state:
```
PR #42: 3 rounds of review
  Round 1: 3 findings (1C, 1I, 1N)
  Round 2: 2 resolved, 1 still pending, 1 new
  Current: 2 pending, 2 resolved
```

## 2. Verify Each Finding

For each finding with status `pending` or `needs-review`:

1. Read the current file at the finding's line
2. Check if the problematic code is gone
3. Check if the suggestion was applied (or equivalent fix)
4. Check PR comments — was the GitHub suggestion applied?

```bash
# Check if suggestion was applied via GitHub
gh api repos/{owner}/{repo}/pulls/{number}/comments | grep -A5 "suggestion"
```

Decision:
- Code fixed → **resolve** with note
- Still broken → **keep pending** — cannot approve

## 3. Generate Final Report

```markdown
# ReviewIQ Final Resolution — PR #42

## All Findings
| # | Severity | Status | Title | Resolution |
|---|----------|--------|-------|------------|
| 1 | CRITICAL | ✅ resolved | Retry without backoff | Backoff with jitter added (Round 2) |
| 2 | IMPORTANT | ✅ resolved | Missing idempotency | Idempotency key added (Round 3) |
| 3 | NIT | ✅ resolved | Error message format | Fixed (Round 2) |
| 4 | IMPORTANT | ✅ resolved | Null check missing | Guard clause added (Round 3) |

## Review Timeline
- **Round 1** (2026-04-15): Initial review — 3 findings
- **Round 2** (2026-04-15): Re-review — 2 resolved, 1 new
- **Round 3** (2026-04-16): Re-review — all resolved

## Verdict
**All findings resolved. APPROVED.**
```

## 4. Approve PR

If ALL findings are resolved:

```bash
# Post final resolution report
gh pr comment <N> --body "$(cat resolution-report.md)"

# Approve the PR
gh pr review <N> --approve --body "ReviewIQ: All findings resolved across 3 rounds of review. LGTM."
```

If findings are still pending:

```
Cannot approve — 2 findings still pending:
  Finding 2: [IMPORTANT] Missing idempotency key — handler.py:18
  Finding 4: [IMPORTANT] Null check missing — handler.py:35

Run /reviewiq-recheck after fixes are pushed.
```

## 5. Save Final State

Update `state.json`: all statuses finalized.
Save `resolution-report.md` in the review folder.
Append to `history.md`:
```
### Resolution — 2026-04-16
All 4 findings resolved. PR approved.
```
