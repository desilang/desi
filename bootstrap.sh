#!/bin/bash
# One-shot developer bootstrap for building Desi from source on macOS and Linux.
#
# Checks for the toolchain Desi needs, installs anything missing, then
# builds the compiler + runtime. Optionally runs the example test suite.
#
# Installs (only what's missing):
#   - Go           (compiler + tools)
#   - LLVM / Clang (LLVM IR -> object code + linking)
#   - OpenSSL      (HTTPS / database support)
#   - Make         (build orchestration)
#
# On macOS, dependencies are installed via Homebrew.
# On Linux, dependencies are installed via apt, dnf, or pacman.

set -e

# ── Colors & Output Helpers ─────────────────────────────────────────
CYAN="\033[0;36m"
GREEN="\033[0;32m"
YELLOW="\033[1;33m"
RED="\033[0;31m"
BOLD="\033[1m"
RESET="\033[0m"

info() { printf "${CYAN}==>${RESET} %s\n" "$1"; }
ok()   { printf "${GREEN}[OK]${RESET} %s\n" "$1"; }
warn() { printf "${YELLOW}[!]${RESET}  %s\n" "$1"; }
err()  { printf "${RED}[ERROR]${RESET} %s\n" "$1" >&2; exit 1; }

# ── Help / Usage ───────────────────────────────────────────────────
show_help() {
    cat << EOF
Desi — build-from-source bootstrap (macOS/Linux)

Usage: $0 [options]

Options:
  -t, --test         Run the example test suite (test_examples.sh) after building.
  -s, --skip-build   Install and verify dependencies only; do not build.
  -c, --clean        Run 'make clean' to clear previous build artifacts.
  -n, --no-elevate   Do not use 'sudo' for Linux package installations.
  -h, --help         Show this help message.

Examples:
  $0                 # Install missing tools and build Desi
  $0 --test          # ...and run the example suite
  $0 --clean --test  # Clean rebuild + run suite
EOF
    exit 0
}

# ── Parse Arguments ────────────────────────────────────────────────
TEST_SUITE=false
SKIP_BUILD=false
CLEAN_BUILD=false
NO_ELEVATE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        -t|--test)
            TEST_SUITE=true
            shift
            ;;
        -s|--skip-build)
            SKIP_BUILD=true
            shift
            ;;
        -c|--clean)
            CLEAN_BUILD=true
            shift
            ;;
        -n|--no-elevate)
            NO_ELEVATE=true
            shift
            ;;
        -h|--help)
            show_help
            ;;
        *)
            err "Unknown option: $1. Run with --help for usage."
            ;;
    esac
done

# ── Detect OS & Architecture ───────────────────────────────────────
OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" != "Darwin" ] && [ "$OS" != "Linux" ]; then
    err "Unsupported operating system: $OS. Desi bootstrap supports Darwin (macOS) and Linux."
fi

printf "\n${BOLD}  Desi — build-from-source bootstrap ($OS)${RESET}\n\n"

# ── Path Setup & LLVM Detection Helpers ─────────────────────────────
# Temporarily add Homebrew LLVM paths if present to make check & build succeed
if [ "$OS" = "Darwin" ]; then
    if [ -d "/opt/homebrew/opt/llvm/bin" ]; then
        export PATH="/opt/homebrew/opt/llvm/bin:$PATH"
    elif [ -d "/usr/local/opt/llvm/bin" ]; then
        export PATH="/usr/local/opt/llvm/bin:$PATH"
    fi
fi

find_llvm_tool() {
    local name="$1"
    if command -v "$name" >/dev/null 2>&1; then
        command -v "$name"
        return 0
    fi
    # macOS candidate prefixes
    for prefix in /opt/homebrew/opt/llvm/bin /usr/local/opt/llvm/bin; do
        if [ -x "$prefix/$name" ]; then
            echo "$prefix/$name"
            return 0
        fi
    done
    # Linux candidate prefixes
    for v in 20 19 18 17 16 15 14; do
        if [ -x "/usr/lib/llvm-$v/bin/$name" ]; then
            echo "/usr/lib/llvm-$v/bin/$name"
            return 0
        fi
    done
    return 1
}

# ── Prerequisite Checks ────────────────────────────────────────────

is_valid_go() {
    local bin="$1"
    [ -x "$bin" ] || return 1
    local ver_str
    ver_str=$("$bin" version 2>/dev/null | awk '{print $3}' | sed 's/go//' || echo "")
    [ -n "$ver_str" ] || return 1
    local major minor
    major=$(echo "$ver_str" | cut -d. -f1)
    minor=$(echo "$ver_str" | cut -d. -f2)
    if [ -z "$minor" ]; then
        minor=0
    fi
    # If version has suffix (e.g. 1.25rc1), remove it
    minor=$(echo "$minor" | sed 's/[^0-9].*//')
    if [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 21 ]; }; then
        return 0
    fi
    return 1
}

find_go() {
    # 1. Check if GO env var is set and valid
    if [ -n "$GO" ] && is_valid_go "$GO"; then
        echo "$GO"
        return 0
    fi
    # 2. Check if go is on the PATH and valid
    if command -v go >/dev/null 2>&1 && is_valid_go "$(command -v go)"; then
        echo "$(command -v go)"
        return 0
    fi
    # 3. Check official default installer path
    if is_valid_go "/usr/local/go/bin/go"; then
        echo "/usr/local/go/bin/go"
        return 0
    fi
    # 4. Check typical home directory sdk paths
    if is_valid_go "$HOME/go/bin/go"; then
        echo "$HOME/go/bin/go"
        return 0
    fi
    # 5. Check other SDK folders (like go1.25.0 in user's home/sdk)
    for d in "$HOME/go"/go*/bin/go "$HOME/go"/*/bin/go "$HOME/sdk"/go*/bin/go "$HOME"/go*/bin/go; do
        if is_valid_go "$d"; then
            echo "$d"
            return 0
        fi
    done
    return 1
}

# 1. Git
info "Checking Git..."
if command -v git >/dev/null 2>&1; then
    ok "Git found ($(git --version | head -n1))"
else
    warn "Git is not installed. You will need it to manage the repository."
    MISSING_GIT=true
fi

# 2. Go (1.21+)
info "Checking Go..."
GO_FOUND=false
GO_BIN=""
if GO_BIN_PATH=$(find_go); then
    GO_VERSION_STR=$("$GO_BIN_PATH" version | awk '{print $3}' | sed 's/go//')
    ok "Go found: $GO_BIN_PATH (version $GO_VERSION_STR)"
    GO_FOUND=true
    GO_BIN="$GO_BIN_PATH"
fi

# 3. LLVM / Clang (llc and clang)
info "Checking LLVM / Clang..."
LLVM_FOUND=false
if LLC_PATH=$(find_llvm_tool llc) && CLANG_PATH=$(find_llvm_tool clang); then
    ok "LLVM found: llc ($LLC_PATH), clang ($CLANG_PATH)"
    LLVM_FOUND=true
else
    warn "LLVM/Clang (llc or clang) not found."
fi

# 4. Make
info "Checking Make..."
MAKE_FOUND=false
if command -v make >/dev/null 2>&1; then
    ok "Make found ($(make --version | head -n1 | cut -d' ' -f1-2))"
    MAKE_FOUND=true
else
    warn "Make not found."
fi

# 5. OpenSSL
info "Checking OpenSSL..."
OPENSSL_FOUND=false
# Check standard headers or brew prefixes
if [ "$OS" = "Darwin" ]; then
    BREW_OPENSSL=""
    if command -v brew >/dev/null 2>&1; then
        BREW_OPENSSL=$(brew --prefix openssl 2>/dev/null || brew --prefix openssl@3 2>/dev/null || echo "")
    fi
    if [ -n "$BREW_OPENSSL" ] && [ -f "$BREW_OPENSSL/include/openssl/ssl.h" ]; then
        ok "OpenSSL found (via Homebrew at $BREW_OPENSSL)"
        OPENSSL_FOUND=true
    elif [ -f "/usr/include/openssl/ssl.h" ]; then
        ok "OpenSSL found in system headers"
        OPENSSL_FOUND=true
    fi
else
    # Linux
    if pkg-config --exists openssl 2>/dev/null; then
        ok "OpenSSL found (via pkg-config)"
        OPENSSL_FOUND=true
    elif [ -f "/usr/include/openssl/ssl.h" ] || [ -f "/usr/local/include/openssl/ssl.h" ]; then
        ok "OpenSSL found in system headers"
        OPENSSL_FOUND=true
    fi
fi

if [ "$OPENSSL_FOUND" = "false" ]; then
    warn "OpenSSL development headers or libraries not found (required for HTTPS / DB)."
fi

# ── Install Missing Dependencies ────────────────────────────────────

SUDO="sudo"
if [ "$NO_ELEVATE" = "true" ] || [ "$USER" = "root" ]; then
    SUDO=""
fi

if [ "$GO_FOUND" = "false" ] || [ "$LLVM_FOUND" = "false" ] || [ "$MAKE_FOUND" = "false" ] || [ "$OPENSSL_FOUND" = "false" ] || [ "$MISSING_GIT" = "true" ]; then
    info "Installing missing dependencies..."

    if [ "$OS" = "Darwin" ]; then
        # macOS
        if ! command -v brew >/dev/null 2>&1; then
            err "Homebrew is required to install missing dependencies on macOS.\n         Please install it from https://brew.sh/ and re-run this script."
        fi

        TO_INSTALL=""
        [ "$MISSING_GIT" = "true" ] && TO_INSTALL="$TO_INSTALL git"
        [ "$GO_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL go"
        [ "$LLVM_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL llvm"
        [ "$OPENSSL_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL openssl"

        if [ -n "$TO_INSTALL" ]; then
            info "Running: brew install $TO_INSTALL"
            brew install $TO_INSTALL
            ok "Homebrew installation complete."
            
            # Re-setup path if LLVM was just installed
            if [ -d "/opt/homebrew/opt/llvm/bin" ]; then
                export PATH="/opt/homebrew/opt/llvm/bin:$PATH"
            elif [ -d "/usr/local/opt/llvm/bin" ]; then
                export PATH="/usr/local/opt/llvm/bin:$PATH"
            fi
        fi

    else
        # Linux
        if command -v apt-get >/dev/null 2>&1; then
            # Debian / Ubuntu
            TO_INSTALL=""
            [ "$MISSING_GIT" = "true" ] && TO_INSTALL="$TO_INSTALL git"
            [ "$GO_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL golang"
            [ "$LLVM_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL llvm clang"
            [ "$MAKE_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL make"
            [ "$OPENSSL_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL libssl-dev"

            if [ -n "$TO_INSTALL" ]; then
                info "Running apt-get update & install..."
                $SUDO apt-get update -qq
                $SUDO apt-get install -y $TO_INSTALL
                ok "apt-get installation complete."
            fi

        elif command -v dnf >/dev/null 2>&1; then
            # Fedora / RHEL / CentOS
            TO_INSTALL=""
            [ "$MISSING_GIT" = "true" ] && TO_INSTALL="$TO_INSTALL git"
            [ "$GO_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL golang"
            [ "$LLVM_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL llvm clang"
            [ "$MAKE_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL make"
            [ "$OPENSSL_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL openssl-devel"

            if [ -n "$TO_INSTALL" ]; then
                info "Running dnf install..."
                $SUDO dnf install -y $TO_INSTALL
                ok "dnf installation complete."
            fi

        elif command -v pacman >/dev/null 2>&1; then
            # Arch Linux
            TO_INSTALL=""
            [ "$MISSING_GIT" = "true" ] && TO_INSTALL="$TO_INSTALL git"
            [ "$GO_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL go"
            [ "$LLVM_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL llvm clang"
            [ "$MAKE_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL make"
            [ "$OPENSSL_FOUND" = "false" ] && TO_INSTALL="$TO_INSTALL openssl"

            if [ -n "$TO_INSTALL" ]; then
                info "Running pacman install..."
                $SUDO pacman -S --noconfirm $TO_INSTALL
                ok "pacman installation complete."
            fi

        else
            warn "Unable to auto-detect package manager. Please install the missing tools manually:"
            [ "$MISSING_GIT" = "true" ] && warn "  - git"
            [ "$GO_FOUND" = "false" ] && warn "  - go (1.21+)"
            [ "$LLVM_FOUND" = "false" ] && warn "  - llvm and clang"
            [ "$MAKE_FOUND" = "false" ] && warn "  - make"
            [ "$OPENSSL_FOUND" = "false" ] && warn "  - openssl-devel (or libssl-dev)"
            exit 1
        fi
    fi
else
    ok "Toolchain is ready."
fi

# ── Build ──────────────────────────────────────────────────────────
if [ "$SKIP_BUILD" = "true" ]; then
    info "-SkipBuild set; not building."
else
    # Ensure Go modules are enabled
    export GO111MODULE=on

    if [ "$CLEAN_BUILD" = "true" ]; then
        info "Cleaning previous build..."
        if [ -n "$GO_BIN" ]; then
            make GO="$GO_BIN" clean
        else
            make clean
        fi
    fi

    info "Building Desi (runtime + tools)..."
    if [ -n "$GO_BIN" ]; then
        make GO="$GO_BIN"
    else
        make
    fi
    ok "Build complete: bin/desic"

    if [ "$TEST_SUITE" = "true" ]; then
        info "Running example test suite..."
        # Prepend Go binary's directory to PATH so test_examples.sh runs the correct Go version
        if [ -n "$GO_BIN" ]; then
            GO_DIR=$(dirname "$GO_BIN")
            export PATH="$GO_DIR:$PATH"
        fi
        if bash test_examples.sh; then
            ok "All example tests passed."
        else
            warn "Some example tests failed (see output above)."
        fi
    fi
fi

# ── Post-Setup Path Advice ──────────────────────────────────────────
# Check if llc is permanently in PATH
if ! command -v llc >/dev/null 2>&1; then
    # LLVM is installed but not in permanent PATH (highly common with macOS Homebrew)
    LLVM_PATH_ADVICE=""
    if [ "$OS" = "Darwin" ]; then
        if [ -d "/opt/homebrew/opt/llvm/bin" ]; then
            LLVM_PATH_ADVICE="/opt/homebrew/opt/llvm/bin"
        elif [ -d "/usr/local/opt/llvm/bin" ]; then
            LLVM_PATH_ADVICE="/usr/local/opt/llvm/bin"
        fi
    fi

    if [ -n "$LLVM_PATH_ADVICE" ]; then
        printf "\n${YELLOW}${BOLD}  Note: LLVM is installed, but needs to be added to your PATH.${RESET}\n"
        printf "  Please add the following line to your shell profile (e.g. ~/.zshrc or ~/.bashrc):\n"
        printf "    ${CYAN}export PATH=\"$LLVM_PATH_ADVICE:\$PATH\"${RESET}\n"
    fi
fi

printf "\n${GREEN}${BOLD}  Done! Build completed successfully.${RESET}\n"
printf "  Run compiled programs using:\n"
printf "    ${CYAN}./build-desi.sh examples/000_hello_world.desi${RESET}\n\n"
