#!/bin/bash
# Build a Desi program into an executable
# Usage: ./build-desi.sh <input.desi> [executable_name]

set -e

if [ $# -lt 1 ]; then
    echo "Usage: $0 <input.desi> [executable_name]"
    exit 1
fi

INPUT="$1"
BASENAME=$(basename "$INPUT" .desi)
OUTPUT_NAME="${2:-$BASENAME}"

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
llc build/program.ll -filetype=obj -o build/program.o

echo "==> Linking executable..."
# Detect if the source uses HTTP or DB module (needs OpenSSL)
EXTRA_LINK_FLAGS=""
if grep -qE "^import (http|db)" "$INPUT" 2>/dev/null; then
    # Add OpenSSL linker flags
    if [ -d "/opt/homebrew/lib" ]; then
        EXTRA_LINK_FLAGS="-L/opt/homebrew/lib -lssl -lcrypto"
    elif [ -d "/usr/local/lib" ]; then
        EXTRA_LINK_FLAGS="-L/usr/local/lib -lssl -lcrypto"
    else
        EXTRA_LINK_FLAGS="-lssl -lcrypto"
    fi
fi
# Link against libdesi.a (static runtime) with dead code elimination
# Note: -lz removed — compression is now bundled via miniz
clang build/program.o -Lbuild -ldesi $EXTRA_LINK_FLAGS -o "build/output/$OUTPUT_NAME" -Wl,-dead_strip

echo "==> Cleaning up intermediate files..."
rm -f build/program.ll build/program.o

echo "✓ Built executable: build/output/$OUTPUT_NAME"
echo "  Run with: ./build/output/$OUTPUT_NAME"
