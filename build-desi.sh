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

# Create build directories
mkdir -p build/output

echo "==> Compiling Desi to LLVM IR..."
./gen/desic -emit-ir "$INPUT" > build/program.ll

echo "==> Compiling C runtime..."
clang -c compiler/runtime/set.c -o build/set.o
clang -c compiler/runtime/dict.c -o build/dict.o
clang -c compiler/runtime/print.c -o build/print.o

echo "==> Compiling LLVM IR to object file..."
llc build/program.ll -filetype=obj -o build/program.o

echo "==> Linking executable..."
clang build/program.o build/set.o build/dict.o build/print.o -o "build/output/$OUTPUT_NAME"

echo "==> Cleaning up intermediate files..."
rm -f build/program.ll build/program.o

echo "✓ Built executable: build/output/$OUTPUT_NAME"
echo "  Run with: ./build/output/$OUTPUT_NAME"
