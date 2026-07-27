#!/bin/bash
# Build a Desi program into an executable
# Usage: ./build-desi.sh [--release] <input.desi> [executable_name]
#   --release    optimized build (llc/clang -O2); also DESI_RELEASE=1

set -e

RELEASE=0
if [ "$1" = "--release" ] || [ "$1" = "-r" ]; then
    RELEASE=1
    shift
fi
if [ "$DESI_RELEASE" = "1" ]; then
    RELEASE=1
fi

if [ $# -lt 1 ]; then
    echo "Usage: $0 [--release] <input.desi> [executable_name]"
    exit 1
fi

INPUT="$1"
BASENAME=$(basename "$INPUT" .desi)
OUTPUT_NAME="${2:-$BASENAME}"

OPT_FLAGS=""
if [ "$RELEASE" = "1" ]; then
    OPT_FLAGS="-O2"
fi

# --- Auto-discover LLVM tools ---
find_tool() {
    local name="$1"
    # Check PATH first with common version suffixes
    for suffix in "" "-mp-21" "-mp-20" "-mp-19" "-mp-18" "-21" "-20" "-19" "-18"; do
        if command -v "${name}${suffix}" >/dev/null 2>&1; then
            echo "${name}${suffix}"
            return
        fi
    done
    # macOS: Homebrew (Apple Silicon, then Intel)
    for prefix in /opt/homebrew/opt/llvm/bin /usr/local/opt/llvm/bin /opt/homebrew/bin /usr/local/bin; do
        if [ -x "$prefix/$name" ]; then
            echo "$prefix/$name"
            return
        fi
    done
    # Linux: versioned LLVM installs
    for v in 20 19 18 17 16 15 14; do
        if [ -x "/usr/lib/llvm-$v/bin/$name" ]; then
            echo "/usr/lib/llvm-$v/bin/$name"
            return
        fi
    done
    # Fallback: bare name (will fail with clear error)
    echo "$name"
}

LLC=$(find_tool llc)
CLANG=$(find_tool clang)

# Ensure tools and library exist
if [ ! -f "bin/desic" ] || [ ! -f "build/libdesi.a" ]; then
    echo "Error: Compiler or runtime library not found."
    echo "Please run 'make' first to build the compiler and runtime."
    exit 1
fi

# Create build directories
mkdir -p build/output

echo "==> Compiling Desi to LLVM IR..."
./bin/desic emit-ir "$INPUT" > build/program.ll

echo "==> Compiling LLVM IR to object file..."
# clang compiles LLVM IR directly, so llc is only a fallback. It has to be:
# llc is not installed on GitHub's macOS or ubuntu runners (Apple's toolchain
# does not ship it, and Ubuntu only provides versioned binaries), which made
# every program fail with "llc: command not found" — find_tool falls back to
# the bare name when it cannot locate one. desic's own pipeline already calls
# clang on the .ll for this reason.
#
# llc also defaults to a non-PIC relocation model, and Linux links
# position-independent executables by default, so an llc-produced object fails
# with "relocation R_X86_64_32 ... can not be used when making a PIE object".
# clang gets this right on its own; the flag covers the fallback. macOS
# requires PIC regardless, so it is correct on both.
LLC_FLAGS="-relocation-model=pic"
if [ "$RELEASE" = "1" ]; then
    if ! $CLANG -w -O2 -c build/program.ll -o build/program.o 2>/dev/null; then
        OPT=$(find_tool opt)
        $OPT -O2 build/program.ll -o build/program_opt.bc
        $LLC -O2 $LLC_FLAGS build/program_opt.bc -filetype=obj -o build/program.o
        rm -f build/program_opt.bc
    fi
else
    if ! $CLANG -w -c build/program.ll -o build/program.o 2>/dev/null; then
        $LLC $LLC_FLAGS build/program.ll -filetype=obj -o build/program.o
    fi
fi

echo "==> Linking executable..."
EXTRA_LINK_FLAGS=""
# -dead_strip is an Apple ld flag. GNU ld rejects it outright
# ("unable to disambiguate: -dead_strip"), so passing it unconditionally made
# every single program fail to link on Linux. --gc-sections is the GNU
# equivalent; desic's own linker invocation already picks between them this way.
DEAD_STRIP_FLAG="-Wl,--gc-sections"
if [ "$(uname)" = "Darwin" ]; then
    MACOSX_VERSION=$(sw_vers -productVersion 2>/dev/null | cut -d. -f1-2 || echo "12.0")
    export MACOSX_DEPLOYMENT_TARGET="$MACOSX_VERSION"
    EXTRA_LINK_FLAGS="-mmacosx-version-min=$MACOSX_VERSION"
    DEAD_STRIP_FLAG="-Wl,-dead_strip"
fi
OPENSSL_LINK_FLAGS=""
if grep -qE "^import (http|db)" "$INPUT" 2>/dev/null; then
    # Add OpenSSL linker flags
    BREW_OPENSSL=""
    if [ -x "/opt/homebrew/bin/brew" ]; then
        BREW_OPENSSL=$(/opt/homebrew/bin/brew --prefix openssl 2>/dev/null || echo "")
    elif command -v brew >/dev/null 2>&1; then
        BREW_OPENSSL=$(brew --prefix openssl 2>/dev/null || echo "")
    fi

    if [ -n "$BREW_OPENSSL" ] && [ -d "$BREW_OPENSSL/lib" ]; then
        OPENSSL_LINK_FLAGS="-L$BREW_OPENSSL/lib -lssl -lcrypto"
    elif [ -d "/opt/homebrew/opt/openssl/lib" ]; then
        OPENSSL_LINK_FLAGS="-L/opt/homebrew/opt/openssl/lib -lssl -lcrypto"
    elif [ -d "/opt/homebrew/opt/openssl@3/lib" ]; then
        OPENSSL_LINK_FLAGS="-L/opt/homebrew/opt/openssl@3/lib -lssl -lcrypto"
    elif [ -d "/usr/local/opt/openssl/lib" ]; then
        OPENSSL_LINK_FLAGS="-L/usr/local/opt/openssl/lib -lssl -lcrypto"
    elif [ -d "/usr/local/opt/openssl@3/lib" ]; then
        OPENSSL_LINK_FLAGS="-L/usr/local/opt/openssl@3/lib -lssl -lcrypto"
    elif [ -d "/opt/homebrew/lib" ]; then
        OPENSSL_LINK_FLAGS="-L/opt/homebrew/lib -lssl -lcrypto"
    elif [ -d "/usr/local/lib" ]; then
        # Check if ssl libraries actually exist in /usr/local/lib to prevent false positive
        if [ -f "/usr/local/lib/libssl.dylib" ] || [ -f "/usr/local/lib/libssl.a" ]; then
            OPENSSL_LINK_FLAGS="-L/usr/local/lib -lssl -lcrypto"
        else
            OPENSSL_LINK_FLAGS="-lssl -lcrypto"
        fi
    else
        OPENSSL_LINK_FLAGS="-lssl -lcrypto"
    fi
fi
# Link against libdesi.a (static runtime) with dead code elimination
# Note: -lz removed — compression is now bundled via miniz
# -lm is required on Linux: the runtime calls round()/pow() from builtins.c, and
# glibc keeps libm separate. macOS folds libm into libSystem, so its absence
# went unnoticed there while every Linux link failed with
# "undefined reference to `round'". Passing it on macOS is harmless.
$CLANG $OPT_FLAGS build/program.o -Lbuild -ldesi $EXTRA_LINK_FLAGS $OPENSSL_LINK_FLAGS -lm -o "build/output/$OUTPUT_NAME" $DEAD_STRIP_FLAG

echo "==> Cleaning up intermediate files..."
rm -f build/program.ll build/program.o

echo "✓ Built executable: build/output/$OUTPUT_NAME"
echo "  Run with: ./build/output/$OUTPUT_NAME"
