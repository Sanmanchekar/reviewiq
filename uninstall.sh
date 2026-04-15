#!/usr/bin/env bash
#
# ReviewIQ Uninstaller
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/uninstall.sh -o /tmp/reviewiq-uninstall.sh && bash /tmp/reviewiq-uninstall.sh
#

set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
SKILLS_DIR="$HOME/.reviewiq"
CLAUDE_DIR="$HOME/.claude"
BINARY="reviewiq"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${GREEN}[reviewiq]${NC} $*"; }
warn()  { echo -e "${YELLOW}[reviewiq]${NC} $*"; }

main() {
    echo -e "${BOLD}"
    echo "  ╔══════════════════════════════════════╗"
    echo "  ║        ReviewIQ Uninstaller           ║"
    echo "  ╚══════════════════════════════════════╝"
    echo -e "${NC}"

    local found=false

    # 1. Remove binaries
    for dir in "$INSTALL_DIR" "/usr/local/bin" "${GOPATH:-$HOME/go}/bin"; do
        for f in "$dir/$BINARY" "$dir/riq"; do
            if [[ -f "$f" ]] || [[ -L "$f" ]]; then
                if [[ -w "$f" ]] || [[ -w "$(dirname "$f")" ]]; then
                    rm -f "$f"
                else
                    sudo rm -f "$f"
                fi
                info "Removed $f"
                found=true
            fi
        done
    done

    # 2. Remove global skills
    if [[ -d "$SKILLS_DIR" ]]; then
        rm -rf "$SKILLS_DIR"
        info "Removed $SKILLS_DIR/"
        found=true
    fi

    # 3. Remove Claude Code config
    if [[ -f "$CLAUDE_DIR/REVIEWIQ.md" ]]; then
        rm -f "$CLAUDE_DIR/REVIEWIQ.md"
        info "Removed $CLAUDE_DIR/REVIEWIQ.md"
        found=true
    fi

    # Remove @REVIEWIQ.md reference from CLAUDE.md
    if [[ -f "$CLAUDE_DIR/CLAUDE.md" ]]; then
        if grep -q "@REVIEWIQ.md" "$CLAUDE_DIR/CLAUDE.md" 2>/dev/null; then
            sed -i.bak '/@REVIEWIQ.md/d' "$CLAUDE_DIR/CLAUDE.md"
            rm -f "$CLAUDE_DIR/CLAUDE.md.bak"
            info "Removed @REVIEWIQ.md from $CLAUDE_DIR/CLAUDE.md"
        fi
    fi

    # 4. Remove old Python package
    if pip show reviewiq &>/dev/null 2>&1; then
        pip uninstall reviewiq -y 2>/dev/null || true
        info "Removed Python package"
        found=true
    fi

    # 5. Clean repo-level files (if in a git repo)
    if git rev-parse --is-inside-work-tree &>/dev/null; then
        local repo_root
        repo_root="$(git rev-parse --show-toplevel)"

        # Remove slash command files
        local removed_repo=false
        for pattern in "reviewiq-*.md" "review-*.md"; do
            for f in "$repo_root/.claude/commands/"$pattern; do
                if [[ -f "$f" ]]; then
                    rm -f "$f"
                    removed_repo=true
                fi
            done
        done
        if $removed_repo; then
            info "Removed slash commands from $repo_root/.claude/commands/"
            found=true
        fi

        # Remove .pr-review/ directory
        if [[ -d "$repo_root/.pr-review" ]]; then
            rm -rf "$repo_root/.pr-review"
            info "Removed $repo_root/.pr-review/"
            found=true
        fi
    fi

    # 6. Clean PATH entries from shell rc files
    for rc in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [[ -f "$rc" ]] && grep -q "# ReviewIQ" "$rc" 2>/dev/null; then
            sed -i.bak '/# ReviewIQ/d' "$rc"
            sed -i.bak "\|$INSTALL_DIR|d" "$rc"
            rm -f "${rc}.bak"
            info "Cleaned PATH entry from $rc"
        fi
    done

    if $found; then
        echo ""
        echo -e "${GREEN}${BOLD}ReviewIQ uninstalled.${NC}"
        echo ""
        echo -e "Removed:"
        echo -e "  - Binary (reviewiq, riq)"
        echo -e "  - Global skills (~/.reviewiq/)"
        echo -e "  - Claude Code config (~/.claude/REVIEWIQ.md)"
        echo -e "  - Repo slash commands (.claude/commands/reviewiq-*.md)"
        echo -e "  - Repo review config (.pr-review/)"
        echo -e "  - PATH entries"
    else
        warn "ReviewIQ not found. Nothing to uninstall."
    fi
}

main
