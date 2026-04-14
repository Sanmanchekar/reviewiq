Review the PR on branch: $ARGUMENTS

Follow the protocol in `.pr-review/agent.md`. Load relevant skills from `.pr-review/skills/` based on the changed files.

## Steps

1. Check for existing state: `ls .pr-review/reviews/` — if found, read the state file for prior findings.
2. Detect base branch: `git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@'` (fallback: `main`)
3. Get diff: `git diff <base>...$ARGUMENTS`
4. Read ALL changed files in full
5. Load skill files from `.pr-review/skills/` — always load `commandments.md`, `security.md`, `scalability.md`, `stability.md`, `maintainability.md`, `performance.md`. Then load language/framework/domain skills matching the changed files.
6. Run the 4-stage review: Understand → Analyze (against skill checklists) → Assess (CRITICAL/IMPORTANT/NIT/QUESTION) → Report
7. Save findings to `.pr-review/reviews/pr-<N>.json` per the state schema in `agent.md`

After review, remind the developer of available commands:
`/review-check`, `/review-explain`, `/review-fix`, `/review-status`, `/review-ask`, `/review-retract`, `/review-wontfix`, `/review-approve`, `/review-summarize`
