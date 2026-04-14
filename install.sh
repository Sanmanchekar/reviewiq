#!/usr/bin/env bash
#
# ReviewIQ Installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
#
# What it does:
#   1. Builds and installs the Go binary to ~/.local/bin
#   2. Copies review skills to ~/.reviewiq/skills/ (global)
#   3. Installs Claude Code global config (~/.claude/REVIEWIQ.md)
#   4. Sets up PATH
#   5. Done — works in every repo, no per-repo init needed
#
# Requires: git, go (>= 1.22)
#

set -euo pipefail

REPO_URL="https://github.com/Sanmanchekar/reviewiq.git"
INSTALL_DIR="$HOME/.local/bin"
SKILLS_DIR="$HOME/.reviewiq/skills"
CLAUDE_DIR="$HOME/.claude"
BINARY="reviewiq"
SHELL_RC=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${GREEN}[reviewiq]${NC} $*"; }
warn()  { echo -e "${YELLOW}[reviewiq]${NC} $*"; }
error() { echo -e "${RED}[reviewiq]${NC} $*" >&2; exit 1; }
step()  { echo -e "${CYAN}[reviewiq]${NC} $*"; }

# ── Checks ───────────────────────────────────────────────────────────────────

check_go() {
    if ! command -v go &>/dev/null; then
        error "Go is not installed. Install it from https://go.dev/dl/ (requires >= 1.22)"
    fi
    local ver
    ver="$(go version | grep -oE '[0-9]+\.[0-9]+' | head -1)"
    info "Found Go $ver"
}

check_git() {
    if ! command -v git &>/dev/null; then
        error "git is not installed."
    fi
}

detect_shell_rc() {
    local shell_name
    shell_name="$(basename "${SHELL:-/bin/bash}")"
    case "$shell_name" in
        zsh)  SHELL_RC="$HOME/.zshrc" ;;
        bash)
            if [[ -f "$HOME/.bash_profile" ]]; then
                SHELL_RC="$HOME/.bash_profile"
            else
                SHELL_RC="$HOME/.bashrc"
            fi
            ;;
        fish) SHELL_RC="$HOME/.config/fish/config.fish" ;;
        *)    SHELL_RC="$HOME/.profile" ;;
    esac
}

# ── Cleanup old installs ────────────────────────────────────────────────────

cleanup_old() {
    local old_path
    old_path="$(command -v reviewiq 2>/dev/null || true)"
    if [[ -n "$old_path" ]]; then
        if head -1 "$old_path" 2>/dev/null | grep -q python; then
            warn "Found old Python version at $old_path — removing..."
            pip uninstall reviewiq -y 2>/dev/null || true
            rm -f "$old_path" 2>/dev/null || true
            info "Old Python version removed"
        fi
    fi
}

# ── Step 1: Build and install binary ─────────────────────────────────────────

install_binary() {
    mkdir -p "$INSTALL_DIR"

    local tmp
    tmp="$(mktemp -d)"
    trap "rm -rf $tmp" EXIT

    step "Cloning repository..."
    git clone --depth 1 "$REPO_URL" "$tmp/reviewiq" 2>/dev/null

    step "Building binary..."
    cd "$tmp/reviewiq"
    go build -o "$INSTALL_DIR/$BINARY" ./cmd/reviewiq/
    chmod +x "$INSTALL_DIR/$BINARY"
    ln -sf "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/riq"

    # Copy skills to global location
    step "Installing skills to $SKILLS_DIR..."
    mkdir -p "$SKILLS_DIR"
    cp "$tmp/reviewiq/.pr-review/skills/"*.md "$SKILLS_DIR/" 2>/dev/null || true

    # Copy agent.md and REVIEWIQ.md to global location
    mkdir -p "$HOME/.reviewiq"
    cp "$tmp/reviewiq/.pr-review/agent.md" "$HOME/.reviewiq/agent.md" 2>/dev/null || true

    # Save REVIEWIQ.md for Claude Code config step
    cp "$tmp/reviewiq/REVIEWIQ.md" "$tmp/REVIEWIQ.md" 2>/dev/null || true

    cd - >/dev/null
    info "Binary installed to $INSTALL_DIR/$BINARY"
    info "Skills installed to $SKILLS_DIR/ ($(ls "$SKILLS_DIR/"*.md 2>/dev/null | wc -l | tr -d ' ') files)"
}

# ── Step 2: Set up PATH ─────────────────────────────────────────────────────

setup_path() {
    if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        return 0
    fi

    detect_shell_rc

    if [[ -n "$SHELL_RC" ]]; then
        if ! grep -q "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
            echo "" >> "$SHELL_RC"
            echo "# ReviewIQ" >> "$SHELL_RC"
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_RC"
            info "Added $INSTALL_DIR to PATH in $SHELL_RC"
        fi
    fi

    export PATH="$INSTALL_DIR:$PATH"
}

# ── Step 3: Install Claude Code global config ────────────────────────────────

install_claude_config() {
    step "Setting up Claude Code global config..."

    mkdir -p "$CLAUDE_DIR"

    # Copy REVIEWIQ.md from cloned repo (or write default)
    local tmp_reviewiq="/tmp/REVIEWIQ.md"
    if [[ -f "$tmp_reviewiq" ]]; then
        cp "$tmp_reviewiq" "$CLAUDE_DIR/REVIEWIQ.md"
        info "Installed ~/.claude/REVIEWIQ.md (from repo)"
    else
    cat > "$CLAUDE_DIR/REVIEWIQ.md" << 'REVIEWIQ_EOF'
# ReviewIQ — Global PR Review Agent

When the user asks to "review this PR", "review PR", "review code", "check review", or any review-related request, activate ReviewIQ.

## Branch Detection

1. **FROM branch** (head): Current checked-out branch via `git rev-parse --abbrev-ref HEAD`
2. **TO branch** (base/target):
   - If user specifies: use that (e.g., "review this PR to develop")
   - If not specified: auto-detect via `git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@'`
   - Fallback: `main`, then `master`

3. If FROM branch equals TO branch, tell the user:
   "You're on the base branch. Checkout your feature branch first:
    git checkout feature/your-branch"

## Commands (respond to natural language)

| User says | Action |
|-----------|--------|
| "review this PR" / "review PR" / "review to main" | Full 4-stage review |
| "review check" / "check review" / "re-review" | Incremental re-review after fixes |
| "explain finding N" / "explain #N" | Deep dive into finding N |
| "fix finding N" / "fix #N" | Apply the suggested fix |
| "review status" / "show findings" | Finding status table |
| "retract N" / "retract finding N" | Retract (agent was wrong) |
| "wontfix N" / "won't fix N" | Mark as won't fix |
| "resolve N" / "mark N resolved" | Mark as resolved |
| "approve" / "final check" | Check for remaining blockers |
| "summarize PR" / "PR summary" | Generate merge commit summary |
| "blast radius" / "impact analysis" | Trace what could break |
| "generate tests" / "test finding N" | Generate test cases |

## Review Protocol

### Step 1: Context Assembly

```bash
HEAD_BRANCH=$(git rev-parse --abbrev-ref HEAD)
BASE_BRANCH=<user-specified or auto-detected>

echo "Reviewing: $HEAD_BRANCH → $BASE_BRANCH"
git log --oneline $BASE_BRANCH..$HEAD_BRANCH
git diff $BASE_BRANCH...$HEAD_BRANCH
git diff --name-only $BASE_BRANCH...$HEAD_BRANCH
```

Read ALL changed files in full. For key symbols, trace with `git grep -n <symbol>`.

### Step 2: Load Skills

Check for skills in this order (first found wins per skill):
1. `.pr-review/skills/` (repo-level — team customizations)
2. `~/.reviewiq/skills/` (global — installed defaults)

**Always load**: `commandments.md`, `security.md`, `scalability.md`, `stability.md`, `maintainability.md`, `performance.md`

**Load by file type** (only matching sections):
- `.py` → Python from `languages.md`, check for django/fastapi/flask in `frameworks.md`
- `.ts/.js` → TypeScript from `languages.md`, check for react/nextjs/express/nestjs/vue/angular in `frameworks.md`
- `.go` → Golang from `languages.md`
- `.java` → Java from `languages.md`, check for spring in `frameworks.md`
- `.rs` → Rust, `.cs` → C#, `.rb` → Ruby, `.cpp/.c` → C++, `.php` → PHP, `.sh` → Shell
- `Dockerfile` / `Chart.yaml` / `*.tf` / CI configs → matching section from `devops.md`

**Load by domain** (if imports/filenames match):
- payment/stripe/razorpay/loan/emi/insurance/ledger/kyc → `fintech.md`
- upi/nach/aadhaar/rbi/nbfc/ifsc → `india-regulatory.md`
- cibil/experian/credit_score/bureau → `credit-bureau.md`
- fraud/risk_engine/velocity/device_fingerprint → `fraud.md`
- sms/twilio/sendgrid/whatsapp/fcm/dlt → `notifications.md`
- saga/outbox/event_sourcing/kafka → `financial-microservices.md`
- gdpr/ccpa/dpdp/consent/pii/anonymiz → `data-privacy.md`

If no skills directory exists, review using built-in knowledge.

### Step 3: 4-Stage Review

**Stage 1 — Understand**: Read files, map intent, trace system context
**Stage 2 — Analyze**: Check against skill checklists for anti-patterns
**Stage 3 — Assess**: Classify each finding:
  - `[CRITICAL]` — bugs, data loss, security vulnerabilities. Must fix.
  - `[IMPORTANT]` — poor error handling, race conditions, perf issues. Should fix.
  - `[NIT]` — style, naming, minor improvements. Won't block.
  - `[QUESTION]` — looks odd, might be intentional. Needs clarification.

**Stage 4 — Report**: For each finding:
```
### Finding <N>: <title>
**Severity**: [CRITICAL/IMPORTANT/NIT/QUESTION]
**File**: `path/to/file:line`
**Status**: open

**Problem**: What's wrong and why it matters.
**Impact**: What breaks.
**Suggested fix**:
<concrete code fix>
**Why this fix**: Rationale.
```

End with summary: files changed, finding counts, assessment (APPROVE / REQUEST CHANGES / NEEDS DISCUSSION).

### Step 4: Save State

Create `.pr-review/reviews/` in the repo if it doesn't exist.
Write findings to `.pr-review/reviews/pr-<N>.json`:

```json
{
  "version": 2,
  "pr": { "number": 0, "repo": "", "title": "", "author": "", "base_branch": "", "head_branch": "" },
  "review_rounds": [{ "round": 1, "timestamp": "ISO8601", "head_sha": "", "base_sha": "", "event": "review", "files_reviewed": [] }],
  "findings": {
    "1": {
      "id": 1, "title": "", "severity": "", "status": "open",
      "file": "", "line": 0, "problem": "", "impact": "",
      "suggested_fix": "", "fix_rationale": "",
      "created_round": 1, "created_at": "", "updated_at": "",
      "status_history": [{ "status": "open", "round": 1, "timestamp": "" }],
      "discussion": []
    }
  },
  "conversation": [],
  "summary": { "total_findings": 0, "open": 0, "resolved": 0, "wontfix": 0, "retracted": 0, "assessment": "PENDING", "last_reviewed_sha": "" }
}
```

## Finding Lifecycle

```
open → resolved         (developer fixed it)
     → partially_fixed  (partially addressed)
     → wontfix          (developer won't fix, reasoning accepted)
     → retracted        (agent was wrong)
```

Every transition: update status, append to status_history with timestamp + note, recompute summary.

## Incremental Re-review (check)

1. Load state — know what SHA was last reviewed
2. Diff only changes since last review
3. For each existing finding: RESOLVED / PARTIALLY FIXED / UNRESOLVED
4. Check for NEW issues from the fixes
5. Update state, output status table

## Rules

1. Never hallucinate file contents — always read the file
2. Concrete fixes only — copy-pasteable code, not "consider using..."
3. Match repo conventions
4. Engage with pushback — developer knows the codebase
5. Severity honesty — don't inflate or downplay
6. No style bikeshedding — focus on logic, correctness, design
7. Cross-file awareness — check if changes break assumptions elsewhere
8. State is truth — always load before acting, save after
REVIEWIQ_EOF
    info "Installed ~/.claude/REVIEWIQ.md (default)"
    fi

    # Add @REVIEWIQ.md to CLAUDE.md if not already there
    local claude_md="$CLAUDE_DIR/CLAUDE.md"
    if [[ -f "$claude_md" ]]; then
        if ! grep -q "@REVIEWIQ.md" "$claude_md" 2>/dev/null; then
            echo "" >> "$claude_md"
            echo "@REVIEWIQ.md" >> "$claude_md"
            info "Added @REVIEWIQ.md to existing ~/.claude/CLAUDE.md"
        else
            info "@REVIEWIQ.md already in ~/.claude/CLAUDE.md"
        fi
    else
        cat > "$claude_md" << 'EOF'
# Claude Code Global Config

@REVIEWIQ.md
EOF
        info "Created ~/.claude/CLAUDE.md with @REVIEWIQ.md"
    fi
}

# ── Verify ───────────────────────────────────────────────────────────────────

verify() {
    if command -v reviewiq &>/dev/null; then
        local ver
        ver="$(reviewiq --version 2>&1)"
        echo ""
        echo -e "${GREEN}${BOLD}Installation successful!${NC}"
        echo ""
        echo -e "  ${ver}"
        echo -e "  Binary:  $(command -v reviewiq)"
        echo -e "  Skills:  $SKILLS_DIR/ ($(ls "$SKILLS_DIR/"*.md 2>/dev/null | wc -l | tr -d ' ') files)"
        echo -e "  Config:  $CLAUDE_DIR/REVIEWIQ.md"
        echo ""
        echo -e "${BOLD}How to use:${NC}"
        echo ""
        echo -e "  ${CYAN}1.${NC} Go to any repo and checkout your feature branch:"
        echo -e "     cd your-project/"
        echo -e "     git checkout feature/my-branch"
        echo ""
        echo -e "  ${CYAN}2.${NC} Open Claude Code and say:"
        echo -e "     ${GREEN}review this PR${NC}                    → reviews current branch against main"
        echo -e "     ${GREEN}review this PR to develop${NC}         → reviews current branch against develop"
        echo ""
        echo -e "  ${CYAN}3.${NC} After review, continue naturally:"
        echo -e "     ${GREEN}explain finding 2${NC}                 → deep dive"
        echo -e "     ${GREEN}fix finding 1${NC}                     → applies the fix"
        echo -e "     ${GREEN}check review${NC}                      → re-review after pushing fixes"
        echo -e "     ${GREEN}retract 3 ORM handles it${NC}          → retract a finding"
        echo -e "     ${GREEN}approve${NC}                           → final check"
        echo ""
        echo -e "  ${CYAN}CLI (optional, needs ANTHROPIC_API_KEY):${NC}"
        echo -e "     reviewiq review <branch>"
        echo -e "     reviewiq status"
        echo ""
        if [[ -n "$SHELL_RC" ]] && ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR" 2>/dev/null; then
            echo -e "${YELLOW}Note: Restart your terminal or run: source $SHELL_RC${NC}"
            echo ""
        fi
    else
        warn "Binary installed but not found in PATH."
        warn "Restart your terminal or run: source $SHELL_RC"
    fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
    echo -e "${BOLD}"
    echo "  ╔══════════════════════════════════════╗"
    echo "  ║         ReviewIQ Installer            ║"
    echo "  ║  AI-Powered PR Review Agent           ║"
    echo "  ╚══════════════════════════════════════╝"
    echo -e "${NC}"

    check_git
    check_go
    cleanup_old
    install_binary
    setup_path
    install_claude_config
    verify
}

main
