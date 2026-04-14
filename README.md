# ReviewIQ

Stateful PR review agent that works across any AI coding tool. Findings are tracked objects with lifecycles, conversations carry full history, and incremental reviews diff against known state.

## Architecture

```
Developer opens PR ──→ Agent posts initial review
                              │
                         State saved
                       (findings, SHAs,
                        conversation)
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
     Claude Code          Codex/Cursor        GitHub Comment
     "/review-pr feat"    "review this PR"    "@review-agent why?"
          │                   │                   │
          └───────────────────┼───────────────────┘
                              ▼
                      Stateful Agent
                    (loads prior findings,
                     conversation history,
                     review round SHAs)
                              │
                         State updated
                              │
                              ▼
                    Response in same surface
```

**The key difference from stateless**: the agent doesn't re-derive findings by parsing old comments. It loads structured state — each finding is a tracked object with an ID, lifecycle status, and discussion thread. Conversation history is sent as proper multi-turn messages, so the LLM has genuine conversational memory.

## State Model

State is persisted as JSON with dual backends:

| Backend | Where | When |
|---------|-------|------|
| **Local file** | `.pr-review/reviews/pr-<N>.json` | CLI mode (Claude Code, Cursor, etc.) |
| **PR comment** | Hidden comment with base64 payload | CI mode (GitHub Actions) |

Both stay in sync when running in CI. CLI mode writes local files only.

### What's Tracked

```
State
├── pr                    PR metadata (title, author, branches)
├── review_rounds[]       Each review pass with SHA + timestamp
│   └── {round, head_sha, base_sha, files_reviewed}
├── findings{}            Tracked objects with lifecycle
│   └── {id, title, severity, status, file, line,
│        problem, impact, suggested_fix,
│        status_history[], discussion[]}
├── conversation[]        Full message history for LLM context
│   └── {role, content, round, timestamp}
└── summary               Computed counters + assessment
    └── {open, resolved, wontfix, retracted, assessment}
```

### Finding Lifecycle

```
open ──→ resolved          developer fixed it
     ──→ partially_fixed   partially addressed
     ──→ wontfix           developer won't fix, agent accepts
     ──→ retracted         agent was wrong

partially_fixed ──→ resolved | wontfix
```

Every transition is recorded with timestamp, round number, and note.

## Quick Start

### Option 1: Claude Code (zero setup)

Copy `.pr-review/` and `.claude/commands/` into your repo:

```bash
# First review — creates state file
/review-pr feature/webhook-retries

# Continue the conversation — state is loaded automatically
> "explain finding 2"
> "fix finding 1"           # transitions finding 1 to resolved
> "retract finding 3"       # agent was wrong about this one

# After developer pushes fixes
> "check"                   # incremental diff against last reviewed SHA
                            # reports: resolved / partially fixed / new issues

# Check current state anytime
> "status"                  # table of all findings with current statuses
```

State persists at `.pr-review/reviews/pr-<N>.json` — pick up where you left off in any session.

### Option 2: Any AI Agent (Codex, Cursor, Aider, Cline)

Drop `.pr-review/agent.md` into your repo. The protocol instructs any agent to read/write the state file:

```
"Review the PR on branch feature/webhook-retries, following the protocol in .pr-review/agent.md"
```

The agent will create `.pr-review/reviews/pr-<N>.json` on first review and load it on subsequent interactions.

### Option 3: GitHub Actions (fully automated)

1. Add your Anthropic API key as a repository secret:
   ```
   Settings > Secrets > Actions > New repository secret
   Name: ANTHROPIC_API_KEY
   ```

2. Copy these into your repo:
   - `.pr-review/agent.md`
   - `scripts/state.py`
   - `scripts/review_agent.py`
   - `.github/workflows/pr-review.yml`

3. Open a PR. The agent:
   - Posts a structured review
   - Saves state as a hidden PR comment (survives across workflow runs)
   - On subsequent pushes, loads state and posts incremental re-review
   - On `@review-agent` comments, loads full conversation history

4. Interact via comments:
   ```
   @review-agent explain finding 2
   @review-agent why is finding 1 critical? our volume is only 100/day
   @review-agent status
   ```

## Interaction Commands

| Command | What happens | State change |
|---|---|---|
| `review` | Full 4-stage review | Creates findings (open), saves state |
| `explain <N>` | Deep dive with code tracing | Adds to finding's discussion thread |
| `fix <N>` | Apply the suggested fix | Finding → resolved |
| `check` | Incremental re-review | Updates statuses, new review round |
| `impact` | Trace blast radius | Read-only |
| `test` | Generate test cases for open findings | Read-only |
| `compare <approach>` | Evaluate alternative | May update finding's suggested_fix |
| `retract <N>` | Agent was wrong | Finding → retracted |
| `wontfix <N>` | Developer won't fix (accepted) | Finding → wontfix |
| `status` | Current state summary | Read-only |
| `approve` | Final check | Assessment → APPROVE (if clear) |
| `summarize` | Merge commit summary | Read-only |

## Incremental Re-review

The killer feature, now with state tracking:

```
Developer: "pushed fixes, check again"

Agent loads state:
  - Knows exactly which SHA it last reviewed
  - Knows all 3 open findings and their details
  - Diffs only the changes since last review

Agent response:
  ## Re-review (Round 2)
  Changes since: abc1234...def5678

  - Finding 1 [CRITICAL]: ~~open~~ → **resolved** — backoff with jitter added
  - Finding 2 [IMPORTANT]: open → **partially_fixed** — retry limit added, no circuit breaker
  - Finding 4 [NIT]: **NEW** — typo in error message on line 92

  Summary: 1 resolved, 1 partial, 1 new nit
  Assessment: REQUEST CHANGES → NEEDS DISCUSSION
```

No re-parsing old comments. No guessing what was fixed. The agent knows.

## Conversation Continuity

The stateful engine sends **full conversation history** to Claude as multi-turn messages, not a single reconstructed prompt. This means:

```
Developer: "finding 2 — won't adding a lock here cause deadlock 
            with the existing mutex on line 40?"

Agent: *already has full context from prior turns*
       "You're right — there's a lock ordering issue..."

Developer: "ok, what about using a channel instead?"

Agent: *references both prior messages naturally*
       "That would solve the ordering issue. Updating finding 2's
        suggested fix to use a buffered channel. The original mutex
        approach had the deadlock risk you identified."
       *updates finding 2's suggested_fix in state*
```

## Configuration

### Environment Variables (CI mode)

| Variable | Required | Default | Description |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | Yes | — | Claude API key |
| `GITHUB_TOKEN` | Yes | — | GitHub token (auto-provided in Actions) |
| `MODEL` | No | `claude-sonnet-4-6-20250514` | Claude model to use |
| `MAX_TOKENS` | No | `8192` | Max response tokens |

### Customization

Edit `.pr-review/agent.md` to customize:
- Review focus areas (add domain-specific checks)
- Severity criteria (adjust for your team's standards)
- Finding lifecycle (add custom statuses)
- Additional commands

## File Structure

```
.pr-review/
  agent.md                    # Review protocol (portable, works in any agent)
  reviews/                    # State files (gitignored or committed, your choice)
    pr-42.json                # State for PR #42
    pr-43.json
.claude/
  commands/
    review-pr.md              # Claude Code slash command
scripts/
  state.py                    # State manager (dual backend: local + GitHub)
  review_agent.py             # Stateful CI agent engine
  review-agent.sh             # Legacy stateless script (kept for reference)
.github/
  workflows/
    pr-review.yml             # GitHub Actions workflow
```

### Git Strategy for State Files

**Option A — Gitignore** (recommended for most teams):
```gitignore
# .gitignore
.pr-review/reviews/
```
State lives in PR comments (CI) or locally (CLI). No repo clutter.

**Option B — Commit** (for teams that want state in version control):
State files are committed. Useful if you want to track review history in git.

## The Automation Spectrum

```
Manual            Semi-auto              Fully automated
----------------------------------------------------------
Developer runs    CI posts initial       CI posts review +
/review-pr in     review with state,     auto-applies nit
their agent       developer interacts    fixes + re-reviews
of choice         via CLI or comments    

     ◄── start here    grow into ──►
```

State works identically at every level — same JSON, same lifecycle, same conversation history.
