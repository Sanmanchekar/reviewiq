# ReviewIQ

Stateful AI-powered PR review agent with finding lifecycle tracking. Works as a CLI, in any AI coding tool, and as a GitHub Actions bot.

## Install

```bash
pip install git+https://github.com/Sanmanchekar/reviewiq.git
```

Or clone and install locally:

```bash
git clone https://github.com/Sanmanchekar/reviewiq.git
cd reviewiq
pip install .
```

Two entry points are available: `reviewiq` and `riq` (shorthand).

## Quick Start

```bash
# Initialize in your repo
reviewiq init

# Review a PR branch
reviewiq review feature/webhook-retries

# Check findings
reviewiq status

# After developer pushes fixes
reviewiq check feature/webhook-retries

# Deep dive into a finding
reviewiq explain 2

# Ask a follow-up question
reviewiq ask "why is finding 1 critical? our volume is only 100/day"

# Transition findings
reviewiq resolve 1 --note "backoff added with jitter"
reviewiq retract 3 --note "ORM handles parameterization"
reviewiq wontfix 2 --note "acceptable risk at our scale"

# Final check
reviewiq approve
```

## CLI Reference

```
reviewiq init                          Initialize .pr-review/ in current repo
reviewiq review <branch> [--base main] Full review of a PR branch
reviewiq check <branch>                Incremental re-review after new commits
reviewiq status [--pr N]               Show all findings with current statuses
reviewiq explain <finding-id>          Deep dive into a specific finding
reviewiq ask <question>                Ask a follow-up question
reviewiq resolve <finding-id> [-n ""]  Mark finding as resolved
reviewiq retract <finding-id> [-n ""]  Retract a finding (agent was wrong)
reviewiq wontfix <finding-id> [-n ""]  Mark as won't fix
reviewiq approve                       Final check — any blockers left?
reviewiq ci                            Run in CI mode (GitHub Actions)
```

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
     CLI (reviewiq)      AI Agent (any)       GitHub Comment
     reviewiq review     "review this PR"     @review-agent
          │                   │                   │
          └───────────────────┼───────────────────┘
                              ▼
                      Stateful Engine
                    (loads prior findings,
                     conversation history,
                     review round SHAs)
                              │
                         State updated
                              │
                              ▼
                    Response in same surface
```

## State Model

State persists as JSON with dual backends:

| Backend | Where | When |
|---------|-------|------|
| **Local file** | `.pr-review/reviews/pr-<N>.json` | CLI mode |
| **PR comment** | Hidden comment with base64 payload | CI mode (GitHub Actions) |

### Finding Lifecycle

```
open ──→ resolved          developer fixed it
     ──→ partially_fixed   partially addressed
     ──→ wontfix           developer won't fix, accepted
     ──→ retracted         agent was wrong

partially_fixed ──→ resolved | wontfix
```

Every transition is recorded with timestamp, round number, and note.

### What's Tracked

```
State
├── pr                    PR metadata
├── review_rounds[]       Each review pass with SHA + timestamp
├── findings{}            Tracked objects with lifecycle
│   └── {id, severity, status, file, line, problem, 
│        suggested_fix, status_history[], discussion[]}
├── conversation[]        Full message history for LLM context
└── summary               Computed counters + assessment
```

## Using with AI Coding Tools

### Claude Code

Copy `.pr-review/` and `.claude/commands/` into your repo:

```bash
/review-pr feature/webhook-retries
```

Then continue conversationally — state persists in `.pr-review/reviews/`.

### Cursor / Codex / Aider / Cline

Drop `.pr-review/agent.md` into your repo and reference it:

```
Review the PR on branch feature/webhook-retries, following .pr-review/agent.md
```

## GitHub Actions (Automated)

1. Add `ANTHROPIC_API_KEY` as a repository secret

2. The workflow at `.github/workflows/pr-review.yml` handles:
   - PR opened → full review
   - Push to PR → incremental re-review
   - `@review-agent <question>` in comments → contextual reply

State survives across workflow runs via hidden PR comments.

## Finding Severity Levels

- **`[CRITICAL]`** — Bugs, data loss, security vulnerabilities. Must fix.
- **`[IMPORTANT]`** — Poor error handling, race conditions, perf issues. Should fix.
- **`[NIT]`** — Style, naming, minor improvements. Won't block.
- **`[QUESTION]`** — Looks odd, might be intentional. Needs clarification.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | Claude API key (required) |
| `MODEL` | `claude-sonnet-4-6-20250514` | Claude model |
| `MAX_TOKENS` | `8192` | Max response tokens |
| `GITHUB_TOKEN` | — | GitHub token (CI only) |

## File Structure

```
src/reviewiq/
  __init__.py               Package init
  __main__.py               python -m reviewiq support
  cli.py                    CLI entry point (reviewiq / riq commands)
  engine.py                 Core review logic (Claude API, state updates)
  state.py                  State manager (dual backend: local + GitHub)
  git.py                    Git operations
  ci.py                     CI mode (GitHub Actions webhook handler)
  templates/
    agent.md                Default agent protocol template
.pr-review/
  agent.md                  Review protocol (customize per repo)
.claude/commands/
  review-pr.md              Claude Code slash command
.github/workflows/
  pr-review.yml             GitHub Actions workflow
```

## License

MIT License. See [LICENSE](LICENSE).
