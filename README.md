<div align="center">

![ReviewIQ Banner](https://img.shields.io/badge/🔍_ReviewIQ-AI_PR_Review_Agent-blue?style=for-the-badge)

# ReviewIQ

### Stateful AI-Powered PR Review Agent with Domain Expert Skills

[![MIT License](https://img.shields.io/badge/License-MIT-green.svg)](https://choosealicense.com/licenses/mit/)
[![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)](https://github.com/Sanmanchekar/reviewiq/releases)
[![Go](https://img.shields.io/badge/go-%3E%3D1.22-00ADD8.svg)](https://go.dev)
[![Claude Code](https://img.shields.io/badge/Claude%20Code-Compatible-purple.svg)](https://claude.ai/code)
[![GitHub Stars](https://img.shields.io/github/stars/Sanmanchekar/reviewiq?style=social)](https://github.com/Sanmanchekar/reviewiq/stargazers)

**One install, works everywhere. 16 review skills, natural language commands in Claude Code, CLI + CI support — powered by stateful finding tracking.**

[Quick Start](#quick-start) |
[Claude Code Commands](#claude-code-commands) |
[CLI Commands](#cli-commands) |
[Skills System](#skills-system) |
[Architecture](#architecture) |
[CI Integration](#ci-integration)

</div>

---

## Overview

ReviewIQ is a stateful PR review agent that carries domain expertise as loadable skill modules. Instead of reasoning from scratch, the agent reviews against pre-built expert checklists for your specific stack — security, performance, fintech, DevOps, and more. Findings are tracked objects with lifecycles, conversations carry full history, and incremental reviews diff against known state.

| Surface | How | LLM | API Key? | State |
|---------|-----|-----|----------|-------|
| **Claude Code** | Natural language: `review this PR` | Claude Code's own | No | Local JSON |
| **CLI** | `reviewiq review <branch>` | Claude API | Yes | Local JSON |
| **GitHub Actions** | Auto on PR open/push/comment | Claude API | Yes | Hidden PR comment |
| **Cursor / Codex / Aider** | Reference `.pr-review/agent.md` | Agent's own | No | Local JSON |

---

## Quick Start

### Install

```bash
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
```

Or with Go:

```bash
go install github.com/Sanmanchekar/reviewiq/cmd/reviewiq@latest
```

Or build from source:

```bash
git clone https://github.com/Sanmanchekar/reviewiq.git
cd reviewiq
go build -o /usr/local/bin/reviewiq ./cmd/reviewiq/
ln -s /usr/local/bin/reviewiq /usr/local/bin/riq
```

### Update / Uninstall

```bash
# Update (same as install — overwrites the binary)
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash

# Uninstall
curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/uninstall.sh | bash
```

### Usage

The installer sets up everything globally — binary, skills (`~/.reviewiq/skills/`), and Claude Code config (`~/.claude/REVIEWIQ.md`). Works in every repo immediately. No per-repo init needed.

**Claude Code (just talk naturally — no API key needed):**
```bash
# Review a PR by link (file-by-file, posts inline comments)
review-pr https://github.com/owner/repo/pull/42

# Or review current branch
review this PR                              # auto-detects: current branch → main
review this PR to develop                   # explicit target branch

# After review, continue naturally:
next                                        # move to next file
explain finding 2                           # deep dive
fix finding 1                               # applies the fix
post                                        # post findings as PR inline comments
check review                                # re-review after pushing fixes
approve                                     # final check
```

**CLI (native terminal — requires `ANTHROPIC_API_KEY` + `GITHUB_TOKEN`):**
```bash
export ANTHROPIC_API_KEY=sk-ant-...
export GITHUB_TOKEN=ghp_...

# Review PR by link — file by file
reviewiq review-pr https://github.com/owner/repo/pull/42
reviewiq review-pr 42                       # if inside the repo

# Post findings as inline PR comments with suggestion blocks
reviewiq review-pr 42 --post

# Branch-based review (no PR link needed)
reviewiq review feature/webhook-retries
reviewiq status
reviewiq check feature/webhook-retries
reviewiq approve
```

**GitHub Actions (automated — requires `ANTHROPIC_API_KEY` secret):**
```
Developer opens PR → Agent posts review automatically
Developer comments @review-agent why? → Agent replies with context
Developer pushes fix → Agent re-reviews incrementally
```

**Per-repo customization** (optional): Run `reviewiq init` to copy skills into your repo for team-specific rules. Repo-level skills at `.pr-review/skills/` override global defaults.

---

## Claude Code Commands

Works globally in Claude Code — just talk naturally. The installer sets up `~/.claude/REVIEWIQ.md` so Claude Code understands review commands in every repo.

### Natural Language Commands

| What you say | What happens |
|-------------|--------------|
| `review this PR` | Full 4-stage review (auto-detects current branch → main) |
| `review this PR to develop` | Review current branch against develop |
| `check review` / `re-review` | Incremental re-review after pushing fixes |
| `review status` / `show findings` | Finding status table |
| `explain finding 2` / `explain #2` | Deep dive with code tracing |
| `fix finding 1` / `fix #1` | Apply the suggested fix directly |
| `resolve 1 backoff added` | Mark finding as resolved |
| `retract 3 ORM handles it` | Retract finding (agent was wrong) |
| `wontfix 2 acceptable risk` | Mark as won't fix |
| `approve` / `final check` | Check for remaining blockers |
| `summarize PR` | Generate merge commit summary |
| `blast radius` / `impact analysis` | Trace what could break |
| `generate tests` / `test finding 2` | Generate test cases |

### Slash Commands (also available)

If you run `reviewiq init` in a repo, 13 `/review-*` slash commands are also created:

`/review-pr`, `/review-check`, `/review-explain`, `/review-fix`, `/review-status`, `/review-ask`, `/review-resolve`, `/review-retract`, `/review-wontfix`, `/review-approve`, `/review-summarize`, `/review-impact`, `/review-test`

### Typical Flow

```bash
# Checkout your feature branch
git checkout feature/payment-retry

# In Claude Code:
review this PR
# Agent: loads skills, reviews, outputs 4 findings

explain finding 2
# Agent: traces code, shows concrete scenarios

fix finding 1
# Agent: applies fix, verifies, marks resolved

# Push fixes, then:
check review
# Agent: Finding 1 → resolved, Finding 2 → partial, 1 new nit

approve
# "APPROVE — no remaining blockers. Safe to merge."

summarize PR
# Generates merge commit message
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
│              │  └ LLM (see below)    │ PR comments               │
│              │ State Manager         │                           │
│              │  ├ Local JSON files   │                           │
│              │  └ GitHub PR comments │                           │
├──────────────┼───────────────────────┼───────────────────────────┤
│              │   Two LLM Modes       │                           │
│              │ Claude Code: agent's  │                           │
│              │   own LLM (no key)    │                           │
│              │ CLI/CI: Claude API    │                           │
│              │   (needs API key)     │                           │
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
- **Dual backend**: Local files (CLI/Claude Code) + hidden PR comment (CI)

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

---

## CLI Commands

The Go binary for terminal and CI use. Requires `ANTHROPIC_API_KEY`.

| Command | Purpose | State Change |
|---------|---------|--------------|
| `reviewiq init` | Initialize `.pr-review/` + `.claude/commands/` | Creates all files |
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

Commands map 1:1 between Claude Code and CLI:

| Action | Claude Code | CLI |
|--------|-------------|-----|
| Full review | `/review-pr <branch>` | `reviewiq review <branch>` |
| Re-review | `/review-check <branch>` | `reviewiq check <branch>` |
| Status | `/review-status` | `reviewiq status` |
| Explain | `/review-explain <N>` | `reviewiq explain <N>` |
| Fix | `/review-fix <N>` | *(not in CLI — agent applies directly)* |
| Ask | `/review-ask <question>` | `reviewiq ask <question>` |
| Resolve | `/review-resolve <N> [note]` | `reviewiq resolve <N> --note "..."` |
| Retract | `/review-retract <N> [reason]` | `reviewiq retract <N> --note "..."` |
| Won't fix | `/review-wontfix <N> [reason]` | `reviewiq wontfix <N> --note "..."` |
| Approve | `/review-approve` | `reviewiq approve` |
| Summarize | `/review-summarize` | *(not in CLI)* |
| Impact | `/review-impact` | *(not in CLI)* |
| Test | `/review-test [N]` | *(not in CLI)* |
| CI mode | *(not applicable)* | `reviewiq ci` |

### Chaining Commands

```bash
# Claude Code workflow
/review-pr feature/payment-retry       # Initial review
/review-explain 2                      # Deep dive
/review-fix 1                          # Apply fix
/review-check feature/payment-retry    # Re-review
/review-approve                        # Final check
/review-summarize                      # Merge commit message

# CLI workflow
reviewiq review feature/payment-retry
reviewiq explain 2
reviewiq retract 3 --note "ORM handles it"
reviewiq check feature/payment-retry
reviewiq approve
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

| Variable | Required for | Default | Description |
|----------|-------------|---------|-------------|
| `ANTHROPIC_API_KEY` | CLI + CI | — | Claude API key. **Not needed for Claude Code slash commands.** |
| `MODEL` | CLI + CI | `claude-sonnet-4-6-20250514` | Claude model |
| `MAX_TOKENS` | CLI + CI | `8192` | Max response tokens |
| `GITHUB_TOKEN` | CI only | — | GitHub token (auto-provided in Actions) |

---

## File Structure

### Installed Globally (by install.sh)

```
~/.local/bin/
  reviewiq                      CLI binary
  riq                           Shorthand symlink
~/.reviewiq/
  agent.md                      Review protocol
  skills/                       16 skill modules (global defaults)
~/.claude/
  REVIEWIQ.md                   Claude Code global config (natural language commands)
```

### Repository (source code)

```
cmd/reviewiq/
  main.go                       CLI entry point (cobra commands)
internal/
  state/state.go                State manager (types, lifecycle, dual backend)
  engine/engine.go              Review engine (Claude API, structured output)
  git/git.go                    Git operations
  skills/skills.go              Skill auto-detection and loading
  ci/ci.go                      CI mode (GitHub Actions webhook handler)
.pr-review/
  agent.md                      Review protocol (customize per repo)
  skills/                       16 skill modules (customize per repo)
    commandments.md             40 universal review laws
    security.md                 OWASP-aligned security checks
    scalability.md              Performance and scaling patterns
    stability.md                Reliability and observability
    maintainability.md          Code quality and complexity
    performance.md              Profiling and optimization
    languages.md                12 language anti-pattern libraries
    frameworks.md               14 framework rule sets
    devops.md                   Docker/K8s/Helm/Terraform/CI-CD
    fintech.md                  Payments/lending/insurance/compliance
    india-regulatory.md         RBI/NBFC/UPI/eKYC/NACH/GST
    credit-bureau.md            CIBIL/Experian/Equifax integration
    fraud.md                    Fraud detection and prevention
    notifications.md            SMS/email/push/WhatsApp compliance
    financial-microservices.md  Saga/outbox/distributed transactions
    data-privacy.md             DPDP/GDPR/CCPA compliance
.claude/commands/               13 Claude Code slash commands
  review-pr.md                  /review-pr <branch>
  review-check.md               /review-check <branch>
  review-explain.md             /review-explain <N>
  review-fix.md                 /review-fix <N>
  review-status.md              /review-status
  review-ask.md                 /review-ask <question>
  review-resolve.md             /review-resolve <N> [note]
  review-retract.md             /review-retract <N> [reason]
  review-wontfix.md             /review-wontfix <N> [reason]
  review-approve.md             /review-approve
  review-summarize.md           /review-summarize
  review-impact.md              /review-impact
  review-test.md                /review-test [N]
.github/workflows/
  pr-review.yml                 GitHub Actions workflow
go.mod                          Go module definition
go.sum                          Dependency checksums
install.sh                      One-line installer
uninstall.sh                    One-line uninstaller
```

---

## Development

```bash
git clone https://github.com/Sanmanchekar/reviewiq.git
cd reviewiq
go build -o reviewiq ./cmd/reviewiq/
./reviewiq --version
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.
