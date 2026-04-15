File-by-file PR review with user confirmation: $ARGUMENTS

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

Same structure as reviewiq-full:
```
.pr-review/
  pr-42/
    state.json
    round-1/
      report.md
    history.md
```

## 3. Show Overview

```
PR #42: "Add payment retry logic" by @developer
Branch: feature/payment-retry → main
Files (4):
  1. src/webhooks/retry.py        (+45, -12)
  2. src/webhooks/handler.py      (+8, -3)
  3. src/config/settings.py       (+2, -0)
  4. tests/test_webhooks.py       (+30, -0)

Reviewing file 1/4...
```

## 4. For Each File

### 4a. Load skills for THIS file only (token-efficient)
- `.py` → Python section from languages.md + check imports for framework
- Only the matching sections, not full skill files
- Log: "Skills: python, django (~2K words)"

### 4b. Review single file
Read only this file's diff + full content. Check against loaded skills.

### 4c. Show findings (if any)

If findings exist for this file, show each with:
```
### Finding 1: Retry without backoff
**Severity**: CRITICAL
**File**: `src/webhooks/retry.py:42`

**Suggestion**:
  time.sleep(min(2 ** attempt * 0.5, 30) + random.uniform(0, 1))

**Resolution**: Add exponential backoff with jitter

**Comment**: At 500 queued webhooks during recovery, immediate retry = thundering herd
```

Then ask:
```
File 1/4: src/webhooks/retry.py — 2 findings (1 CRITICAL, 1 NIT)

  [P] Post comments to PR    — post all findings for this file
  [S] Skip                    — don't post, move to next file
  [F] Fix <N>                 — apply suggestion for finding N
```

Wait for user input: `P` / `S` / `F <N>`

### 4d. If NO findings for this file
```
File 3/4: src/config/settings.py — No findings ✓
Moving to next file...
```
Auto-move to next file. No prompt needed.

### 4e. Post (if user chose P)
Post inline comments with ```suggestion blocks for this file only.

### 4f. Move to next file
Repeat from 4a.

## 5. Summary

After all files:
```
## Review Complete
| File | Findings | Posted | Skipped |
|------|----------|--------|---------|
| retry.py | 2 | 2 | 0 |
| handler.py | 1 | 0 | 1 |
| settings.py | 0 | — | — |
| test_webhooks.py | 1 | 1 | 0 |

Total: 4 findings | Posted: 3 | Skipped: 1
Assessment: REQUEST CHANGES
```

Post summary comment to PR.

## 6. Save State & Report

`state.json`: all findings with status (`pending` if posted, `skipped` if skipped).
`round-N/report.md`: full markdown report.
`history.md`: append round summary.

## Token Optimization

- Load skills PER FILE, not all at once
- Only the matching section (Python section, not full languages.md)
- Review one file at a time — previous file's context is NOT carried
- Auto-skip files with no findings (no LLM call needed for "no issues")
- Skill detection is local (file extension + import grep) — no LLM call
