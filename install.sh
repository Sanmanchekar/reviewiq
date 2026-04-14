#!/usr/bin/env bash
#
# ReviewIQ Installer
# Usage: curl -sSL https://raw.githubusercontent.com/Sanmanchekar/reviewiq/main/install.sh | bash
#

set -euo pipefail

REPO="Sanmanchekar/reviewiq"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="reviewiq"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${GREEN}[reviewiq]${NC} $*"; }
warn() { echo -e "${YELLOW}[reviewiq]${NC} $*"; }
error() { echo -e "${RED}[reviewiq]${NC} $*" >&2; exit 1; }

# Detect OS and architecture
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)  os="linux" ;;
        darwin) os="darwin" ;;
        *)      error "Unsupported OS: $os" ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)             error "Unsupported architecture: $arch" ;;
    esac

    echo "${os}_${arch}"
}

# Check for required tools
check_deps() {
    for cmd in curl tar; do
        if ! command -v "$cmd" &>/dev/null; then
            error "Required tool not found: $cmd"
        fi
    done
}

# Get latest release tag
get_latest_version() {
    curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": "\(.*\)".*/\1/'
}

main() {
    info "Installing ReviewIQ..."
    check_deps

    local platform
    platform="$(detect_platform)"
    info "Detected platform: $platform"

    # Try to get latest release; fall back to building from source
    local version
    version="$(get_latest_version 2>/dev/null || echo "")"

    if [ -n "$version" ]; then
        info "Latest version: $version"
        local url="https://github.com/${REPO}/releases/download/${version}/reviewiq_${platform}.tar.gz"

        local tmp
        tmp="$(mktemp -d)"
        trap "rm -rf $tmp" EXIT

        info "Downloading from release..."
        if curl -sSL "$url" -o "$tmp/reviewiq.tar.gz" 2>/dev/null; then
            tar -xzf "$tmp/reviewiq.tar.gz" -C "$tmp"
            if [ -w "$INSTALL_DIR" ]; then
                cp "$tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
            else
                sudo cp "$tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
            fi
            chmod +x "$INSTALL_DIR/$BINARY_NAME"
            info "Installed $BINARY_NAME to $INSTALL_DIR"
            # Create riq symlink
            if [ -w "$INSTALL_DIR" ]; then
                ln -sf "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/riq"
            else
                sudo ln -sf "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR/riq"
            fi
            info "Created symlink: riq -> reviewiq"
            reviewiq --version
            return
        fi
        warn "No pre-built binary found for $platform, building from source..."
    fi

    # Fall back to go install
    if command -v go &>/dev/null; then
        info "Building from source with Go..."
        go install "github.com/${REPO}/cmd/reviewiq@latest"
        # Create riq symlink
        local gobin
        gobin="$(go env GOPATH)/bin"
        if [ -f "$gobin/$BINARY_NAME" ]; then
            ln -sf "$gobin/$BINARY_NAME" "$gobin/riq" 2>/dev/null || true
        fi
        info "Installed via go install"
        reviewiq --version 2>/dev/null || "$gobin/reviewiq" --version
    else
        error "No pre-built binary available and Go is not installed.\n  Install Go from https://go.dev/dl/ and try again, or build manually:\n    git clone https://github.com/${REPO}.git && cd reviewiq && go build -o /usr/local/bin/reviewiq ./cmd/reviewiq/"
    fi
}

main
