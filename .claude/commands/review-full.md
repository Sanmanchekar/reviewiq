Full PR review in one shot — review all files, post comments, suggestions, and resolutions: $ARGUMENTS

$ARGUMENTS: GitHub PR link (e.g., `https://github.com/owner/repo/pull/42`) or PR number.

Unlike `/review-pr` (file-by-file interactive), this reviews the entire diff at once and posts everything to the PR automatically.

## Step 1: Fetch PR

```bash
# Get PR metadata and diff
gh pr view <number> --json title,author,baseRefName,headRefName,files
gh pr diff <number>
```

## Step 2: Load ALL Relevant Skills

Detect skills across ALL changed files at once:
- Always load: `commandments.md`, `security.md`, `scalability.md`, `stability.md`, `maintainability.md`, `performance.md`
- By file types: load matching sections from `languages.md`, `frameworks.md`, `devops.md`
- By domain: load matching domain skills (`fintech.md`, `fraud.md`, etc.)

Skills from: `.pr-review/skills/` (repo) → `~/.reviewiq/skills/` (global fallback)

## Step 3: Full Review

Read ALL changed files in full. Review the entire diff against loaded skills:

1. **Per-file analysis**: correctness, edge cases, security, performance per skill checklists
2. **Cross-file analysis**: do changes in file A break assumptions in file B? Missing migrations? Inconsistent error handling?
3. **Test coverage**: are all changed paths covered by tests in the PR?

For each finding:
- Exact file path and line number
- Severity: CRITICAL / IMPORTANT / NIT / QUESTION
- Concrete fix as exact replacement code (for GitHub ```suggestion blocks)
- Resolution recommendation

## Step 4: Post to PR

Post everything to the PR using `gh` CLI:

**Summary comment** with finding table:
```bash
gh pr comment <number> --body "## ReviewIQ Full Review
| # | Severity | File | Finding |
|---|----------|------|---------|
| 1 | CRITICAL | retry.py:42 | Retry without backoff |
..."
```

**Inline comments** with suggestion blocks on each finding:
```bash
gh api repos/{owner}/{repo}/pulls/{number}/comments \
  -f body="**[CRITICAL] Retry without backoff**
...
\`\`\`suggestion
time.sleep(min(2 ** attempt * 0.5, 30) + random.uniform(0, 1))
\`\`\`" \
  -f path="src/webhooks/retry.py" -F line=42 -f side="RIGHT" \
  -f commit_id="$(gh pr view <number> --json headRefOid -q .headRefOid)"
```

## Step 5: Save State

Save to `.pr-review/reviews/pr-<N>.json`.

After this, the user can:
- `explain finding N` — deep dive
- `check review` — re-review after fixes
- `approve` — final check
