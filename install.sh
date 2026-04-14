#!/usr/bin/env bash
#
# ReviewIQ Installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
#
# What it does:
#   1. Clones the repo (shallow)
#   2. Builds the Go binary
#   3. Installs to ~/.local/bin (no sudo needed)
#   4. Adds to PATH if not already there
#   5. Creates riq symlink
#   6. Cleans up
#
# Requires: git, go (>= 1.22)
#

set -euo pipefail

REPO="https://github.com/Sanmanchekar/reviewiq.git"
INSTALL_DIR="$HOME/.local/bin"
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

# ── Remove old Python version ────────────────────────────────────────────────

cleanup_old() {
    # Check for old Python-based reviewiq
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

# ── Install ──────────────────────────────────────────────────────────────────

ensure_install_dir() {
    mkdir -p "$INSTALL_DIR"
}

add_to_path() {
    if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        return 0
    fi

    detect_shell_rc

    if [[ -n "$SHELL_RC" ]] && [[ -f "$SHELL_RC" ]]; then
        if ! grep -q "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
            echo "" >> "$SHELL_RC"
            echo "# ReviewIQ" >> "$SHELL_RC"
            echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$SHELL_RC"
            info "Added $INSTALL_DIR to PATH in $SHELL_RC"
        fi
    fi

    # Also add to current session
    export PATH="$INSTALL_DIR:$PATH"
}

build_and_install() {
    local tmp
    tmp="$(mktemp -d)"
    trap "rm -rf $tmp" EXIT

    step "Cloning repository..."
    git clone --depth 1 "$REPO" "$tmp/reviewiq" 2>/dev/null

    step "Building binary..."
    cd "$tmp/reviewiq"
    go build -o "$INSTALL_DIR/$BINARY" ./cmd/reviewiq/
    chmod +x "$INSTALL_DIR/$BINARY"

    # Create riq shorthand symlink
    ln -sf "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/riq"

    cd - >/dev/null
    info "Built and installed to $INSTALL_DIR/$BINARY"
}

auto_init() {
    # Run reviewiq init in the current directory if it's a git repo
    if git rev-parse --is-inside-work-tree &>/dev/null; then
        local repo_root
        repo_root="$(git rev-parse --show-toplevel)"
        if [[ ! -f "$repo_root/.pr-review/agent.md" ]] || [[ ! -d "$repo_root/.claude/commands" ]]; then
            step "Detected git repo at $repo_root — running reviewiq init..."
            cd "$repo_root"
            reviewiq init
            cd - >/dev/null
        else
            info "Repo already initialized (.pr-review/ and .claude/commands/ exist)"
        fi
    else
        info "Not inside a git repo — skipping auto-init."
        info "Run 'reviewiq init' inside any git repo to set up slash commands."
    fi
}

verify() {
    if command -v reviewiq &>/dev/null; then
        local ver
        ver="$(reviewiq --version 2>&1)"
        echo ""
        echo -e "${GREEN}${BOLD}Installation successful!${NC}"
        echo -e "  ${ver}"
        echo -e "  Binary: $(command -v reviewiq)"
        echo ""
        echo -e "${BOLD}Usage:${NC}"
        echo -e "  ${CYAN}Claude Code:${NC}  /review-pr <branch>  (+ 12 other /review-* commands)"
        echo -e "  ${CYAN}CLI:${NC}          reviewiq review <branch>  (needs ANTHROPIC_API_KEY)"
        echo -e "  ${CYAN}New repo:${NC}     cd other-project/ && reviewiq init"
        echo ""
        if [[ -n "$SHELL_RC" ]] && ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR" 2>/dev/null; then
            echo -e "${YELLOW}Note: Restart your terminal or run:${NC}"
            echo -e "  source $SHELL_RC"
            echo ""
        fi
    else
        warn "Binary installed to $INSTALL_DIR/$BINARY but not found in PATH."
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
    ensure_install_dir
    add_to_path
    build_and_install
    auto_init
    verify
}

main
