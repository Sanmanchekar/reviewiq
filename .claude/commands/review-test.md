Generate test cases for the reviewed changes: $ARGUMENTS

Optional argument: specific finding ID to generate tests for.

## Steps

1. Load state from `.pr-review/reviews/` to get open findings.
2. Find existing test files for the changed code:
   ```
   find . -name "*test*" -o -name "*spec*" | head -20
   ```
3. Read existing test files to learn conventions (framework, naming, file location).
4. Generate test cases covering:
   - **Happy path** of the new behavior
   - **Edge cases** each open finding identified
   - **Regression tests** for the original behavior (make sure the fix doesn't break what worked)
5. If `$ARGUMENTS` specifies a finding ID, focus tests on that finding's specific scenario.
6. Write tests matching the repo's existing conventions. Output the test code.
