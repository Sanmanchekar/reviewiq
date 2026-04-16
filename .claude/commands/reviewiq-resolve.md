Resolve all findings and approve PR: $ARGUMENTS

$ARGUMENTS: PR link, PR number, or branch.

## IMPORTANT: This command APPLIES fixes, it does NOT just verify them.

You MUST edit the actual source files to fix each finding. Do NOT just check if fixes exist.

## Steps
1. Detect repo: `REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)`
2. Get PR info: `gh pr view <N> --repo $REPO --json headRefOid,headRefName,baseRefName`
3. **Checkout the PR branch**: `gh pr checkout <N>` — you must be on the PR branch to push fixes
4. Load state from GitHub PR hidden comment (`<!-- REVIEWIQ_STATE_COMMENT -->`)
   - If no state comment exists, check conversation history for findings from prior review
5. For EACH pending/open finding:
   a. Read the target file using the Read tool
   b. Locate the problematic code at the finding's line number
   c. **EDIT the file** — use the Edit tool to apply the `suggested_fix` as the replacement code
   d. After editing, verify the fix is syntactically correct
6. **RUN TESTS** after all fixes are applied:
   a. Detect the project's test framework (pytest, go test, npm test, etc.)
   b. Run the test suite: focus on files that were modified
   c. If tests fail: diagnose, fix the issue, re-run until green
   d. If no test framework exists: run linter/type-checker (flake8, mypy, eslint, tsc, etc.)
   e. If no linter either: at minimum run syntax checks (py_compile, node --check, etc.)
7. **COMMIT AND PUSH the fixes**:
   a. `git add` all modified files
   b. `git commit -m "fix: resolve ReviewIQ findings for PR #<N>"`
   c. `git push` to the PR branch
   d. This is MANDATORY — without push, the PR branch still has the old broken code
8. After push:
   a. Save updated state to GitHub PR hidden comment (mark all as resolved)
   b. Post resolution report as PR comment listing every fix applied + test results
   c. Approve PR: `gh pr review <N> --repo $REPO --approve --body "All findings resolved and tests passing — ReviewIQ"`

## Key behavior
- This command WRITES CODE. It edits files directly.
- Every finding with a `suggested_fix` gets applied via the Edit tool.
- If a suggested_fix is unclear, use your judgment to write the correct fix.
- Tests MUST pass before committing. If tests fail, fix until green.
- Fixes MUST be committed and pushed. Approval without push is meaningless.
- After all edits, show `git diff --stat` so the developer can review before push.
