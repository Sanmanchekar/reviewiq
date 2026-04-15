<div align="center">

![ReviewIQ Banner](https://img.shields.io/badge/🔍_ReviewIQ-AI_PR_Review_Agent-blue?style=for-the-badge)

# ReviewIQ

### Stateful AI-Powered PR Review Agent with Domain Expert Skills

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://choosealicense.com/licenses/mit/)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/Sanmanchekar/reviewiq/releases)
[![Go](https://img.shields.io/badge/go-%3E%3D1.22-00ADD8.svg)](https://go.dev)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-Compatible-purple.svg)](https://claude.ai/code)
[![GitHub Stars](https://img.shields.io/github/stars/Sanmanchekar/reviewiq?style=social)](https://github.com/Sanmanchekar/reviewiq/stargazers)

**3 commands. 16 review skills. Review PRs, post inline suggestions, track findings across iterations.**

[Quick Start](#quick-start) |
[Commands](#commands) |
[Review Folder](#review-folder) |
[Skills System](#skills-system) |
[Architecture](#architecture)

</div>

---

## Overview

ReviewIQ reviews PRs using domain expert skill modules — security, performance, fintech, DevOps, and more. Findings are tracked with statuses (pending/resolved), iterations are saved as markdown reports, and the agent posts inline suggestions directly on the PR.

| Command | What it does |
|---------|-------------|
| **`reviewiq-full`** | Full review — all files at once, auto-posts to PR |
| **`reviewiq-pr`** | Interactive — file-by-file, user confirms post/skip per file |
| **`reviewiq-recheck`** | Re-review — loads history, auto-resolves fixed findings, flags new issues |

**Input**: PR link, PR number, or branch — all three commands accept any format.

---

## Quick Start

### Install

```bash
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
```

Installs: Go binary + gh CLI + 16 skills + Claude Code config. One command, everything ready.

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
# Full review — posts everything to PR
/reviewiq-full https://github.com/owner/repo/pull/42

# File-by-file — confirm post/skip per file
/reviewiq-pr https://github.com/owner/repo/pull/42

# Re-review — check what's fixed, what's still open
/reviewiq-recheck https://github.com/owner/repo/pull/42

# Also works with PR number or branch
/reviewiq-full 42
/reviewiq-pr feature/payment-retry

# Natural language
review this PR                    # acts like reviewiq-full
recheck                           # acts like reviewiq-recheck
```

**CLI** (needs `ANTHROPIC_API_KEY`):
```bash
reviewiq full https://github.com/owner/repo/pull/42
reviewiq pr 42
reviewiq recheck 42
```

---

## Commands

### `/reviewiq-full` — Full Review

Reviews all files at once with cross-file analysis, posts everything to PR automatically.

```
/reviewiq-full https://github.com/owner/repo/pull/42
```

**Flow**:
1. Fetch PR diff + all file contents
2. Load relevant skills across all files
3. 4-stage review: Understand → Analyze → Assess → Report
4. Post inline comments with `suggestion` blocks on PR
5. Post summary comment with finding table
6. Save state + markdown report

**Each finding includes**:
- **Suggestion**: exact replacement code (GitHub `Apply suggestion` button)
- **Resolution**: how to fix it
- **Comment**: additional context

---

### `/reviewiq-pr` — File-by-File Interactive

Reviews one file at a time. Shows findings, waits for user confirmation before posting.

```
/reviewiq-pr https://github.com/owner/repo/pull/42
```

**Flow per file**:
```
Reviewing file 1/4: src/webhooks/retry.py
Skills loaded: python, django (~2K words)

  Finding 1: [CRITICAL] Retry without backoff — line 42
  Suggestion: time.sleep(min(2 ** attempt * 0.5, 30) + random.uniform(0, 1))
  Resolution: Add exponential backoff with jitter
  Comment: At 500 queued webhooks, immediate retry = thundering herd

  [P] Post comments    [S] Skip    [F 1] Fix finding 1
```

- **If findings**: shows them, waits for `P` (post) / `S` (skip) / `F <N>` (fix)
- **If no findings**: auto-moves to next file (no prompt)

**Token efficient**: loads skills per-file (~2-3K words vs ~8K for all files).

---

### `/reviewiq-recheck` — Re-review with History

Loads previous review state, checks what's fixed, what's still open, flags new issues.

```
/reviewiq-recheck https://github.com/owner/repo/pull/42
```

**Flow**:
1. Load `state.json` from previous review
2. Fetch current code
3. For each pending finding:
   - Code fixed? → **auto-resolve**
   - Still broken? → **keep pending**
   - Changed differently? → **needs-review**
4. Check new changes for new issues
5. Post update to PR
6. Save updated state + new round report

**Output**:
```
Re-review Report — Round 2

| # | Severity | Previous | Current | Note |
|---|----------|----------|---------|------|
| 1 | CRITICAL | pending  | resolved | Backoff added ✓ |
| 2 | IMPORTANT| pending  | pending  | Still missing idempotency |
| 3 | NIT      | pending  | resolved | Error message fixed ✓ |
| 4 | IMPORTANT| —        | NEW      | Null check missing |

Previously: 3 findings → Now: 2 open, 2 resolved
Assessment: REQUEST CHANGES → NEEDS DISCUSSION
```

---

## Review Folder

Each review gets a persistent folder with iteration tracking and markdown reports:

```
.pr-review/
  pr-42/                          # one folder per PR/branch
    state.json                    # master state: all findings + statuses
    round-1/
      report.md                   # markdown report for round 1
    round-2/
      report.md                   # markdown report for round 2
    history.md                    # running log of all rounds
```

### state.json

```json
{
  "pr": { "number": 42, "repo": "owner/repo", "title": "...", "base": "main", "head": "feature/xyz" },
  "current_round": 2,
  "last_reviewed_sha": "abc123",
  "findings": {
    "1": {
      "id": 1, "severity": "CRITICAL", "status": "pending",
      "file": "retry.py", "line": 42,
      "title": "Retry without backoff",
      "suggestion": "time.sleep(min(2 ** attempt * 0.5, 30))",
      "resolution": "Add exponential backoff with jitter",
      "comment": "Thundering herd risk at scale",
      "created_round": 1, "resolved_round": null,
      "history": [
        { "round": 1, "status": "pending", "note": "Initial" },
        { "round": 2, "status": "resolved", "note": "Backoff added" }
      ]
    }
  },
  "summary": { "total": 3, "pending": 1, "resolved": 2 }
}
```

### Finding Statuses

| Status | Meaning |
|--------|---------|
| `pending` | Found, not yet fixed |
| `resolved` | Fix confirmed in code |
| `skipped` | User chose to skip |
| `needs-review` | Code changed but not per suggestion |

### Markdown Reports (round-N/report.md)

```markdown
# ReviewIQ Report — PR #42 Round 1
**Date**: 2026-04-15
**Skills**: commandments, security, python, django (~4.2K words)
**Files**: 4 | **Findings**: 3

## Finding 1: [CRITICAL] Retry without backoff — `retry.py:42`
**Status**: pending
**Suggestion**: `time.sleep(min(2 ** attempt * 0.5, 30))`
**Resolution**: Add exponential backoff
**Comment**: Thundering herd risk

## Summary
Pending: 3 | Resolved: 0
Assessment: REQUEST CHANGES
```

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
| `reviewiq-full` | All relevant across all files | ~5-8K words |
| `reviewiq-pr` | Per-file only | ~2-3K words/file |
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
│ Prior state  │  └ skill-guided       │ State JSON                │
│              │ State Manager         │ History log               │
│              │  └ folder per review  │                           │
├──────────────┼───────────────────────┼───────────────────────────┤
│              │ Claude Code: no key   │                           │
│              │ CLI: Claude API       │                           │
└──────────────┴───────────────────────┴───────────────────────────┘
```

---

## Configuration

| Variable | Required for | Description |
|----------|-------------|-------------|
| `ANTHROPIC_API_KEY` | CLI only | **Not needed for Claude Code.** |
| `GITHUB_TOKEN` | CLI + posting to PR | Auto-configured during install via `gh auth`. |

---

## File Structure

```
# Installed globally
~/.local/bin/reviewiq             CLI binary
~/.reviewiq/skills/               16 skill modules
~/.claude/REVIEWIQ.md             Claude Code global config

# Per-repo (created on first review)
.pr-review/
  pr-42/                          Review state + reports per PR
    state.json
    round-1/report.md
    history.md

# Source code
cmd/reviewiq/main.go              CLI (Go + cobra)
internal/
  state/ engine/ git/ skills/ github/ ci/
.claude/commands/
  reviewiq-full.md                /reviewiq-full
  reviewiq-pr.md                  /reviewiq-pr
  reviewiq-recheck.md             /reviewiq-recheck
.pr-review/skills/                16 skill files
install.sh                        One-line installer
uninstall.sh                      One-line uninstaller
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.
