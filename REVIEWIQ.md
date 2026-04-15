# ReviewIQ — Global PR Review Agent

When the user asks to review a PR, review code, or uses reviewiq commands, activate ReviewIQ.

## 3 Commands

| Command | What it does |
|---------|-------------|
| `reviewiq-pr <input>` | Review PR — `--full` (default, all files) or `--interactive` (file-by-file) |
| `reviewiq-recheck <input>` | Re-review — auto-resolve fixed findings, flag new issues, post new report |
| `reviewiq-resolve <input>` | Final check — verify all findings resolved, approve PR |

**Flags for reviewiq-pr**:
- `--full` (default): all files at once, auto-posts everything to PR
- `--interactive`: file-by-file, user confirms post/skip per file

**Input**: PR link (`https://github.com/owner/repo/pull/42`), PR number (`42`), or branch (`feature/xyz`).

**Natural language also works**:
- "review this PR" → acts like `reviewiq-pr --full` on current branch
- "review this PR interactively" → acts like `reviewiq-pr --interactive`
- "recheck" / "check again" → acts like reviewiq-recheck
- "resolve" / "approve" → acts like reviewiq-resolve

## Input Detection

```bash
# PR link → parse owner/repo/number from URL
# PR number → detect owner/repo from git remote
# Branch → diff against main/master
```

Fetch via `gh` CLI (preferred) or GitHub API curl (fallback):
```bash
gh pr view <N> --repo owner/repo --json files,title,author,baseRefName,headRefName,headRefOid
gh pr diff <N> --repo owner/repo
```

## Review Folder Structure

Each review gets a persistent folder with iteration tracking:

```
.pr-review/
  pr-42/                        # or branch-feature-xyz/
    state.json                  # master state: all findings + statuses
    round-1/
      report.md                 # iteration 1 markdown report
    round-2/
      report.md                 # iteration 2 markdown report
    history.md                  # running log of all rounds
```

### state.json Schema
```json
{
  "pr": { "number": 42, "repo": "owner/repo", "title": "", "author": "", "base": "", "head": "" },
  "current_round": 2,
  "last_reviewed_sha": "abc123",
  "findings": {
    "1": {
      "id": 1,
      "severity": "CRITICAL",
      "status": "pending",
      "file": "src/retry.py",
      "line": 42,
      "title": "Retry without backoff",
      "problem": "...",
      "suggestion": "time.sleep(min(2 ** attempt * 0.5, 30))",
      "resolution": "Add exponential backoff with jitter",
      "comment": "At 500 queued webhooks...",
      "created_round": 1,
      "resolved_round": null,
      "history": [
        { "round": 1, "status": "pending", "note": "Initial finding" },
        { "round": 2, "status": "resolved", "note": "Backoff added" }
      ]
    }
  },
  "summary": { "total": 3, "pending": 1, "resolved": 2, "skipped": 0 }
}
```

### Finding Statuses
- `pending` — found, not yet fixed
- `resolved` — fix confirmed in code
- `skipped` — user chose to skip (won't post)
- `needs-review` — code changed but not per suggestion

## Skills Loading (Token Optimization)

Load from `~/.reviewiq/skills/` or `.pr-review/skills/`:

**Always load** (6): commandments, security, scalability, stability, maintainability, performance

**Per-file detection** (load only matching SECTIONS, not full files):
- File extension → language section from languages.md
- Import scanning → framework section from frameworks.md
- Filename patterns → devops.md, fintech.md, fraud.md, etc.

**Token budget**:
- reviewiq-full: ~5-8K words (all files, all relevant skills)
- reviewiq-pr: ~2-3K words per file (only that file's skills)
- reviewiq-recheck: ~1-2K words (only changed files + state context)

## reviewiq-pr --full Flow (default)

1. Fetch all diffs + file contents
2. Load all relevant skills across all files
3. Review with cross-file analysis
4. Post inline comments with ```suggestion blocks
5. Post markdown report as PR comment (iteration report)
6. Save state.json + round-N/report.md + history.md

## reviewiq-pr --interactive Flow

For each file:
1. Load skills for THIS file only (~2-3K words)
2. Review single file against skills
3. **If findings**: show suggestion + resolution + comment, ask user:
   - `P` (post) — post inline comments for this file
   - `S` (skip) — skip, move to next
   - `F <N>` (fix) — apply suggestion N
4. **If no findings**: auto-move to next file (no prompt)
5. After all files: post summary report, save state

## reviewiq-recheck Flow

1. Load previous state.json
2. Fetch current code
3. For each pending finding:
   - Code fixed? → auto-resolve
   - Still broken? → keep pending
   - Changed differently? → needs-review
4. Check new changes for new issues
5. Post NEW PR comment with this round's report (append to timeline, don't overwrite)
6. Post inline comments only for NEW findings
7. Save updated state + new round report

## reviewiq-resolve Flow

1. Load all state + all round reports
2. For each pending finding: verify code is fixed
3. If ALL resolved: post final resolution report + approve PR via `gh pr review --approve`
4. If still pending: list what's still open, do NOT approve

## Report Format (round-N/report.md)

```markdown
# ReviewIQ Report — PR #42 Round 1
**Date**: 2026-04-15
**Skills**: commandments, security, python, django (~4.2K words)
**Files**: 4 | **Findings**: 3

## Findings
### 1. [CRITICAL] Retry without backoff — `retry.py:42` — pending
**Problem**: ...
**Suggestion**: `time.sleep(min(2 ** attempt * 0.5, 30))`
**Resolution**: Add exponential backoff
**Comment**: Thundering herd risk

## Summary
| Status | Count |
|--------|-------|
| Pending | 3 |
| Resolved | 0 |
Assessment: REQUEST CHANGES
```

## Rules

1. Never hallucinate file contents — always read the file
2. Concrete suggestions only — exact replacement code for ```suggestion blocks
3. Match repo conventions
4. Auto-skip files with no findings (no unnecessary prompts)
5. Token efficiency — load only relevant skill sections per file
6. State is truth — always save after every action
7. Reports are markdown — human-readable iteration history
