#!/bin/sh
# Desi Installer — https://desilang.org
# Usage: curl -sSL https://desilang.org/install.sh | sh
#
# Installs desic, desifmt, and desilsp to ~/.desi/bin
# and adds ~/.desi/bin to your PATH.
#
# Environment variables:
#   DESI_INSTALL_DIR  — Override install directory (default: ~/.desi)
#   DESI_VERSION      — Install a specific version (default: latest)

set -e

# ── Colors ──────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { printf "${CYAN}▸${RESET} %s\n" "$1"; }
ok()    { printf "${GREEN}✓${RESET} %s\n" "$1"; }
warn()  { printf "${YELLOW}⚠${RESET} %s\n" "$1"; }
err()   { printf "${RED}✗${RESET} %s\n" "$1" >&2; exit 1; }

# ── Detect OS & Architecture ───────────────────────────────────────
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *) err "Unsupported operating system: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) err "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${OS}-${ARCH}"
}

# ── Resolve Version ────────────────────────────────────────────────
resolve_version() {
    if [ -n "$DESI_VERSION" ]; then
        VERSION="$DESI_VERSION"
        return
    fi

    info "Fetching latest release..."
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -sSL -H "Accept: application/json" \
            "https://api.github.com/repos/desilang/desi/releases/latest" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- --header="Accept: application/json" \
            "https://api.github.com/repos/desilang/desi/releases/latest" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    else
        err "curl or wget is required"
    fi

    if [ -z "$VERSION" ]; then
        err "Could not determine latest version. Set DESI_VERSION manually."
    fi
}

# ── Download & Install ─────────────────────────────────────────────
install() {
    INSTALL_DIR="${DESI_INSTALL_DIR:-$HOME/.desi}"
    BIN_DIR="$INSTALL_DIR/bin"
    mkdir -p "$BIN_DIR"

    # Build download URL
    FILENAME="desi-${VERSION}-${PLATFORM}"
    if [ "$OS" = "windows" ]; then
        ARCHIVE="${FILENAME}.zip"
    else
        ARCHIVE="${FILENAME}.tar.gz"
    fi
    URL="https://github.com/desilang/desi/releases/download/${VERSION}/${ARCHIVE}"

    info "Downloading Desi ${VERSION} for ${PLATFORM}..."
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    if command -v curl >/dev/null 2>&1; then
        curl -sSL "$URL" -o "$TMPDIR/$ARCHIVE" || err "Download failed. Check the version and platform."
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$URL" -O "$TMPDIR/$ARCHIVE" || err "Download failed. Check the version and platform."
    fi

    info "Extracting..."
    cd "$TMPDIR"
    if [ "$OS" = "windows" ]; then
        unzip -qo "$ARCHIVE"
    else
        tar xzf "$ARCHIVE"
    fi

    # Copy binaries
    cp "${FILENAME}/desic"   "$BIN_DIR/" 2>/dev/null || true
    cp "${FILENAME}/desifmt" "$BIN_DIR/" 2>/dev/null || true
    cp "${FILENAME}/desilsp" "$BIN_DIR/" 2>/dev/null || true

    # Windows: copy .exe variants
    cp "${FILENAME}/desic.exe"   "$BIN_DIR/" 2>/dev/null || true
    cp "${FILENAME}/desifmt.exe" "$BIN_DIR/" 2>/dev/null || true
    cp "${FILENAME}/desilsp.exe" "$BIN_DIR/" 2>/dev/null || true

    chmod +x "$BIN_DIR"/* 2>/dev/null || true

    ok "Installed to ${BIN_DIR}"
}

# ── Update PATH ────────────────────────────────────────────────────
update_path() {
    BIN_DIR="${DESI_INSTALL_DIR:-$HOME/.desi}/bin"
    EXPORT_LINE="export PATH=\"${BIN_DIR}:\$PATH\""

    # Check if already in PATH
    case ":$PATH:" in
        *":${BIN_DIR}:"*) return ;;
    esac

    # Detect shell config
    SHELL_NAME=$(basename "$SHELL" 2>/dev/null || echo "sh")
    case "$SHELL_NAME" in
        zsh)  RC="$HOME/.zshrc" ;;
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                RC="$HOME/.bashrc"
            else
                RC="$HOME/.bash_profile"
            fi
            ;;
        fish)
            FISH_LINE="set -gx PATH ${BIN_DIR} \$PATH"
            RC="$HOME/.config/fish/config.fish"
            mkdir -p "$(dirname "$RC")"
            if ! grep -qF "$BIN_DIR" "$RC" 2>/dev/null; then
                echo "" >> "$RC"
                echo "# Desi" >> "$RC"
                echo "$FISH_LINE" >> "$RC"
                warn "Added to ${RC} — restart your shell or run: source ${RC}"
            fi
            return
            ;;
        *)    RC="$HOME/.profile" ;;
    esac

    if ! grep -qF "$BIN_DIR" "$RC" 2>/dev/null; then
        echo "" >> "$RC"
        echo "# Desi" >> "$RC"
        echo "$EXPORT_LINE" >> "$RC"
        warn "Added to ${RC} — restart your shell or run: source ${RC}"
    fi
}

# ── Main ───────────────────────────────────────────────────────────
main() {
    printf "\n${BOLD}  Desi Installer${RESET}\n\n"

    detect_platform
    resolve_version
    install
    update_path

    printf "\n${GREEN}${BOLD}  Desi ${VERSION} installed successfully!${RESET}\n\n"
    printf "  Run ${CYAN}desic --version${RESET} to verify.\n"
    printf "  Get started: ${CYAN}https://desilang.org/getting-started/${RESET}\n\n"
}

main
