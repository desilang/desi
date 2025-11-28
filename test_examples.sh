#!/bin/bash
# Test all working example files (26+) to ensure no regressions
# Usage: ./test_examples.sh [start_number]
# Example: ./test_examples.sh 26

set -e

START_NUM=${1:-26}
FAILED_TESTS=()
PASSED_COUNT=0
TOTAL_COUNT=0

echo "=========================================="
echo "Running Desi Example Tests (from $START_NUM)"
echo "=========================================="
echo ""

# Build compiler first
echo "Building compiler..."
go build -o bin/desic ./compiler/cmd/desic
if [ $? -ne 0 ]; then
    echo "❌ Compiler build failed!"
    exit 1
fi
echo "✓ Compiler built successfully"
echo ""

# Run tests
for f in examples/[2-9][0-9]_*.desi; do
    if [ ! -f "$f" ]; then
        continue
    fi
    
    # Extract number from filename
    num=$(basename "$f" | cut -d_ -f1)
    
    # Skip if below start number
    if [[ $num -lt $START_NUM ]]; then
        continue
    fi
    
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    echo "[$TOTAL_COUNT] Testing: $f"
    
    # Build and run
    if ./build-desi.sh "$f" "test_exec" > /dev/null 2>&1; then
        if ./build/output/test_exec > /dev/null 2>&1; then
            echo "  ✓ PASSED"
            PASSED_COUNT=$((PASSED_COUNT + 1))
        else
            echo "  ❌ FAILED (runtime error)"
            FAILED_TESTS+=("$f (runtime)")
        fi
    else
        echo "  ❌ FAILED (compile error)"
        FAILED_TESTS+=("$f (compile)")
    fi
    echo ""
done

# Summary
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total:  $TOTAL_COUNT"
echo "Passed: $PASSED_COUNT"
echo "Failed: $((TOTAL_COUNT - PASSED_COUNT))"
echo ""

if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo "Failed tests:"
    for test in "${FAILED_TESTS[@]}"; do
        echo "  - $test"
    done
    exit 1
else
    echo "✓ All tests passed!"
    exit 0
fi
