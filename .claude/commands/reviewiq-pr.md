Review PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch + optional flag.

```
/reviewiq-pr 42                          # default: --full
/reviewiq-pr 42 --full                   # all files at once, auto-post
/reviewiq-pr 42 --interactive            # file-by-file, post/skip per file
/reviewiq-pr https://github.com/owner/repo/pull/42
/reviewiq-pr feature/payment-retry --interactive
```

Parse $ARGUMENTS: extract the PR identifier and check for `--full` or `--interactive` flag. Default is `--full`.

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

```
.pr-review/
  pr-42/
    state.json          # findings with status: pending/resolved
    round-1/
      report.md         # markdown report for this iteration
    history.md          # running log across rounds
```

If folder exists from previous review, increment round number.

## 3. Load Skills

From `~/.reviewiq/skills/` or `.pr-review/skills/`:
- **Always** (6): commandments, security, scalability, stability, maintainability, performance
- **By extension**: only matching section from languages.md
- **By imports**: only matching section from frameworks.md
- **By domain**: fintech.md, fraud.md, etc. only if triggered

Log: "Skills loaded: X, Y, Z (~N words)"

---

## --full Mode (default)

Reviews all files at once with cross-file analysis, auto-posts everything.

### 4a. Review All Files

Read full diff + all file contents. 4-stage review:
- Understand → Analyze (against skill checklists) → Assess → Report

For each finding:
```
### Finding <N>: <title>
**Severity**: CRITICAL / IMPORTANT / NIT / QUESTION
**File**: `path:line` | **Status**: pending
**Suggestion**: exact replacement code
**Resolution**: how to fix it
**Comment**: additional context
```

### 5a. Post to PR

- Inline comments with ```suggestion blocks on each finding line
- PR comment with full markdown report (iteration report)

```bash
gh pr comment <N> --body "$(cat round-N/report.md)"
```

### 6a. Save

`state.json` + `round-N/report.md` + append to `history.md`

---

## --interactive Mode

Reviews one file at a time. Shows findings, waits for user confirmation.

### 4b. For Each File

**Load skills for THIS file only** (token-efficient: ~2-3K words vs ~8K)

**If findings exist**, show each with suggestion + resolution + comment, then ask:
```
File 1/4: src/webhooks/retry.py — 2 findings (1 CRITICAL, 1 NIT)

  Finding 1: [CRITICAL] Retry without backoff — line 42
  Suggestion: time.sleep(min(2 ** attempt * 0.5, 30))
  Resolution: Add exponential backoff with jitter

  [P] Post comments    [S] Skip    [F 1] Fix finding 1
```

Wait for: `P` (post) / `S` (skip) / `F <N>` (apply fix)

**If NO findings**: auto-move to next file (no prompt)

### 5b. After All Files

Post summary as PR comment. Save state + report.

---

## Report Format (posted as PR comment)

```markdown
# ReviewIQ Report — PR #42 Round 1
**Date**: 2026-04-15 | **Mode**: full | **Skills**: python, django (~4.2K words)

## Findings
### 1. [CRITICAL] Retry without backoff — `retry.py:42` — pending
**Suggestion**: `time.sleep(min(2 ** attempt * 0.5, 30))`
**Resolution**: Add exponential backoff with jitter
**Comment**: Thundering herd risk

## Summary
| Status | Count |
|--------|-------|
| Pending | 3 |
| Resolved | 0 |
Assessment: REQUEST CHANGES
```

Each round is a NEW PR comment — previous rounds stay visible.

## Token Budget

| Mode | What's loaded | Cost |
|------|-------------|------|
| `--full` | All files + all relevant skills | ~5-8K words |
| `--interactive` | One file + that file's skills | ~2-3K words/file |
