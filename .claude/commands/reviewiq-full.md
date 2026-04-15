Full PR review — all files, post everything to PR: $ARGUMENTS

$ARGUMENTS: PR link (`https://github.com/owner/repo/pull/42`), PR number (`42`), or branch (`feature/xyz`).

## 1. Detect Input & Fetch

```bash
# PR link or number
gh pr view <N> --repo owner/repo --json files,title,author,baseRefName,headRefName,number,body,headRefOid
gh pr diff <N> --repo owner/repo

# Branch (no PR)
git diff main...<branch>
git diff --name-only main...<branch>
```

## 2. Create Review Folder

Create `.pr-review/pr-<N>/` (or `.pr-review/branch-<name>/`):
```
.pr-review/
  pr-42/
    state.json          # findings with status: pending/resolved
    round-1/
      report.md         # markdown report for this iteration
    history.md          # running log across rounds
```

If folder exists from previous review, increment round number.

## 3. Load Skills (token-efficient)

From `~/.reviewiq/skills/` or `.pr-review/skills/`:
- **Always** (6): commandments, security, scalability, stability, maintainability, performance
- **By extension**: only matching section from languages.md (e.g., Python section only)
- **By imports**: only matching section from frameworks.md (e.g., Django section only)
- **By domain**: fintech.md, fraud.md, etc. only if triggered

Log: "Skills loaded: X, Y, Z (~N words)"

## 4. Review All Files

Read full diff + all file contents. 4-stage review:
- Understand → Analyze (against skill checklists) → Assess → Report

For each finding output:
```
### Finding <N>: <title>
**Severity**: CRITICAL / IMPORTANT / NIT / QUESTION
**File**: `path:line`
**Status**: pending
**Problem**: ...
**Suggestion**: (exact replacement code for ```suggestion block)
**Resolution**: how to fix it
**Comment**: additional context
```

## 5. Post to PR

**Inline comments**: post each finding as inline comment with ```suggestion block on the exact line.

**PR comment with full iteration report**: post the markdown report as a PR comment. This becomes the persistent record of each review round.

```bash
gh pr comment <N> --body "$(cat round-N/report.md)"
```

The report format:
```markdown
# ReviewIQ Report — PR #42 Round 1
**Date**: 2026-04-15 | **Skills**: python, django (~4.2K words)

## Findings
### 1. [CRITICAL] Retry without backoff — `retry.py:42` — pending
**Suggestion**: `time.sleep(min(2 ** attempt * 0.5, 30))`
**Resolution**: Add exponential backoff with jitter
**Comment**: Thundering herd risk at 500 queued webhooks

## Summary
| Status | Count |
|--------|-------|
| Pending | 3 |
| Resolved | 0 |
Assessment: REQUEST CHANGES
```

On subsequent rounds (reviewiq-recheck), the new report is posted as a NEW comment — previous round reports stay visible in the PR history. Each iteration appends, never overwrites.

## 6. Save State & Report

`state.json`: all findings with `status: "pending"`.
`round-N/report.md`: full markdown report (same as posted to PR).
`history.md`: append this round's summary.
