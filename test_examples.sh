#!/bin/bash
# Test all .desi example files
# Usage: ./test_examples.sh [start] or ./test_examples.sh [start,end]
#   Examples:
#     ./test_examples.sh        # Run all tests
#     ./test_examples.sh 26     # Run tests from 26 to end
#     ./test_examples.sh 0,25   # Run tests from 0 to 25 (inclusive)
# Exit codes:
#   0 - All tests passed
#   1 - Some tests failed

set -e

BUILD_DIR="build/output"
mkdir -p "$BUILD_DIR"

# Expected failure tests: Add "# EXPECTED: COMPILE_ERROR" or "# EXPECTED: RUNTIME_ERROR"
# as the first line of the test file

# Parse arguments - support comma syntax for range
if [[ -z "$1" ]]; then
    START_NUM=0
    END_NUM=9999
elif [[ "$1" == *","* ]]; then
    # Comma syntax: 0,25
    START_NUM=$(echo "$1" | cut -d',' -f1)
    END_NUM=$(echo "$1" | cut -d',' -f2)
else
    # Single number: start from there, go to end
    START_NUM=$1
    END_NUM=${2:-9999}
fi

FAILED_TESTS=()
PASSED_COUNT=0
TOTAL_COUNT=0

echo "=========================================="
echo "Running Desi Example Tests ($START_NUM to $END_NUM)"
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
    
    # Extract number from filename (remove leading zeros)
    num=$(basename "$f" | grep -o '^[0-9]*' | sed 's/^0*//')
    if [[ -z "$num" ]]; then num=0; fi
    
    # Skip if below start number or above end number
    if [[ $num -lt $START_NUM ]]; then
        continue
    fi
    if [[ $num -gt $END_NUM ]]; then
        continue
    fi
    
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    echo "[$TOTAL_COUNT] Testing: $f"
    
    # Check if this test is expected to fail
    EXPECTED_FAIL=$(head -n 1 "$f" | grep "# EXPECTED:" || true)
    
    # Build and run
    COMPILE_SUCCESS=false
    RUNTIME_SUCCESS=false
    
    OUTPUT_LOG="build/output/test_output.log"
    if ./build-desi.sh "$f" "test_exec" > "$OUTPUT_LOG" 2>&1; then
        COMPILE_SUCCESS=true
        if ./build/output/test_exec >> "$OUTPUT_LOG" 2>&1; then
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
            cat "$OUTPUT_LOG" | sed 's/^/    /' # Indent output
            FAILED_TESTS+=("$f (compile)")
        else
            echo "  ❌ FAILED (runtime error)"
            cat "$OUTPUT_LOG" | sed 's/^/    /' # Indent output
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
