Review this PR: $ARGUMENTS

$ARGUMENTS can be:
- A GitHub PR link: `https://github.com/owner/repo/pull/42`
- A branch name: `feature/my-branch`
- Empty (auto-detect current branch)

## Step 1: Detect Input

If `$ARGUMENTS` contains `github.com/*/pull/*`:
  → Fetch PR via `gh pr view <number> --json files,title,author,baseRefName,headRefName`
  → Get per-file diffs via `gh pr diff <number>`

If `$ARGUMENTS` is a branch name or empty:
  → Use current branch: `git rev-parse --abbrev-ref HEAD`
  → Detect base: `git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@'` (fallback: main)
  → Get changed files: `git diff --name-only <base>...<head>`

## Step 2: Show Overview

```
PR #42: "Add payment retry logic" by @developer
Branch: feature/payment-retry → main
Changed files (4):
  1. src/webhooks/retry.py        (+45, -12)
  2. src/webhooks/handler.py      (+8, -3)
  3. src/config/settings.py       (+2, -0)
  4. tests/test_webhooks.py       (+30, -0)
```

## Step 3: File-by-File Review

For EACH file, one at a time:

1. **Show the diff** for this file only
2. **Detect skills** for this file type (e.g., `.py` → load python section from `languages.md`, check imports for framework)
3. **Load only relevant skills** from `~/.reviewiq/skills/` or `.pr-review/skills/`
4. **Review this single file** against the loaded skill checklists
5. **Report findings** with severity, line number, and concrete fix
6. **Ask the user** what to do next:
   - `next` → move to next file
   - `implement N` / `fix N` → apply the fix
   - `explain N` → deep dive
   - `skip` → skip this file
   - `post` → post findings as inline PR comments (uses `gh pr comment`)

## Step 4: Post to PR (when user says "post")

Post findings as inline comments on the PR using `gh` CLI:

```bash
# Post inline comment with suggestion on specific file and line
gh api repos/{owner}/{repo}/pulls/{number}/comments \
  -f body="**[CRITICAL] Retry without backoff**

Webhook retries fire immediately. Thundering herd risk.

\`\`\`suggestion
time.sleep(min(2 ** attempt * 0.5, 30) + random.uniform(0, 1))
\`\`\`

_Why_: Exponential backoff with jitter and 30s cap." \
  -f commit_id="<sha>" \
  -f path="src/webhooks/retry.py" \
  -F line=42 \
  -f side="RIGHT"
```

The ```suggestion block lets the developer click "Apply suggestion" in GitHub UI.

## Step 5: Summary

After all files, post a summary comment:
```
## ReviewIQ Summary
| File | Findings | Critical | Important | Nit |
|------|----------|----------|-----------|-----|
| retry.py | 2 | 1 | 0 | 1 |
| handler.py | 1 | 0 | 1 | 0 |
Total: 3 findings | Assessment: REQUEST CHANGES
```

## Step 6: Save State

Save to `.pr-review/reviews/pr-<N>.json` after each file (not just at the end).

## Token Efficiency

- Load skills per-file, not all at once
- Review one file at a time — never load all files into context
- Only load the matching language/framework section, not the full skill file
