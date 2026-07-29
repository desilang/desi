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

    # The archive holds bin/ and lib/. Both matter: lib/ carries libdesi,
    # the C runtime every compiled program links against, and desic finds it
    # at ../lib relative to its own location — so the two must stay siblings.
    LIB_DIR="$INSTALL_DIR/lib"
    mkdir -p "$LIB_DIR"

    if [ ! -d "${FILENAME}/bin" ] || [ ! -d "${FILENAME}/lib" ]; then
        err "Archive layout unexpected: ${FILENAME} has no bin/ and lib/. Report this at https://github.com/desilang/desi/issues"
    fi

    cp -R "${FILENAME}/bin/." "$BIN_DIR/"
    cp -R "${FILENAME}/lib/." "$LIB_DIR/"
    chmod +x "$BIN_DIR"/* 2>/dev/null || true

    ok "Installed to ${BIN_DIR}"
}

# ── Prerequisites ──────────────────────────────────────────────────
# Desi compiles to native code: desic emits LLVM IR and then calls clang to
# assemble and link it. Without clang the install still succeeds, but the
# first `desic build` fails — so say so here rather than there.
check_prereqs() {
    MISSING=""

    if ! command -v clang >/dev/null 2>&1; then
        MISSING="clang"
    fi

    if [ -z "$MISSING" ]; then
        return
    fi

    warn "clang was not found on your PATH."
    printf "  Desi compiles through LLVM, so clang is required to build programs.\n"
    case "$OS" in
        darwin)
            printf "  Install it with:  ${CYAN}xcode-select --install${RESET}\n"
            printf "  or:               ${CYAN}brew install llvm${RESET}\n"
            ;;
        linux)
            if command -v apt-get >/dev/null 2>&1; then
                printf "  Install it with:  ${CYAN}sudo apt-get install clang libssl-dev${RESET}\n"
            elif command -v dnf >/dev/null 2>&1; then
                printf "  Install it with:  ${CYAN}sudo dnf install clang openssl-devel${RESET}\n"
            elif command -v pacman >/dev/null 2>&1; then
                printf "  Install it with:  ${CYAN}sudo pacman -S clang openssl${RESET}\n"
            else
                printf "  Install clang and your distribution's OpenSSL development package.\n"
            fi
            ;;
        *)
            printf "  Install LLVM/clang for your platform.\n"
            ;;
    esac
    printf "\n"
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
    check_prereqs
    printf "  Run ${CYAN}desic version${RESET} to verify.\n"
    printf "  Get started: ${CYAN}https://desilang.org/getting-started/${RESET}\n\n"
}

main
