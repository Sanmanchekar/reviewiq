<div align="center">

![ReviewIQ Banner](https://img.shields.io/badge/🔍_ReviewIQ-AI_PR_Review_Agent-blue?style=for-the-badge)

# ReviewIQ

### Stateful AI-Powered PR Review Agent with Domain Expert Skills

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://choosealicense.com/licenses/mit/)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/Sanmanchekar/reviewiq/releases)
[![Go](https://img.shields.io/badge/go-%3E%3D1.22-00ADD8.svg)](https://go.dev)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-Compatible-purple.svg)](https://claude.ai/code)
[![GitHub Stars](https://img.shields.io/github/stars/Sanmanchekar/reviewiq?style=social)](https://github.com/Sanmanchekar/reviewiq/stargazers)

**4 commands. 16 review skills. Review PRs, post inline suggestions, track findings across iterations, fix code and auto-approve.**

[Quick Start](#quick-start) |
[Commands](#commands) |
[State Storage](#state-storage) |
[Skills System](#skills-system) |
[Architecture](#architecture)

</div>

---

## Overview

ReviewIQ reviews PRs using domain expert skill modules — security, performance, fintech, DevOps, and more. Findings are tracked with statuses (open/resolved) in hidden GitHub PR comments, and the agent posts inline suggestions directly on the PR.

| Command | What it does |
|---------|-------------|
| **`reviewiq-pr`** | Review PR — `--full` (default) or `--interactive` (file-by-file) |
| **`reviewiq-recheck`** | Re-review — auto-resolves fixed findings, flags new issues |
| **`reviewiq-resolve`** | Fix code, run tests, commit+push, mark resolved, auto-approve |
| **`reviewiq-test`** | Run tests for PR changes — detect framework, targeted + full suite |

**Input**: PR link, PR number, or branch. **Flags**: `--full` (default), `--interactive`.

---

## Quick Start

### Install

```bash
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
```

Auto-installs all dependencies (Go, git, gh CLI) if missing. Then builds binary, copies 16 skills, sets up Claude Code config.

### Update / Uninstall

```bash
# Update
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash

# Uninstall
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/uninstall.sh | bash
```

### Usage

**Claude Code** (no API key needed):
```bash
# Full review (default) — all files, auto-posts to PR
/reviewiq-pr https://github.com/owner/repo/pull/42
/reviewiq-pr 42 --full                   # same, explicit flag

# File-by-file — confirm post/skip per file
/reviewiq-pr 42 --interactive

# Re-review — check what's fixed, what's still open
/reviewiq-recheck 42

# Resolve — fix all findings, test, push, approve
/reviewiq-resolve 42

# Run tests for changed files
/reviewiq-test 42

# Natural language
review this PR                    # acts like reviewiq-pr --full
review this PR interactively      # acts like reviewiq-pr --interactive
recheck                           # acts like reviewiq-recheck
resolve / approve                 # acts like reviewiq-resolve
run tests                         # acts like reviewiq-test
```

**CLI** (needs `ANTHROPIC_API_KEY`):
```bash
reviewiq full https://github.com/owner/repo/pull/42         # full review + post
reviewiq pr 42 --post                                        # file-by-file + post
reviewiq check feature/xyz                                   # incremental re-review
reviewiq status                                              # show findings
reviewiq approve                                             # check blockers + approve
```

---

## Commands

### `/reviewiq-pr` — Review PR

One command, two modes:

```
/reviewiq-pr 42                    # --full (default): all files, auto-post
/reviewiq-pr 42 --full             # same, explicit
/reviewiq-pr 42 --interactive      # file-by-file, post/skip per file
```

**Each finding includes**: suggestion (exact code), resolution (how to fix), comment (context).

#### `--full` (default)

All files at once with cross-file analysis. Auto-posts inline comments + report.

1. Load all relevant skills (~5-8K words)
2. 4-stage review: Understand -> Analyze -> Assess -> Report
3. Post `suggestion` blocks on each finding line
4. Post markdown report as PR comment
5. Save state to GitHub PR hidden comment

#### `--interactive`

One file at a time, only that file's skills loaded (~2-3K words).

```
File 1/4: src/webhooks/retry.py
Skills: python, django (~2K words)

  Finding 1: [CRITICAL] Retry without backoff — line 42
  Suggestion: time.sleep(min(2 ** attempt * 0.5, 30))
  Resolution: Add exponential backoff with jitter

  [P] Post    [S] Skip    [F 1] Fix
```

- **Findings found** -> `P` (post) / `S` (skip) / `F <N>` (fix)
- **No findings** -> auto-moves to next file

#### PR Timeline

Each round is a NEW comment (previous rounds stay visible):
```
ReviewIQ Report — Round 1 (3 findings, REQUEST CHANGES)
ReviewIQ Report — Round 2 (2 resolved, 1 new, NEEDS DISCUSSION)
ReviewIQ Resolution — All resolved, APPROVED
```

---

### `/reviewiq-recheck` — Re-review with History

Loads previous state from GitHub PR comment, checks what's fixed, what's still open, flags new issues.

```
/reviewiq-recheck https://github.com/owner/repo/pull/42
```

**Flow**:
1. Load state from GitHub PR hidden comment
2. Increment round number
3. Fetch current code, compare against last reviewed SHA
4. For each pending finding:
   - Code fixed? -> **auto-resolve**
   - Still broken? -> **keep pending**
   - Changed differently? -> **needs-review**
5. Check new changes for new issues
6. Post update to PR
7. Save updated state to GitHub PR hidden comment

**Output**:
```
Re-review Report — Round 2

| # | Severity | Previous | Current | Note |
|---|----------|----------|---------|------|
| 1 | CRITICAL | pending  | resolved | Backoff added |
| 2 | IMPORTANT| pending  | pending  | Still missing idempotency |
| 3 | NIT      | pending  | resolved | Error message fixed |
| 4 | IMPORTANT| —        | NEW      | Null check missing |

Previously: 3 findings -> Now: 2 open, 2 resolved
Assessment: REQUEST CHANGES -> NEEDS DISCUSSION
```

---

### `/reviewiq-resolve` — Fix All, Test & Approve

Checks out the PR branch, applies suggested fixes to code, runs tests, commits+pushes, and auto-approves.

```
/reviewiq-resolve https://github.com/owner/repo/pull/42
```

**Flow**:
1. Checkout PR branch (`gh pr checkout <N>`)
2. Load state from GitHub PR hidden comment
3. For each open finding: apply `suggested_fix` to the target file
4. Run tests (or linter, or syntax checks as fallback)
5. Commit and push fixes to the PR branch
6. Save state, post resolution report
7. Auto-approve PR via `gh pr review --approve`

**Output**:
```
ReviewIQ Resolution — PR #42

Applied fixes:
  1. [CRITICAL] Retry without backoff — retry.py:42 -> backoff added
  2. [IMPORTANT] Missing idempotency — handler.py:18 -> idempotency key added
  3. [NIT] Typo in error message — utils.py:92 -> fixed

Tests: 12 passed, 0 failed
Commit: abc1234 "fix: resolve ReviewIQ findings for PR #42"
Pushed to branch: feature/webhooks

All 3 findings resolved. PR APPROVED.
```

---

### `/reviewiq-test` — Run Tests

Standalone test runner for PR changes. Detects framework, runs targeted tests.

```
/reviewiq-test 42
```

**Flow**:
1. Detect changed files from PR diff
2. Auto-detect test framework (pytest, jest, go test, maven, rspec, etc.)
3. Run targeted tests for changed files, then full suite if needed
4. Run linter/type-checker if available
5. Report pass/fail with error output

---

## State Storage

State lives in a **hidden GitHub PR comment** — no local files. This makes state available across machines, CI, and team members.

### Finding Statuses

| Status | Meaning |
|--------|---------|
| `open` | Found, not yet fixed |
| `resolved` | Fix applied/confirmed |
| `wontfix` | Developer won't fix (accepted) |
| `retracted` | Finding was wrong |
| `partially_fixed` | Partially addressed |

### Round Numbers

Each iteration increments: review = round 1, first recheck = round 2, etc.

---

## Skills System

Auto-detects languages, frameworks, and domains from changed files. Loads only relevant skill sections — not full files.

### Always Loaded (6)

| Skill | What it covers |
|-------|----------------|
| **Commandments** | 40 universal laws: correctness, security, reliability, data, APIs, testing |
| **Security** | Injection, auth, crypto, data protection, dependencies (OWASP-aligned) |
| **Scalability** | Database, caching, concurrency, network, compute, architecture |
| **Stability** | Error handling, resilience, observability, deployment safety |
| **Maintainability** | Complexity, naming, organization, testability, refactoring |
| **Performance** | Algorithms, memory, database, I/O, CPU, frontend, caching |

### Auto-Detected

| Skill | Triggers on |
|-------|-------------|
| **Languages** | `.py`, `.java`, `.go`, `.ts`, `.cpp`, `.rs`, `.cs`, `.rb`, `.php`, `.sh` |
| **Frameworks** | Django, FastAPI, Flask, Spring, React, Next.js, Express, Vue, Angular, Rails, .NET |
| **DevOps** | Dockerfile, Chart.yaml, `*.tf`, K8s manifests, CI configs |
| **Fintech** | stripe, razorpay, payment, loan, emi, insurance, ledger, kyc |
| **India Regulatory** | upi, nach, aadhaar, rbi, nbfc, ifsc |
| **Credit Bureau** | cibil, experian, equifax, credit_score |
| **Fraud** | fraud, risk_engine, velocity, device_fingerprint |
| **Notifications** | sms, twilio, sendgrid, fcm, whatsapp |
| **Financial Microservices** | saga, outbox, event_sourcing, kafka |
| **Data Privacy** | gdpr, ccpa, dpdp, consent, pii |

### Token Budget

| Mode | Skills loaded | Per-call cost |
|------|-------------|---------------|
| `reviewiq-pr --full` | All relevant across all files | ~5-8K words |
| `reviewiq-pr --interactive` | Per-file only | ~2-3K words/file |
| `reviewiq-recheck` | Only changed files | ~1-2K words |

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        REVIEWIQ PLATFORM                         │
├──────────────┬───────────────────────┬───────────────────────────┤
│ Input        │   Processing          │   Output                  │
├──────────────┼───────────────────────┼───────────────────────────┤
│ PR link      │ Skill Detection       │ Inline PR comments        │
│ PR number    │  └ per-file loading   │ ```suggestion blocks      │
│ Branch       │ 4-Stage Review        │ Markdown reports          │
│ Prior state  │  └ skill-guided       │ GitHub state comment      │
│              │ State Manager         │ Commit + push fixes       │
│              │  └ GitHub PR comment  │                           │
├──────────────┼───────────────────────┼───────────────────────────┤
│              │ Claude Code: no key   │                           │
│              │ CLI: Claude API       │                           │
└──────────────┴───────────────────────┴───────────────────────────┘
```

---

## CI Integration (GitHub Actions)

ReviewIQ can auto-review PRs in CI. Copy `.github/workflows/pr-review.yml` to your repo and add `ANTHROPIC_API_KEY` to repo secrets.

### Automatic triggers

| Event | What happens |
|---|---|
| PR opened | Full review with inline `suggestion` comments |
| PR push (new commits) | Incremental re-review — auto-resolves fixed findings, flags new issues |

### Comment commands

Comment on any PR to trigger actions:

```
@review-agent                     → ask a question about the PR
@review-agent resolve             → verify all fixes applied, approve if clear
@review-agent recheck             → re-review with full history
@review-agent test                → run tests for changed files
@review-agent explain finding 3   → deep dive into a specific finding
```

### CI ↔ Slash command mapping

| CI trigger | Slash command equivalent |
|---|---|
| PR opened | `/reviewiq-pr --full` |
| PR push | `/reviewiq-recheck` |
| `@review-agent resolve` | `/reviewiq-resolve` |
| `@review-agent recheck` | `/reviewiq-recheck` |
| `@review-agent test` | `/reviewiq-test` |

### Setup

1. Copy workflow: `cp .github/workflows/pr-review.yml <your-repo>/.github/workflows/`
2. Add secret: `ANTHROPIC_API_KEY` in repo Settings → Secrets
3. `GITHUB_TOKEN` is auto-provided by GitHub Actions — no setup needed

---

## Configuration

**No tokens needed locally.** ReviewIQ reuses your existing `gh` CLI auth — if you can push code, you can review PRs.

| Variable | Context | Description |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | CLI binary only | **Not needed for Claude Code.** |
| `GITHUB_TOKEN` | GitHub Actions only | Auto-provided by GitHub. **Not needed locally.** |

> **Prerequisite**: `gh auth login` (one-time). If you already use `gh` or GitHub Desktop, you're set.

---

## File Structure

```
# Installed globally
~/.local/bin/reviewiq             CLI binary
~/.reviewiq/skills/               16 skill modules
~/.claude/REVIEWIQ.md             Claude Code global config
~/.claude/commands/reviewiq-*.md  4 slash commands (global, work in every repo)

# Per-repo
.pr-review/skills/                Skill files (customizable per repo)
.pr-review/agent.md               Review protocol

# State (no local files)
GitHub PR hidden comment           Base64-encoded state in <!-- REVIEWIQ_STATE_COMMENT -->

# Source code
cmd/reviewiq/main.go              CLI (Go + cobra)
internal/
  state/ engine/ git/ skills/ github/ ci/
install.sh                        One-line installer
uninstall.sh                      One-line uninstaller
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.
