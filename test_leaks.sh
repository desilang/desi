#!/bin/bash
# Memory leak verification for Desi programs
# Uses macOS 'leaks' tool (Valgrind equivalent for ARM)

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <test.desi>"
    exit 1
fi

DESI_FILE="$1"
EXEC_NAME="leak_test"

echo "=========================================="
echo "Memory Leak Test: $(basename $DESI_FILE)"
echo "=========================================="
echo ""

# Build the executable
echo "Building..."
./build-desi.sh "$DESI_FILE" "$EXEC_NAME" > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "❌ Build failed"
    exit 1
fi
echo "✓ Build successful"
echo ""

# Run with leaks detection
echo "Running leak detection..."
leaks --atExit -- ./build/output/$EXEC_NAME > /tmp/leak_output.txt 2>&1

# Check results
if grep -q "0 leaks for 0 total leaked bytes" /tmp/leak_output.txt; then
    echo "✅ NO LEAKS DETECTED"
    exit 0
else
    echo "❌ MEMORY LEAKS DETECTED"
    echo ""
    cat /tmp/leak_output.txt
    exit 1
fi
