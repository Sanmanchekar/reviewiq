Re-review PR with history: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch.

## Steps
1. Detect repo: `gh repo view --json nameWithOwner -q .nameWithOwner`
2. Load state from GitHub PR hidden comment (`<!-- REVIEWIQ_STATE_COMMENT -->`)
3. Increment round number: new round = count of previous rounds + 1
4. Fetch current code, compare against last reviewed SHA in state
5. For each pending finding:
   - Code fixed? → auto-resolve, update status history
   - Still broken? → keep pending
   - Changed differently? → mark needs-review
6. Check new changes for NEW issues — assign next finding IDs
7. Post new round report as PR comment (append to timeline, never overwrite previous rounds)
8. Save updated state to GitHub PR hidden comment
