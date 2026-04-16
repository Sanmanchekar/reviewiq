Run tests for PR changes: $ARGUMENTS

$ARGUMENTS: PR link, PR number, branch, or empty (uses current branch).

## Steps
1. Detect repo: `REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)`
2. Identify changed files:
   - If PR number given: `gh pr diff <N> --repo $REPO --name-only`
   - If branch: `git diff --name-only main...HEAD`
   - If empty: use current branch vs main/master
3. Detect test framework from project:
   - Python: pytest, unittest (look for pytest.ini, setup.cfg, pyproject.toml, conftest.py)
   - JavaScript/TypeScript: jest, vitest, mocha (look for package.json scripts)
   - Go: `go test`
   - Java: maven/gradle test
   - Ruby: rspec, minitest
4. Run tests:
   a. First try targeted: only tests related to changed files
   b. If no targeted tests found, run full suite
   c. Also run linter/type-checker if available (flake8, mypy, eslint, tsc, etc.)
5. Report results:
   - List each test file run and pass/fail status
   - Show any failures with error output
   - If all pass: report green
   - If failures: show what failed and suggest fixes

## Test discovery patterns
- Python: `pytest <changed_dir>/` or `pytest tests/ -k <module_name>`
- JS/TS: `npx jest --findRelatedTests <changed_files>`
- Go: `go test ./path/to/changed/...`
- Look for test files matching: `test_*.py`, `*_test.go`, `*.test.ts`, `*.spec.ts`

## Key behavior
- Always run tests from the project root
- Respect existing test configuration (pytest.ini, jest.config, etc.)
- If no tests exist for changed code, suggest what tests should be written
- Report results clearly — don't just say "tests passed", show what ran
