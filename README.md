<div align="center">

![ReviewIQ Banner](https://img.shields.io/badge/🔍_ReviewIQ-AI_PR_Review_Agent-blue?style=for-the-badge)

# ReviewIQ

### Stateful AI-Powered PR Review Agent with Domain Expert Skills

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://choosealicense.com/licenses/mit/)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/Sanmanchekar/reviewiq/releases)
[![Python](https://img.shields.io/badge/python-%3E%3D3.10-brightgreen.svg)](https://python.org)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-Compatible-purple.svg)](https://claude.ai/code)
[![GitHub Stars](https://img.shields.io/github/stars/Sanmanchekar/reviewiq?style=social)](https://github.com/Sanmanchekar/reviewiq/stargazers)

**16 review skills across 4 surfaces — CLI, CI, AI agents, and GitHub comments — powered by stateful finding tracking.**

[Quick Start](#-quick-start) |
[Review Workflow](#-review-workflow) |
[CLI Commands](#-cli-commands) |
[Skills System](#-skills-system) |
[Architecture](#-architecture) |
[CI Integration](#-ci-integration)

</div>

---

## Overview

ReviewIQ is a stateful PR review agent that carries domain expertise as loadable skill modules. Instead of reasoning from scratch, the agent reviews against pre-built expert checklists for your specific stack — security, performance, fintech, DevOps, and more. Findings are tracked objects with lifecycles, conversations carry full history, and incremental reviews diff against known state.

| Surface | How | State |
|---------|-----|-------|
| **CLI** | `reviewiq review feature/branch` | Local JSON files |
| **Claude Code** | `/review-pr feature/branch` | Local JSON files |
| **Cursor / Codex / Aider** | Reference `.pr-review/agent.md` | Local JSON files |
| **GitHub Actions** | Auto on PR open/push/comment | Hidden PR comment |

---

## Quick Start

### Install

```bash
pip install git+https://github.com/Sanmanchekar/reviewiq.git
```

Or clone and install locally:

```bash
git clone https://github.com/Sanmanchekar/reviewiq.git
cd reviewiq
pip install .
```

Two entry points: `reviewiq` and `riq` (shorthand).

### Usage

**CLI (native terminal):**
```bash
reviewiq init                                    # Initialize skills in your repo
reviewiq review feature/webhook-retries          # Full review
reviewiq status                                  # Finding status table
reviewiq check feature/webhook-retries           # Re-review after fixes
reviewiq explain 2                               # Deep dive into finding #2
reviewiq ask "why is this critical?"             # Follow-up question
reviewiq resolve 1 --note "backoff added"        # Transition finding
reviewiq approve                                 # Final check
```

**Claude Code (slash commands):**
```bash
/review-pr feature/webhook-retries
# Then conversationally:
> "explain finding 2"
> "fix finding 1"
> "check"
```

**GitHub Actions (automated):**
```
Developer opens PR → Agent posts review automatically
Developer comments @review-agent why? → Agent replies with context
Developer pushes fix → Agent re-reviews incrementally
```

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        REVIEWIQ PLATFORM                         │
├──────────────┬───────────────────────┬───────────────────────────┤
│ Input Layer  │   Processing Core     │   Output Layer            │
├──────────────┼───────────────────────┼───────────────────────────┤
│ PR branch    │ Skill Detection       │ Structured findings       │
│ Git diff     │  ├ File extensions    │ Severity classification   │
│ Changed files│  ├ Import scanning    │ Concrete code fixes       │
│ File contents│  └ Domain matching    │ Finding lifecycle         │
│ PR metadata  │ Review Engine         │ Incremental re-reviews    │
│ Prior state  │  ├ 4-stage pipeline   │ Conversation history      │
│ Conversation │  ├ Skill-guided       │ State JSON                │
│              │  └ Claude API         │ PR comments               │
│              │ State Manager         │                           │
│              │  ├ Local JSON files   │                           │
│              │  └ GitHub PR comments │                           │
├──────────────┼───────────────────────┼───────────────────────────┤
│              │   Review Dimensions   │                           │
│              │ Correctness ···· ✓    │                           │
│              │ Security ······ ✓     │                           │
│              │ Scalability ··· ✓     │                           │
│              │ Stability ····· ✓     │                           │
│              │ Maintainability  ✓    │                           │
│              │ Performance ··· ✓     │                           │
└──────────────┴───────────────────────┴───────────────────────────┘
```

### Processing Pipeline

```
PR Opened → Skill Detection → Context Assembly → 4-Stage Review → State Persist
                │                    │                  │              │
          File extensions      Git diff +          Understand     Findings as
          Import scanning      Full file read      Analyze        tracked JSON
          Domain matching      Symbol tracing      Assess         objects with
                                                   Report         lifecycle
```

### State Persistence

All interactions write state to `.pr-review/reviews/`:
- **JSON state files**: Findings, conversation, review rounds, assessment
- **Finding lifecycle**: `open → resolved | partially_fixed | wontfix | retracted`
- **Cross-session**: Resume any review from where you left off
- **Dual backend**: Local files (CLI) + hidden PR comment (CI)

---

## Review Workflow

The core 4-stage review pipeline with stateful finding tracking.

### Review Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                     4-STAGE REVIEW PIPELINE                      │
├─────────────┬──────────────┬──────────────┬─────────────────────┤
│ 1. UNDERSTAND│ 2. ANALYZE   │ 3. ASSESS    │ 4. REPORT          │
├─────────────┼──────────────┼──────────────┼─────────────────────┤
│ Read files   │ Correctness  │ CRITICAL     │ Structured findings │
│ Map intent   │ Edge cases   │ IMPORTANT    │ Concrete fixes      │
│ Trace system │ Security     │ NIT          │ Impact analysis     │
│ Read history │ Performance  │ QUESTION     │ Summary + assess    │
│              │ Concurrency  │              │                     │
│              │ Cross-file   │              │                     │
└─────────────┴──────────────┴──────────────┴─────────────────────┘
```

### Finding Lifecycle

Every finding is a tracked object with a full audit trail:

```
open ──→ resolved          developer fixed it
     ──→ partially_fixed   partially addressed, needs more work
     ──→ wontfix           developer won't fix, agent accepts reasoning
     ──→ retracted         agent was wrong, or not actually an issue

partially_fixed ──→ resolved | wontfix
```

Every transition recorded with: timestamp, round number, note.

### Incremental Re-review

After the developer pushes fixes, the agent knows exactly what to check:

```
📋 RE-REVIEW (Round 2)
Changes since: abc1234...def5678

FINDING STATUS UPDATES:
┌────┬──────────┬──────────────────┬────────────────────────────────┐
│ #  │ Severity │ Status Change    │ Note                           │
├────┼──────────┼──────────────────┼────────────────────────────────┤
│ 1  │ CRITICAL │ open → resolved  │ Backoff with jitter added      │
│ 2  │ IMPORTANT│ open → partial   │ Retry limit added, no breaker  │
│ 4  │ NIT      │ NEW              │ Typo in error message line 92  │
└────┴──────────┴──────────────────┴────────────────────────────────┘

Summary: 1 resolved, 1 partial, 1 new nit
Assessment: REQUEST CHANGES → NEEDS DISCUSSION
```

### Conversation Continuity

The agent sends full conversation history as multi-turn messages — genuine memory, not reconstructed:

```
Developer: "finding 2 — won't the lock cause deadlock with line 40?"

Agent: *already has full context from prior turns*
       "You're right — lock ordering issue. Updating suggested
        fix to use a buffered channel instead."
       *updates finding 2's suggested_fix in state*

Developer: "ok, retract finding 3, ORM handles that"

Agent: *transitions finding 3 to retracted*
       "Retracted — ORM parameterizes by default."
```

---

## CLI Commands

| Command | Purpose | State Change |
|---------|---------|--------------|
| `reviewiq init` | Initialize `.pr-review/` with skills | Creates skill files |
| `reviewiq review <branch>` | Full 4-stage review | Creates findings (open) |
| `reviewiq check <branch>` | Incremental re-review | Updates finding statuses |
| `reviewiq status` | Show finding status table | Read-only |
| `reviewiq explain <N>` | Deep dive into finding N | Adds to discussion thread |
| `reviewiq ask <question>` | Follow-up question | Adds to conversation |
| `reviewiq resolve <N>` | Mark finding as resolved | Finding → resolved |
| `reviewiq retract <N>` | Agent was wrong | Finding → retracted |
| `reviewiq wontfix <N>` | Developer won't fix | Finding → wontfix |
| `reviewiq approve` | Final blocker check | Assessment → APPROVE |
| `reviewiq ci` | Run in CI mode | Both backends |

### Sample CLI Output

```
$ reviewiq review feature/payment-retry

Reviewing feature/payment-retry against main...
[reviewiq] Skills loaded: commandments, security, scalability, stability,
           maintainability, performance, python, django, fintech
[reviewiq] Calling Claude with 2 message(s), round 1

### Finding 1: Retry without backoff
**Severity**: [CRITICAL]
**File**: `src/webhooks/retry.py:42`
**Status**: open

**Problem**: Webhook retries fire immediately on failure with no delay.
During incident recovery, queued webhooks retry simultaneously.

**Impact**: Thundering herd against downstream services. At 500 queued
webhooks, this is a self-inflicted DDoS.

**Suggested fix**:
  time.sleep(min(2 ** attempt * 0.5, 30) + random.uniform(0, 1))

**Why this fix**: Exponential backoff with jitter and 30s cap.

## Summary
- 3 files changed, 4 findings (1 critical, 2 important, 1 nit)
- Overall assessment: REQUEST CHANGES
- Key risk: thundering herd on webhook retry

State saved: .pr-review/reviews/pr-42917.json
Findings: 4 (4 open)
```

```
$ reviewiq status

ReviewIQ Status — PR #42917 (Round 1)
Assessment: REQUEST CHANGES

#    Severity     Status           Title                                    File
----------------------------------------------------------------------------------------------------
1    CRITICAL     OPEN             Retry without backoff                    src/webhooks/retry.py:42
2    IMPORTANT    OPEN             Missing idempotency key                  src/webhooks/handler.py:18
3    IMPORTANT    OPEN             Webhook signature not verified           src/webhooks/verify.py:7
4    NIT          OPEN             Inconsistent error message format        src/webhooks/retry.py:56

Total: 4 | Open: 4 | Resolved: 0 | Won't fix: 0 | Retracted: 0
```

---

## Skills System

ReviewIQ auto-detects languages, frameworks, and domains from the PR's changed files and loads only the relevant expert checklists. This means focused reviews with minimal token waste.

### How It Works

```
PR changes: src/payment/service.py, src/loan/emi.py, Dockerfile
                          │
                  Skill Detection
                  (file extensions + imports)
                          │
    ┌─────────────────────┼──────────────────────┐
    ▼                     ▼                      ▼
 Always (6):           Detected:              Domains:
 commandments          python (lang)          fintech
 security              django (framework)     india-regulatory
 scalability           docker (devops)        fraud
 stability
 maintainability
 performance
                          │
                          ▼
              Only relevant skills loaded
              (~5K tokens vs ~14K for all)
```

### Always Loaded (every review)

| Skill | Rules | What it covers |
|-------|-------|----------------|
| **Commandments** | 40 | Correctness, security, reliability, performance, maintainability, data, APIs, testing |
| **Security** | 50+ | Injection, auth, crypto, data protection, infra security, dependencies (OWASP-aligned) |
| **Scalability** | 40+ | Database, caching, concurrency, network, compute, architecture patterns |
| **Stability** | 35+ | Error handling, resilience (circuit breakers, bulkheads), observability, deployment safety |
| **Maintainability** | 40+ | Complexity limits, naming, code organization, dependencies, testability, refactoring signals |
| **Performance** | 45+ | Algorithms, memory, database, network/IO, CPU, frontend, caching, concurrency |

### Auto-Detected: Languages & Frameworks

| Skill | Triggers on | Key checks |
|-------|-------------|------------|
| **Languages** | `.py`, `.java`, `.go`, `.ts`, `.cpp`, `.rs`, `.cs`, `.rb`, `.php`, `.sh`, `.cob` | Anti-patterns, performance traps, concurrency pitfalls, type safety per language |
| **Frameworks** | Django, FastAPI, Flask, Spring, React, Next.js, Express, NestJS, Vue, Angular, Rails, .NET | N+1 queries, missing auth, CSRF, XSS, framework-specific misuse |
| **DevOps** | `Dockerfile`, `Chart.yaml`, `*.tf`, K8s manifests, CI configs | Container security, resource limits, Helm values, Terraform state, CI secrets |

### Auto-Detected: Financial Services

| Skill | Triggers on | Key checks |
|-------|-------------|------------|
| **Fintech** | stripe, razorpay, payment, loan, emi, insurance, ledger, kyc | Floating-point money, idempotency, PCI-DSS, EMI/APR calculation, double-entry bookkeeping |
| **India Regulatory** | upi, nach, aadhaar, rbi, nbfc, ifsc, cersai | RBI digital lending, NBFC compliance, UPI/NEFT/RTGS, eKYC, Account Aggregator, GST |
| **Credit Bureau** | cibil, experian, equifax, credit_score, bureau | Hard vs soft inquiry, data retention, score processing, bureau reporting, disputes |
| **Fraud** | fraud, risk_engine, velocity, device_fingerprint | Velocity checks, ATO prevention, payment/lending fraud, ML model review, rule engines |
| **Notifications** | sms, twilio, sendgrid, fcm, whatsapp, dlt | TRAI DND/DLT, RBI SMS mandates, email compliance, push, WhatsApp Business API |
| **Financial Microservices** | saga, outbox, event_sourcing, kafka | Saga pattern, compensating transactions, transactional outbox, distributed consistency |
| **Data Privacy** | gdpr, ccpa, dpdp, consent, pii, anonymiz | DPDP Act, GDPR, CCPA, PII detection, encryption, consent management, data deletion |

### Sample: Skill Detection in Action

```
🔍 SKILL DETECTION: PR #142

┌───────────────────┬──────────────┬──────────────────────────────┐
│ Category          │ Detected     │ Trigger                      │
├───────────────────┼──────────────┼──────────────────────────────┤
│ Always            │ 6 skills     │ (loaded on every review)     │
│ Language          │ python       │ .py file extensions          │
│ Framework         │ django       │ `from django.db import`      │
│ DevOps            │ docker       │ Dockerfile present           │
│ Domain: fintech   │ ✓            │ `from razorpay import`       │
│ Domain: fraud     │ ✓            │ risk_engine in filepath      │
│ Domain: privacy   │ ✓            │ `consent` in imports         │
└───────────────────┴──────────────┴──────────────────────────────┘

Skills prompt: 8,200 words loaded (vs 14,000 for all skills)
Token savings: 41%
```

### Customization

Skills live in `.pr-review/skills/`. Edit them to add your team's domain rules:

```bash
# Add a custom rule
echo "- **Custom Auth**: All endpoints must use our AuthMiddleware" >> .pr-review/skills/security.md

# Add an entirely new skill
cat > .pr-review/skills/mobile.md << 'EOF'
# Mobile Review Rules
- Missing offline support → IMPORTANT: app must work offline
- Missing deep link handling → NIT: cold start from deep link
EOF
```

---

## CI Integration

### GitHub Actions (Automated)

1. Add `ANTHROPIC_API_KEY` as a repository secret

2. The workflow at `.github/workflows/pr-review.yml` handles:

| Event | What happens | State |
|-------|-------------|-------|
| PR opened | Full 4-stage review posted as comment | Saved to hidden PR comment |
| Push to PR | Incremental re-review (resolved/partial/new) | Updated in PR comment |
| `@review-agent <question>` | Contextual reply with full history | Updated in PR comment |

3. State survives across workflow runs — the agent remembers all prior findings and conversation.

### Interaction via Comments

```markdown
# Developer asks about a finding:
@review-agent why is finding 1 critical? our webhook volume is only 100/day

# Agent replies with context:
At 100/day normal volume, you're right. The risk is during incident recovery —
if 500 webhooks queue up during a 2hr outage and all retry simultaneously,
you get a thundering herd. At your volume, a simple 2s fixed delay suffices.

# Developer asks for re-review:
@review-agent check
```

---

## Command Cross-Reference

Commands chain naturally across review lifecycle:

```bash
# Full review lifecycle
reviewiq review feature/payment-retry       # Initial review
reviewiq explain 2                          # Deep dive
reviewiq ask "what about using channels?"   # Alternative discussion
reviewiq retract 3 --note "ORM handles it"  # Retract finding
# developer pushes fixes
reviewiq check feature/payment-retry        # Incremental re-review
reviewiq approve                            # Final check

# Quick status check
reviewiq status                             # Table of all findings

# CI lifecycle (automated)
# PR opened → reviewiq ci (event: opened)
# Push → reviewiq ci (event: synchronize)
# Comment → reviewiq ci (event: comment)
```

---

## Token Optimization

The skills system minimizes token usage through selective loading:

| PR Type | Skills Loaded | Prompt Size | Savings vs All |
|---------|--------------|-------------|----------------|
| React component | 6 always + typescript + react | ~5,600 words | 60% |
| Python API | 6 always + python + django | ~5,800 words | 58% |
| Fintech full stack | 6 always + python + django + 5 domains | ~13,800 words | 0% (full) |
| Dockerfile only | 6 always + docker | ~5,200 words | 63% |

Skills use compressed checklist format — anti-pattern → severity → fix. No prose filler.

---

## Finding Severity Levels

| Severity | Meaning | Merge? |
|----------|---------|--------|
| `[CRITICAL]` | Bugs, data loss, security vulnerabilities, outages | Must fix |
| `[IMPORTANT]` | Poor error handling, race conditions, performance issues | Should fix |
| `[NIT]` | Style, naming, minor improvements | Won't block |
| `[QUESTION]` | Looks odd, might be intentional. Needs clarification | Discuss |

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_API_KEY` | — | Claude API key (required) |
| `MODEL` | `claude-sonnet-4-6-20250514` | Claude model |
| `MAX_TOKENS` | `8192` | Max response tokens |
| `GITHUB_TOKEN` | — | GitHub token (CI only, auto-provided in Actions) |

---

## File Structure

```
src/reviewiq/
  __init__.py               Package init + version
  __main__.py               python -m reviewiq support
  cli.py                    CLI entry point (reviewiq / riq)
  engine.py                 Review engine (Claude API, state updates)
  state.py                  State manager (dual backend: local + GitHub)
  git.py                    Git operations
  ci.py                     CI mode (GitHub Actions webhook handler)
  skills.py                 Skill auto-detection and loading
  templates/
    agent.md                Default agent protocol
    skills/                 16 skill modules (copied on init)
      commandments.md       40 universal review laws
      security.md           OWASP-aligned security checks
      scalability.md        Performance and scaling patterns
      stability.md          Reliability and observability
      maintainability.md    Code quality and complexity
      performance.md        Profiling and optimization
      languages.md          12 language anti-pattern libraries
      frameworks.md         14 framework rule sets
      devops.md             Docker/K8s/Helm/Terraform/CI-CD
      fintech.md            Payments/lending/insurance/compliance
      india-regulatory.md   RBI/NBFC/UPI/eKYC/NACH/GST
      credit-bureau.md      CIBIL/Experian/Equifax integration
      fraud.md              Fraud detection and prevention
      notifications.md      SMS/email/push/WhatsApp compliance
      financial-microservices.md  Saga/outbox/distributed transactions
      data-privacy.md       DPDP/GDPR/CCPA compliance
.pr-review/
  agent.md                  Review protocol (customize per repo)
  skills/                   Skill modules (customize per repo)
.claude/commands/
  review-pr.md              Claude Code slash command
.github/workflows/
  pr-review.yml             GitHub Actions workflow
```

---

## Development

```bash
git clone https://github.com/Sanmanchekar/reviewiq.git
cd reviewiq
pip install -e .
reviewiq --version
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.
