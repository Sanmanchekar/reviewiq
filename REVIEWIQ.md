# ReviewIQ — Global PR Review Agent

When the user asks to review a PR, review code, or uses reviewiq commands, activate ReviewIQ.

## 3 Commands

| Command | What it does |
|---------|-------------|
| `reviewiq-pr <input>` | Review PR — `--full` (default, all files) or `--interactive` (file-by-file) |
| `reviewiq-recheck <input>` | Re-review — auto-resolve fixed findings, flag new issues, post new report |
| `reviewiq-resolve <input>` | Fix all findings in code, run tests, mark resolved, auto-approve PR |
| `reviewiq-test <input>` | Run tests for PR changes — detect framework, run targeted tests, report results |

**Flags for reviewiq-pr**:
- `--full` (default): all files at once, auto-posts everything to PR
- `--interactive`: file-by-file, user confirms post/skip per file

**Input**: PR link (`https://github.com/owner/repo/pull/42`), PR number (`42`), or branch (`feature/xyz`).

**Natural language also works**:
- "review this PR" → acts like `reviewiq-pr --full` on current branch
- "review this PR interactively" → acts like `reviewiq-pr --interactive`
- "recheck" / "check again" → acts like reviewiq-recheck
- "resolve" / "approve" → acts like reviewiq-resolve
- "run tests" / "test this" → acts like reviewiq-test

## Input Detection

**CRITICAL**: NEVER guess the repo name. Always detect it first.

**Step 1 — Detect owner/repo** (run FIRST, before anything else):
```bash
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
```

**Step 2 — Get PR info + head SHA**:
```bash
gh pr view <N> --repo $REPO --json title,author,baseRefName,headRefName,headRefOid,url
gh pr diff <N> --repo $REPO
HEAD_SHA=$(gh pr view <N> --repo $REPO --json headRefOid -q .headRefOid)
```

## State Storage (GitHub-only)

State lives ONLY in a hidden GitHub PR comment — no local files. This makes state available across machines and CI.

The state comment has markers:
```
<!-- REVIEWIQ_STATE_COMMENT -->
<details><summary>ReviewIQ State (Round N) — X open, Y resolved</summary>
...summary table...
</details>
<!-- REVIEWIQ_STATE_START -->
<base64-encoded JSON state>
<!-- REVIEWIQ_STATE_END -->
```

### How to load state

**Endpoint matters**: state lives in *issue comments* (PR-level comments), NOT *review comments* (inline). They are different APIs:

| What | Endpoint | When |
|---|---|---|
| List all PR-level comments | `repos/{r}/issues/{N}/comments` | use this to find state comments |
| Fetch single PR-level comment by id | `repos/{r}/issues/comments/{id}` | use this to re-fetch state |
| Fetch single review comment (inline) | `repos/{r}/pulls/comments/{id}` | NOT for state — will 404 |

```bash
# 1. List ALL state comments (PR-level) — pick the one with the highest round
gh api repos/{owner}/{repo}/issues/{pr}/comments --paginate \
  -q '.[] | select(.body | contains("<!-- REVIEWIQ_STATE_COMMENT -->")) | {id: .id, user: .user.login, body: .body}'

# 2. Fetch a specific state comment by id (when re-loading mid-session)
gh api repos/{owner}/{repo}/issues/comments/{comment_id} -q '.body'

# 3. Decode the base64 payload between the START/END markers
```

**Round number rule**: `next_round = max(round markers across ALL existing state comments) + 1`. NEVER assume sequential — phantom rounds exist when CI auto-recheck and human recheck both run on the same PR. Tolerate gaps.

**Foreign state**: if the highest-round comment was authored by a user/bot that is NOT this session's actor (e.g., CI bot left a state comment we didn't write), treat its `resolved`/`wontfix` statuses as **untrusted**. For each such finding, re-verify against the current code before honoring the cached status. If the code still has the problem, flip the status back to `open` with a `status_history` entry noting the discrepancy.

If no state comment found, start fresh (round 1, empty findings).

### How to save state (MANDATORY after every command)

**Always create a NEW comment — never PATCH/overwrite previous rounds.** Each round's state is preserved for audit trail.

```bash
# Always POST new comment (never update old ones)
gh api repos/{owner}/{repo}/issues/{pr}/comments \
  -f body='<!-- REVIEWIQ_STATE_COMMENT -->
<!-- REVIEWIQ_STATE_ROUND_N -->
<details><summary>ReviewIQ State (Round N) — X open, Y resolved</summary>
...summary table...
</details>
<!-- REVIEWIQ_STATE_START -->
{base64_encoded_state}
<!-- REVIEWIQ_STATE_END -->'
```

**CRITICAL**: You MUST save state after every reviewiq command. Always POST new, never PATCH — previous round states are preserved for history.

### State Schema
```json
{
  "version": 2,
  "pr": { "number": 42, "repo": "owner/repo", "title": "", "author": "", "base_branch": "", "head_branch": "" },
  "review_rounds": [
    { "round": 1, "timestamp": "...", "head_sha": "abc123", "base_sha": "def456", "event": "review", "files_reviewed": [...] }
  ],
  "findings": {
    "1": {
      "id": 1, "severity": "CRITICAL", "status": "open",
      "file": "src/retry.py", "line": 42,
      "title": "Retry without backoff",
      "problem": "...", "impact": "...",
      "suggested_fix": "time.sleep(min(2 ** attempt * 0.5, 30))",
      "fix_rationale": "Exponential backoff prevents thundering herd",
      "created_round": 1,
      "status_history": [
        { "status": "open", "round": 1, "timestamp": "...", "note": "Initial finding" }
      ]
    }
  },
  "summary": { "total_findings": 3, "open": 1, "resolved": 2, "wontfix": 0, "retracted": 0, "assessment": "REQUEST CHANGES", "last_reviewed_sha": "abc123" }
}
```

### Finding Statuses
- `open` — found, not yet fixed
- `resolved` — fix confirmed in code
- `wontfix` — developer won't fix (accepted)
- `retracted` — finding was wrong
- `partially_fixed` — partially addressed

### Round Numbers
Each iteration increments the round: review = round 1, first recheck = round 2, etc.
Round number = `len(review_rounds) + 1` for the current action.

## Skills Loading (Token Optimization)

Load from `~/.reviewiq/skills/` or `.pr-review/skills/`:

**Always load** (6): commandments, security, scalability, stability, maintainability, performance

**Per-file detection** (load only matching SECTIONS, not full files):
- File extension → language section from languages.md
- Import scanning → framework section from frameworks.md
- Filename patterns → devops.md, fintech.md, fraud.md, etc.

**Database skills** (load on the matching triggers below):
- `sql.md` — `.sql` files; raw SQL strings (`SELECT`, `INSERT`, `UPDATE`, `DELETE`, `JOIN`); `cursor.execute`, `db.query`, `conn.exec`
- `migrations.md` — paths `migrations/`, `db/migrate/`, `alembic/versions/`, `flyway/`, `liquibase/`; filenames matching `V<n>__*.sql`, `*_migration.py`, `*.changeset.xml`
- `orm.md` — imports of `django.db`, `sqlalchemy`, `prisma`, `@prisma/client`, `typeorm`, `sequelize`, `mongoose`, `gorm`, `hibernate`, `entity`, `@Entity`
- `transactions.md` — `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `transaction.atomic`, `@Transactional`, `db.transaction`, `SELECT ... FOR UPDATE`, `pg_advisory_lock`, `SETNX`+lock semantics
- `postgres.md` — `psycopg2`, `asyncpg`, `pg`, `node-postgres`, `pgx`, `gorm.io/driver/postgres`; PG-specific syntax (`JSONB`, `RETURNING`, `ON CONFLICT`, `CONCURRENTLY`); `.sql` files with PG dialect
- `redis.md` — `redis`, `ioredis`, `redis-py`, `go-redis`, `lettuce`, `jedis`, `StackExchange.Redis`; commands `SETNX`, `MGET`, `XADD`, `SUBSCRIBE`, `EVAL`
- `mongodb.md` — `pymongo`, `mongoose`, `mongodb` (node), `MongoClient`, `@Document`, aggregation `$match`/`$lookup`/`$group`, `findOneAndUpdate`
- `elasticsearch.md` — `elasticsearch`, `@elastic/elasticsearch`, `opensearch`, `org.elasticsearch.client`; query DSL (`_search`, `bool`, `match`, `term`, `aggs`); index APIs (`_bulk`, `_mapping`)

**Token budget**:
- reviewiq-pr --full: ~5-8K words (all files, all relevant skills)
- reviewiq-pr --interactive: ~2-3K words per file (only that file's skills)
- reviewiq-recheck: ~1-2K words (only changed files + state context)

## reviewiq-pr --full Flow (default)

1. Detect repo, fetch all diffs + file contents
2. Load state from GitHub PR comment (or start fresh, round 1)
3. Load all relevant skills across all files
4. Review with cross-file analysis
5. Post inline comments with ```suggestion blocks
6. Post markdown report as PR comment (iteration report)
7. **SAVE STATE** — create the `<!-- REVIEWIQ_STATE_COMMENT -->` hidden comment with base64 state JSON (see "How to save state" above). This is NOT optional — without it, recheck/resolve will have no history.

## reviewiq-pr --interactive Flow

For each file:
1. Load skills for THIS file only (~2-3K words)
2. Review single file against skills
3. **If findings**: show suggestion + resolution + comment, ask user:
   - `P` (post) — post inline comments for this file
   - `S` (skip) — skip, move to next
   - `F <N>` (fix) — apply suggestion N
4. **If no findings**: auto-move to next file (no prompt)
5. After all files: post summary report, save state to GitHub

## reviewiq-recheck Flow

1. Load state from GitHub PR hidden comment. List ALL state comments (not just one) and pick the highest-round marker. Record each comment's `user.login` so step 5 can detect foreign authorship.
2. **Round number**: `next_round = max(round markers across ALL existing state comments) + 1`. NEVER assume the previous round was the one *we* wrote — CI auto-recheck may have inserted a phantom round between sessions.
3. **Pull latest code**: `git fetch origin && git pull` — ensures local repo has all new commits for incremental diff
4. Fetch current code, compare against `last_reviewed_sha` in state
5. **Re-verify foreign state** — if the loaded state was authored by a user/bot OTHER than the current actor (e.g., a CI bot like `github-actions[bot]`), do NOT trust its `resolved`/`wontfix` statuses. For each such finding:
   - Read the file at the recorded line
   - If the problematic pattern is still present → flip status back to `open` and add a `status_history` entry: `"Foreign state claimed resolved; code still shows the issue (re-verified)"`
   - If genuinely fixed → keep `resolved` and add a `status_history` entry: `"Foreign state confirmed by re-verification"`
6. For each pending finding:
   - Code fixed? → auto-resolve
   - Still broken? → keep pending
   - Was previously `resolved` but problem returned (e.g. revert) → mark `open` with status_history note: `"Regressed: previous fix was reverted in <sha>"`
   - Changed differently? → needs-review
7. Check new changes for new issues
8. **Skip prompt** — list remaining `open` findings (after auto-resolution pass) in a numbered table, then ask:
   ```
   Mark any as wontfix? Type `W <N>` or `W 1,3,5`, or Enter to keep all open:
   ```
   - For each ID marked: set `status: wontfix`, append a `status_history` entry with `note: "Skipped by user during recheck"`
   - **No-op recheck guard**: if head SHA is unchanged AND no statuses changed via this prompt AND no foreign state was rewritten, skip posting a new round report. Still save state if anything was marked wontfix or re-verified.
9. Post NEW PR comment with this round's report (append to timeline, don't overwrite). The report MUST surface **status flips** prominently — any finding whose status changed since the previous round gets a dedicated row showing `previous → current` (e.g. `resolved → open (regressed)`, `open → wontfix`, `resolved (foreign) → open (re-verified)`). Status flips are higher signal than vanilla `open` findings and should be visually distinct (⚠ marker or section).
10. Post inline comments only for NEW findings
11. Save updated state to GitHub PR hidden comment

## reviewiq-resolve Flow

**THIS COMMAND WRITES CODE. It edits source files directly to apply fixes.**

1. Checkout the PR branch: `gh pr checkout <N>`
2. Load state from GitHub PR hidden comment (or conversation history)
3. **Skip prompt** — BEFORE editing any file, list every `open` finding in a numbered table (id / severity / file:line / title), then ask:
   ```
   Mark any as wontfix? Type `W <N>` or `W 1,3,5`, or Enter to fix all:
   ```
   - For each ID marked: set `status: wontfix`, append a `status_history` entry with `note: "Skipped by user during resolve"`. Skip these in step 4.
   - If user hits Enter: proceed with all open findings.
   - If ALL findings are marked wontfix: skip steps 4–5 (no edits, no commit), jump to step 6 to save state and post a "no fixes applied — N findings marked wontfix" report. Do NOT auto-approve unless the user explicitly says so.
4. For EACH remaining open finding:
   a. Read the target file using the Read tool
   b. Locate the problematic code at the finding's line number
   c. **EDIT the file** — use the Edit tool to replace the broken code with `suggested_fix`
   d. If `suggested_fix` is unclear, write the correct fix using your judgment
   e. Mark the finding as `resolved` in state
   f. **If user rejects the Edit**: do NOT silently stop. Ask:
      ```
      Edit rejected on finding #<id>. Mark as wontfix and continue? [Y/n]
      ```
      - `Y` (default): set `status: wontfix` with note `"Edit rejected by user during resolve"`, move to next finding
      - `n`: abort — save state with this finding still `open`, post a partial-resolution report listing applied vs. aborted, do NOT approve
5. **RUN TESTS** — detect test framework, run tests for modified files, fix until green. If no tests: run linter (flake8/mypy/eslint/tsc). If no linter: at minimum syntax checks.
6. **COMMIT AND PUSH** — `git add`, `git commit -m "fix: resolve ReviewIQ findings"`, `git push`. Without push, the PR still has broken code and approval is meaningless. Skip if no edits were applied.
7. **SAVE STATE** — update the `<!-- REVIEWIQ_STATE_COMMENT -->` hidden comment (mark resolved/wontfix per finding)
8. Post resolution report as PR comment listing every fix applied + skipped findings + test results
9. Auto-approve PR: `gh pr review <N> --repo <REPO> --approve --body "All findings resolved and tests passing — ReviewIQ"` — only if at least one fix was applied AND no findings remain `open`

**Key**: resolve = checkout + SKIP PROMPT + APPLY fixes + RUN TESTS + COMMIT + PUSH + SAVE STATE + approve.

## reviewiq-test Flow

Standalone test command — run tests for PR changes without resolving findings.

1. Detect repo and changed files (from PR diff or branch diff)
2. Detect test framework:
   - Python: pytest, unittest (pytest.ini, conftest.py, pyproject.toml)
   - JS/TS: jest, vitest, mocha (package.json scripts)
   - Go: `go test`
   - Java: maven/gradle
   - Ruby: rspec, minitest
3. Run targeted tests for changed files first, then full suite if needed
4. Also run linter/type-checker if available
5. Report: list each test run, pass/fail, error output for failures
6. If no tests exist for changed code, suggest what tests should be written

## Posting Inline Comments via gh API

**CRITICAL**: Do NOT use `/pulls/{N}/comments` — it rejects `line`/`subject_type`.
Use the **Reviews API** (`/pulls/{N}/reviews`) which posts all inline comments in one batch.

### Pre-flight: validate every inline `line` against diff hunks (MANDATORY)

GitHub's reviews API rejects with `422 Line could not be resolved` if the line isn't inside a hunk that was actually changed. Validate **before** posting — every retry costs a full round-trip.

```bash
# Build {file -> [(hunk_start_new, hunk_end_new), ...]} from the unified diff
gh pr diff {N} --repo {owner}/{repo} --patch > /tmp/reviewiq_diff.patch

# For each finding (file, line):
# 1. Parse hunks from /tmp/reviewiq_diff.patch — for each `@@ -a,b +c,d @@` in `+++ b/<file>`,
#    the hunk covers NEW-file lines [c, c+d-1].
# 2. If (file, line) falls inside ANY hunk → keep as inline.
# 3. Else → demote to PR-level (post in the summary report body, not in `comments`).
```

After validation, the `comments` array passed to `/pulls/{N}/reviews` must contain ONLY findings whose `(path, line)` lies within a hunk. Demoted findings still appear in the markdown summary so the developer sees them — they just don't anchor inline.

### Post review with inline comments

**Use Bash with `cat` heredoc to create `/tmp/reviewiq_payload.json`** — do NOT use the Write tool (path resolution issues) or Edit tool (file won't exist) or `echo` pipe (backticks break shell escaping). Always use Bash with a single-quoted heredoc delimiter.

```bash
HEAD_SHA=$(gh pr view {N} --repo {owner}/{repo} --json headRefOid -q .headRefOid)

# 1. Write JSON to temp file (single-quoted heredoc = no shell interpretation)
cat > /tmp/reviewiq_payload.json << 'ENDOFJSON'
{
  "commit_id": "HEAD_SHA_PLACEHOLDER",
  "body": "## ReviewIQ Review\n\n**X findings** | Assessment: **REQUEST CHANGES**",
  "event": "COMMENT",
  "comments": [
    {
      "path": "src/auth.py",
      "line": 42,
      "body": "**[CRITICAL] Title**\n\nProblem.\n\n```suggestion\nfix code\n```\n\n_Why_: rationale"
    }
  ]
}
ENDOFJSON

# 2. Replace placeholder with actual SHA
sed -i '' "s/HEAD_SHA_PLACEHOLDER/$HEAD_SHA/" /tmp/reviewiq_payload.json

# 3. Post
gh api repos/{owner}/{repo}/pulls/{N}/reviews --input /tmp/reviewiq_payload.json
```

**Key rules**:
- Always use temp file + `--input`, never `echo` pipe (backticks break shell)
- `event`: `"COMMENT"`, `"REQUEST_CHANGES"`, or `"APPROVE"`
- Each comment: `path` (string, must be a file in the PR diff), `line` (integer, line in NEW file), `body` (string)
- `commit_id`: PR head SHA (required)
- All findings go in ONE review call — do NOT post individual comments
- `path` MUST be a file that exists in the PR diff, otherwise GitHub returns "Path could not be resolved"

### Post PR-level comment (for summary reports)
```bash
gh pr comment {N} --repo {owner}/{repo} --body "$(cat <<'EOF'
report markdown here
EOF
)"
```

### Approve PR
```bash
gh pr review {N} --repo {owner}/{repo} --approve --body "All findings resolved."
```

## Rules

1. Never hallucinate file contents — always read the file
2. Concrete suggestions only — exact replacement code for ```suggestion blocks
3. Match repo conventions
4. Auto-skip files with no findings (no unnecessary prompts)
5. Token efficiency — load only relevant skill sections per file
6. State is on GitHub — always load from PR comment before acting, save after every action
7. Round numbers are incremental — never reuse a round number
8. Reports are markdown — each round is a NEW PR comment (timeline history)
