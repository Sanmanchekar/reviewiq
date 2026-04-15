Re-review PR with history — check what's resolved, what's still open: $ARGUMENTS

$ARGUMENTS: PR link (`https://github.com/owner/repo/pull/42`), PR number (`42`), or branch (`feature/xyz`).

## 1. Load Previous State

Read `.pr-review/pr-<N>/state.json`. If no state exists:
  "No previous review found. Run /reviewiq-full or /reviewiq-pr first."

Show previous review summary:
```
Previous review: Round 1 (2026-04-15)
  3 findings: 1 CRITICAL (pending), 1 IMPORTANT (pending), 1 NIT (pending)
  Assessment: REQUEST CHANGES
```

## 2. Fetch Current State

```bash
gh pr diff <N> --repo owner/repo
gh pr view <N> --repo owner/repo --json files,headRefOid,comments
```

Also fetch PR comments to see if suggestions were applied:
```bash
gh api repos/{owner}/{repo}/pulls/{number}/comments
```

## 3. Check Each Finding Against Current Code

For each finding in state.json with status `pending`:

1. Read the current version of the file
2. Check if the problematic code still exists at the reported line
3. Check if the suggested fix was applied (exact match or equivalent)
4. Check PR comments — was the suggestion accepted/applied via GitHub UI?

Classify:
- **Auto-resolve**: code is fixed (suggestion applied or equivalent fix) → status: `resolved`
- **Still open**: problematic code still present → status: `pending` (keep open)
- **Modified**: code changed but not per suggestion → status: `needs-review` (re-analyze)

## 4. Check for New Issues

Diff current code against the last reviewed SHA (from state.json).
If there are NEW changes since last review:
- Review only the new/changed code
- Load skills only for the new files
- Add new findings (if any)

## 5. Report

```
## Re-review Report — Round 2

### Status Updates
| # | Severity | Previous | Current | Note |
|---|----------|----------|---------|------|
| 1 | CRITICAL | pending | resolved | Backoff with jitter added ✓ |
| 2 | IMPORTANT | pending | pending | Still missing idempotency key |
| 3 | NIT | pending | resolved | Error message fixed ✓ |
| 4 | IMPORTANT | — | NEW | Null check missing (new code) |

### Summary
- Previously: 3 findings (1C, 1I, 1N)
- Now: 2 open (1I, 1I-new), 2 resolved
- Assessment: REQUEST CHANGES → NEEDS DISCUSSION

### Still Open
Finding 2: Missing idempotency key — `handler.py:18`
Finding 4 (NEW): Null check missing — `handler.py:35`
```

## 6. Post Update to PR

**Post as NEW PR comment** — do NOT edit previous round comments. Each round is a separate comment so the full history is visible in the PR timeline.

```bash
gh pr comment <N> --body "$(cat round-N/report.md)"
```

For NEW findings only, post inline comments with ```suggestion blocks.
Do NOT re-post inline comments for existing pending findings.

## 7. Save State & Report

Update `state.json` with new statuses.
Create `round-N/report.md` (same content as posted to PR).
Append to `history.md`:
```
### Round 2 — 2026-04-15
Re-review: 2/3 resolved, 1 new finding
Assessment: NEEDS DISCUSSION
```

The PR now has a visible timeline:
```
Comment 1: ReviewIQ Report — Round 1 (3 findings, REQUEST CHANGES)
Comment 2: ReviewIQ Report — Round 2 (2 resolved, 1 new, NEEDS DISCUSSION)
Comment 3: ReviewIQ Report — Round 3 (all resolved, APPROVE)
```

## Token Optimization

- Only re-read files that had findings or new changes
- Don't re-review files with no findings and no changes
- Load skills only for files being re-analyzed
- Use previous finding context (from state.json) instead of re-deriving
