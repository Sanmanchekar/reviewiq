#!/usr/bin/env bash
#
# ReviewIQ Installer
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
#
# What it does:
#   1. Checks/installs dependencies (git, go >= 1.22, gh CLI)
#   2. Builds and installs the Go binary to ~/.local/bin
#   3. Copies review skills to ~/.reviewiq/skills/ (global)
#   4. Installs Claude Code global config (~/.claude/REVIEWIQ.md)
#   5. Sets up PATH
#   6. Done — works in every repo, no per-repo init needed
#
# Requires: curl (everything else is auto-installed if missing)
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

# ── Checks & Auto-Install ────────────────────────────────────────────────────

check_curl() {
    if ! command -v curl &>/dev/null; then
        error "curl is required but not installed. Install it first."
    fi
}

check_git() {
    if command -v git &>/dev/null; then
        info "Found git $(git --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo '')"
        return
    fi

    step "Installing git..."
    local os
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"

    case "$os" in
        darwin)
            # macOS: xcode-select installs git
            xcode-select --install 2>/dev/null || true
            # Wait briefly for the install dialog
            if ! command -v git &>/dev/null; then
                error "git not found. Accept the Xcode Command Line Tools prompt and re-run."
            fi
            ;;
        linux)
            if command -v apt-get &>/dev/null; then
                sudo apt-get update -qq && sudo apt-get install -y -qq git
            elif command -v dnf &>/dev/null; then
                sudo dnf install -y git
            elif command -v yum &>/dev/null; then
                sudo yum install -y git
            elif command -v pacman &>/dev/null; then
                sudo pacman -S --noconfirm git
            else
                error "git is not installed. Install it manually."
            fi
            ;;
        *) error "git is not installed. Install it manually." ;;
    esac

    if command -v git &>/dev/null; then
        info "Installed git $(git --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo '')"
    else
        error "Failed to install git."
    fi
}

check_go() {
    local required_major=1
    local required_minor=22

    if command -v go &>/dev/null; then
        local ver
        ver="$(go version </dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)"
        local major minor
        major="$(echo "$ver" | cut -d. -f1)"
        minor="$(echo "$ver" | cut -d. -f2)"
        if [[ "$major" -gt "$required_major" ]] || { [[ "$major" -eq "$required_major" ]] && [[ "$minor" -ge "$required_minor" ]]; }; then
            info "Found Go $ver"
            return
        fi
        warn "Found Go $ver but >= ${required_major}.${required_minor} is required. Upgrading..."
    fi

    step "Installing Go..."
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) arch="amd64" ;;
    esac

    # Fetch latest stable Go version
    local go_ver
    go_ver="$(curl -sSL 'https://go.dev/VERSION?m=text' | head -1 | sed 's/go//')" || go_ver="1.23.4"

    local go_url="https://go.dev/dl/go${go_ver}.${os}-${arch}.tar.gz"
    local go_tmp
    go_tmp="$(mktemp -d)"

    curl -sSL "$go_url" -o "$go_tmp/go.tar.gz"

    # Install to /usr/local (standard location)
    if [[ -w /usr/local ]]; then
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "$go_tmp/go.tar.gz"
    else
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "$go_tmp/go.tar.gz"
    fi
    rm -rf "$go_tmp"

    export PATH="/usr/local/go/bin:$PATH"

    # Persist in shell RC
    detect_shell_rc
    if [[ -n "$SHELL_RC" ]]; then
        if ! grep -q '/usr/local/go/bin' "$SHELL_RC" 2>/dev/null; then
            echo "" >> "$SHELL_RC"
            echo "# Go" >> "$SHELL_RC"
            echo 'export PATH="/usr/local/go/bin:$PATH"' >> "$SHELL_RC"
        fi
    fi

    if command -v go &>/dev/null; then
        info "Installed Go $(go version </dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1)"
    else
        error "Failed to install Go. Install manually from https://go.dev/dl/"
    fi
}

install_gh() {
    if command -v gh &>/dev/null; then
        info "Found gh $(gh --version 2>/dev/null | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo '')"
        return
    fi

    step "Installing GitHub CLI (gh)..."
    local os arch gh_installed=false
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) arch="amd64" ;;
    esac

    case "$os" in
        darwin)
            if command -v brew &>/dev/null; then
                brew install gh 2>&1 | tail -3 || true
                gh_installed=true
            else
                # Direct download for macOS without brew
                local gh_ver
                gh_ver=$(curl -sSL https://api.github.com/repos/cli/cli/releases/latest | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/' || echo "2.89.0")
                local gh_url="https://github.com/cli/cli/releases/download/v${gh_ver}/gh_${gh_ver}_macOS_${arch}.zip"
                local gh_tmp="$(mktemp -d)"
                curl -sSL "$gh_url" -o "$gh_tmp/gh.zip"
                unzip -q "$gh_tmp/gh.zip" -d "$gh_tmp"
                cp "$gh_tmp"/gh_*/bin/gh "$INSTALL_DIR/gh"
                chmod +x "$INSTALL_DIR/gh"
                rm -rf "$gh_tmp"
                gh_installed=true
            fi
            ;;
        linux)
            if command -v apt-get &>/dev/null; then
                # Debian/Ubuntu
                local gh_ver
                gh_ver=$(curl -sSL https://api.github.com/repos/cli/cli/releases/latest | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/' || echo "2.89.0")
                local gh_url="https://github.com/cli/cli/releases/download/v${gh_ver}/gh_${gh_ver}_linux_${arch}.deb"
                local gh_tmp="$(mktemp -d)"
                curl -sSL "$gh_url" -o "$gh_tmp/gh.deb"
                sudo dpkg -i "$gh_tmp/gh.deb" 2>/dev/null || sudo apt-get install -f -y 2>/dev/null
                rm -rf "$gh_tmp"
                gh_installed=true
            elif command -v yum &>/dev/null; then
                # RHEL/CentOS
                local gh_ver
                gh_ver=$(curl -sSL https://api.github.com/repos/cli/cli/releases/latest | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/' || echo "2.89.0")
                local gh_url="https://github.com/cli/cli/releases/download/v${gh_ver}/gh_${gh_ver}_linux_${arch}.rpm"
                local gh_tmp="$(mktemp -d)"
                curl -sSL "$gh_url" -o "$gh_tmp/gh.rpm"
                sudo yum install -y "$gh_tmp/gh.rpm" 2>/dev/null || true
                rm -rf "$gh_tmp"
                gh_installed=true
            elif command -v dnf &>/dev/null; then
                # Fedora
                sudo dnf install -y gh 2>/dev/null || true
                gh_installed=true
            else
                # Generic Linux — download binary directly
                local gh_ver
                gh_ver=$(curl -sSL https://api.github.com/repos/cli/cli/releases/latest | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/' || echo "2.89.0")
                local gh_url="https://github.com/cli/cli/releases/download/v${gh_ver}/gh_${gh_ver}_linux_${arch}.tar.gz"
                local gh_tmp="$(mktemp -d)"
                curl -sSL "$gh_url" -o "$gh_tmp/gh.tar.gz"
                tar -xzf "$gh_tmp/gh.tar.gz" -C "$gh_tmp"
                cp "$gh_tmp"/gh_*/bin/gh "$INSTALL_DIR/gh"
                chmod +x "$INSTALL_DIR/gh"
                rm -rf "$gh_tmp"
                gh_installed=true
            fi
            ;;
    esac

    if command -v gh &>/dev/null; then
        info "Installed gh $(gh --version 2>/dev/null | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo '')"
    elif $gh_installed; then
        info "gh installed to $INSTALL_DIR"
    else
        warn "Could not install gh CLI. Install manually: https://cli.github.com"
    fi
}

setup_gh_auth() {
    # Skip if gh not installed
    if ! command -v gh &>/dev/null; then
        return
    fi

    # Check if already authenticated (via browser login, SSH, credential manager, etc.)
    if gh auth status &>/dev/null; then
        info "gh already authenticated (reusing existing git credentials)"
        return
    fi

    step "GitHub authentication required for PR reviews."
    echo ""
    echo -e "  ReviewIQ reuses your existing git credentials — no token needed."
    echo -e "  Run: ${BOLD}gh auth login${NC}"
    echo -e "  This opens a browser for GitHub OAuth (same auth git uses)."
    echo ""
    warn "Run 'gh auth login' to enable PR reviews."
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

    # Clean up old review-*.md command files in current repo (if in a git repo)
    if git rev-parse --is-inside-work-tree &>/dev/null; then
        local repo_root
        repo_root="$(git rev-parse --show-toplevel)"
        local old_cmds
        old_cmds=$(ls "$repo_root/.claude/commands/review-"*.md 2>/dev/null || true)
        if [[ -n "$old_cmds" ]]; then
            rm -f "$repo_root/.claude/commands/review-"*.md 2>/dev/null || true
            info "Removed old review-*.md commands (renamed to reviewiq-*.md)"
        fi
    fi
}

# ── Step 1: Build and install binary ─────────────────────────────────────────

install_binary() {
    mkdir -p "$INSTALL_DIR"

    local tmp
    tmp="$(mktemp -d)"
    trap "rm -rf $tmp" EXIT

    step "Downloading source..."
    curl -sSL "https://github.com/Sanmanchekar/reviewiq/archive/refs/heads/main.tar.gz" -o "$tmp/reviewiq.tar.gz" </dev/null
    tar -xzf "$tmp/reviewiq.tar.gz" -C "$tmp" </dev/null
    mv "$tmp/reviewiq-main" "$tmp/reviewiq"

    step "Building binary..."
    cd "$tmp/reviewiq"
    go build -o "$INSTALL_DIR/$BINARY" ./cmd/reviewiq/ </dev/null
    chmod +x "$INSTALL_DIR/$BINARY"
    ln -sf "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/riq"

    # Copy skills to global location
    step "Installing skills to $SKILLS_DIR..."
    mkdir -p "$SKILLS_DIR"
    cp "$tmp/reviewiq/.pr-review/skills/"*.md "$SKILLS_DIR/" 2>/dev/null || true

    # Copy agent.md and REVIEWIQ.md to global location
    mkdir -p "$HOME/.reviewiq"
    cp "$tmp/reviewiq/.pr-review/agent.md" "$HOME/.reviewiq/agent.md" 2>/dev/null || true

    # Copy REVIEWIQ.md directly to Claude config (before tmp cleanup)
    mkdir -p "$CLAUDE_DIR"
    cp "$tmp/reviewiq/REVIEWIQ.md" "$CLAUDE_DIR/REVIEWIQ.md" 2>/dev/null || true

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

    # REVIEWIQ.md was already copied from repo in install_binary()
    if [[ -f "$CLAUDE_DIR/REVIEWIQ.md" ]]; then
        info "~/.claude/REVIEWIQ.md installed (from repo)"
    else
        warn "REVIEWIQ.md not found — install may have failed"
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

# ── Step 4: Init current repo ─────────────────────────────────────────────────

repo_init() {
    if ! git rev-parse --is-inside-work-tree &>/dev/null; then
        info "Not inside a git repo — skipping repo init."
        info "Run 'reviewiq init </dev/null' inside any git repo to set up slash commands."
        return
    fi

    local repo_root
    repo_root="$(git rev-parse --show-toplevel)"

    step "Setting up repo at $repo_root..."
    cd "$repo_root"

    # Run reviewiq init </dev/null (creates .pr-review/, .claude/commands/reviewiq-*, cleans old review-*)
    reviewiq init </dev/null

    cd - >/dev/null
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

    check_curl
    check_git
    check_go
    install_gh
    setup_gh_auth
    cleanup_old
    install_binary
    setup_path
    install_claude_config
    repo_init
    verify
}

main
