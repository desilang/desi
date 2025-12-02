#!/bin/bash
# Test all .desi example files
# Exit codes:
#   0 - All tests passed
#   1 - Some tests failed

set -e

BUILD_DIR="build/output"
mkdir -p "$BUILD_DIR"

# Expected failure tests: Add "# EXPECTED: COMPILE_ERROR" or "# EXPECTED: RUNTIME_ERROR"
# as the first line of the test file

START_NUM=${1:-0}
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

# Run tests - find all files matching [0-9]*.desi pattern and sort numerically
for f in $(find examples -name '[0-9]*.desi' | sort -V); do
    if [ ! -f "$f" ]; then
        continue
    fi
    
    # Extract number from filename
    num=$(basename "$f" | grep -o '^[0-9]*')
    
    # Skip if below start number
    if [[ $num -lt $START_NUM ]]; then
        continue
    fi
    
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    echo "[$TOTAL_COUNT] Testing: $f"
    
    # Check if this test is expected to fail
    EXPECTED_FAIL=$(head -n 1 "$f" | grep "# EXPECTED:" || true)
    
    # Build and run
    COMPILE_SUCCESS=false
    RUNTIME_SUCCESS=false
    
    if ./build-desi.sh "$f" "test_exec" > /dev/null 2>&1; then
        COMPILE_SUCCESS=true
        if ./build/output/test_exec > /dev/null 2>&1; then
            RUNTIME_SUCCESS=true
        fi
    fi
    
    # Determine if test passed based on expectations
    if [[ -n "$EXPECTED_FAIL" ]]; then
        # Expected to fail
        if echo "$EXPECTED_FAIL" | grep -q "COMPILE_ERROR"; then
            if [[ "$COMPILE_SUCCESS" == false ]]; then
                echo "  ✓ PASSED (expected compile error)"
                PASSED_COUNT=$((PASSED_COUNT + 1))
            else
                echo "  ❌ FAILED (expected compile error, but compiled successfully)"
                FAILED_TESTS+=("$f (unexpected success)")
            fi
        elif echo "$EXPECTED_FAIL" | grep -q "RUNTIME_ERROR"; then
            if [[ "$COMPILE_SUCCESS" == true && "$RUNTIME_SUCCESS" == false ]]; then
                echo "  ✓ PASSED (expected runtime error)"
                PASSED_COUNT=$((PASSED_COUNT + 1))
            else
                echo "  ❌ FAILED (expected runtime error, but ran successfully)"
                FAILED_TESTS+=("$f (unexpected success)")
            fi
        fi
    else
        # Expected to pass
        if [[ "$COMPILE_SUCCESS" == true && "$RUNTIME_SUCCESS" == true ]]; then
            echo "  ✓ PASSED"
            PASSED_COUNT=$((PASSED_COUNT + 1))
        elif [[ "$COMPILE_SUCCESS" == false ]]; then
            echo "  ❌ FAILED (compile error)"
            FAILED_TESTS+=("$f (compile)")
        else
            echo "  ❌ FAILED (runtime error)"
            FAILED_TESTS+=("$f (runtime)")
        fi
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
