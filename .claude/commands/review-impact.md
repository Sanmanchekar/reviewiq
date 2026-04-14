Blast radius analysis for the current PR changes.

## Steps

1. Load state from `.pr-review/reviews/` to understand known findings.
2. Get the full diff and changed files.
3. For each changed function/class/module, trace ALL callers:
   ```
   git grep -n <symbol>
   ```
4. Map:
   - Direct callers — code that directly calls the changed code
   - Transitive dependencies — code that calls the callers
   - Shared state — config, DB schema, env vars, caches affected
   - External consumers — APIs, webhooks, events other services depend on
5. Flag anything that could break in production but would pass tests (missing integration coverage).
6. Output a blast radius table:

```
## Blast Radius
| Changed | Directly Affected | Transitively Affected | Risk |
|---------|-------------------|----------------------|------|
| retry.py:send() | handler.py, worker.py | api.py (via handler) | HIGH |
```
