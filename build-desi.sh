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
# Link against libdesi.a (static runtime)
clang build/program.o -Lbuild -ldesi -o "build/output/$OUTPUT_NAME"

echo "==> Cleaning up intermediate files..."
rm -f build/program.ll build/program.o

echo "✓ Built executable: build/output/$OUTPUT_NAME"
echo "  Run with: ./build/output/$OUTPUT_NAME"
