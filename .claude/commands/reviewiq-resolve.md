Resolve all findings and approve PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch.

## IMPORTANT: This command APPLIES fixes, it does NOT just verify them.

You MUST edit the actual source files to fix each finding. Do NOT just check if fixes exist.

## Steps
1. Detect repo: `REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)`
2. Get PR info: `gh pr view <N> --repo $REPO --json headRefOid,headRefName,baseRefName`
3. Load state from GitHub PR hidden comment (`<!-- REVIEWIQ_STATE_COMMENT -->`)
   - If no state comment exists, check conversation history for findings from prior review
4. For EACH pending/open finding:
   a. Read the target file using the Read tool
   b. Locate the problematic code at the finding's line number
   c. **EDIT the file** — use the Edit tool to apply the `suggested_fix` as the replacement code
   d. After editing, verify the fix is syntactically correct
5. **RUN TESTS** after all fixes are applied:
   a. Detect the project's test framework (pytest, go test, npm test, etc.)
   b. Run the test suite: focus on files that were modified
   c. If tests fail: diagnose, fix the issue, re-run until green
   d. If no test framework exists: run linter/type-checker if available
6. After all fixes pass tests:
   a. Save updated state to GitHub PR hidden comment (mark all as resolved)
   b. Post resolution report as PR comment listing every fix applied + test results
   c. Approve PR: `gh pr review <N> --repo $REPO --approve --body "All findings resolved and tests passing — ReviewIQ"`

## Key behavior
- This command WRITES CODE. It edits files directly.
- Every finding with a `suggested_fix` gets applied via the Edit tool.
- If a suggested_fix is unclear, use your judgment to write the correct fix.
- Tests MUST pass before approving. If tests fail, fix until green.
- After all edits, the developer can review the changes via git diff.
