Review PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch + optional flag (--full or --interactive).

/reviewiq-pr 42                     # --full (default): all files, auto-post
/reviewiq-pr 42 --full              # same, explicit
/reviewiq-pr 42 --interactive       # file-by-file, post/skip per file

## Steps
1. Detect repo: `gh repo view --json nameWithOwner -q .nameWithOwner` — use this EXACT string everywhere
2. Fetch PR data + head SHA via gh CLI
3. Load existing state from GitHub PR hidden comment (look for `<!-- REVIEWIQ_STATE_COMMENT -->`) or start fresh as round 1
4. Load relevant skills from ~/.reviewiq/skills/ or .pr-review/skills/
5. --full (default): review all files, cross-file analysis, auto-post inline comments + report
6. --interactive: review per file, show findings, ask P(post)/S(skip)/F(fix), auto-skip if no findings
7. Save state to GitHub PR hidden comment (create or update the `<!-- REVIEWIQ_STATE_COMMENT -->` comment)
