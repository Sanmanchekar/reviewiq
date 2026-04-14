#!/usr/bin/env bash
#
# ReviewIQ Uninstaller
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/uninstall.sh | bash
#

set -euo pipefail

INSTALL_DIR="$HOME/.local/bin"
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

    # Remove binary from ~/.local/bin
    for f in "$INSTALL_DIR/$BINARY" "$INSTALL_DIR/riq"; do
        if [[ -f "$f" ]] || [[ -L "$f" ]]; then
            rm -f "$f"
            info "Removed $f"
            found=true
        fi
    done

    # Remove binary from /usr/local/bin (if installed there)
    for f in "/usr/local/bin/$BINARY" "/usr/local/bin/riq"; do
        if [[ -f "$f" ]] || [[ -L "$f" ]]; then
            if [[ -w "$f" ]]; then
                rm -f "$f"
            else
                sudo rm -f "$f"
            fi
            info "Removed $f"
            found=true
        fi
    done

    # Remove from go/bin (if installed via go install)
    local gobin="${GOPATH:-$HOME/go}/bin"
    for f in "$gobin/$BINARY" "$gobin/riq"; do
        if [[ -f "$f" ]] || [[ -L "$f" ]]; then
            rm -f "$f"
            info "Removed $f"
            found=true
        fi
    done

    # Remove old Python version if present
    if pip show reviewiq &>/dev/null 2>&1; then
        pip uninstall reviewiq -y 2>/dev/null || true
        info "Removed Python package"
        found=true
    fi

    # Clean PATH entry from shell rc files
    for rc in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [[ -f "$rc" ]] && grep -q "# ReviewIQ" "$rc" 2>/dev/null; then
            # Remove the ReviewIQ block (comment + export line)
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
        echo -e "Note: .pr-review/ and .claude/commands/review-*.md in your repos"
        echo -e "were NOT removed (they're part of the repo, not the install)."
        echo -e "Delete them manually if you want:"
        echo -e "  rm -rf .pr-review/ .claude/commands/review-*.md"
    else
        warn "ReviewIQ not found. Nothing to uninstall."
    fi
}

main
