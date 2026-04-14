Review the PR on branch: $ARGUMENTS

Follow the stateful review agent protocol defined in .pr-review/agent.md.

## State Management

1. Check for existing state:
   ```
   ls .pr-review/reviews/
   ```
   If a state file exists for this PR, read it — it contains prior findings, their statuses, and conversation history. You have continuity.

2. If no state exists, this is the first review. You'll create the state file after reviewing.

## Context Assembly

1. Determine base branch:
   ```
   git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@'
   ```
   Fall back to `main` if that fails.

2. Get the full diff:
   ```
   git diff <base>...$ARGUMENTS
   ```

3. Read ALL changed files in full — never review from diff alone.

4. For each non-trivial symbol introduced or modified in the diff, trace references:
   ```
   git grep -n <symbol>
   ```

5. Read recent history of changed files:
   ```
   git log -5 --follow <each changed file>
   ```

## Execute Review

If this is the **first review** (no state file):
- Run the full 4-stage review pipeline from .pr-review/agent.md
- After reporting, create the state file at `.pr-review/reviews/pr-<N>.json`
- Save each finding as a structured object with status `open`

If this is a **returning session** (state file exists):
- Load the state — you already know the findings, their statuses, and prior discussion
- Show current status summary
- Ask what the developer wants to do next

## State File Format

After review, write `.pr-review/reviews/pr-<number>.json`:
```json
{
  "version": 2,
  "pr": {
    "number": 0,
    "repo": "",
    "title": "PR title",
    "author": "username",
    "base_branch": "main",
    "head_branch": "feature/branch"
  },
  "review_rounds": [
    {
      "round": 1,
      "timestamp": "ISO8601",
      "head_sha": "abc123",
      "base_sha": "def456",
      "event": "review",
      "files_reviewed": ["file1.ts", "file2.ts"]
    }
  ],
  "findings": {
    "1": {
      "id": 1,
      "title": "Short title",
      "severity": "CRITICAL",
      "status": "open",
      "file": "path/to/file.ext",
      "line": 42,
      "problem": "Description of the problem",
      "impact": "What breaks",
      "suggested_fix": "Code fix",
      "fix_rationale": "Why this approach",
      "created_round": 1,
      "created_at": "ISO8601",
      "updated_at": "ISO8601",
      "status_history": [
        {"status": "open", "round": 1, "timestamp": "ISO8601"}
      ],
      "discussion": []
    }
  },
  "conversation": [],
  "summary": {
    "total_findings": 0,
    "open": 0,
    "resolved": 0,
    "wontfix": 0,
    "retracted": 0,
    "assessment": "PENDING",
    "last_reviewed_sha": ""
  }
}
```

## After Review — Available Commands

With state loaded, I can:
- `explain <N>` — deep dive into finding N (loads from state, traces code)
- `fix <N>` — apply the fix and transition finding to resolved
- `check` — re-review after you push changes (incremental diff against last reviewed SHA)
- `impact` — trace blast radius of changes
- `test` — generate test cases targeting open findings
- `compare <approach>` — evaluate an alternative
- `retract <N>` — retract a finding (I was wrong)
- `wontfix <N>` — accept developer's reasoning for not fixing
- `status` — show current state of all findings
- `approve` — final check for remaining blockers
- `summarize` — generate merge commit summary

Every action updates the state file so the next session picks up where we left off.
