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

Post each finding as inline comment with ```suggestion block.
Post summary comment with finding table + assessment.

## 6. Save State & Report

`state.json`: all findings with `status: "pending"`.
`round-N/report.md`: full markdown report with date, skills used, all findings.
`history.md`: append this round's summary.
