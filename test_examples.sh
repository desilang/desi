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
#
# Test file markers:
#   # EXPECTED: COMPILE_ERROR  - Test should fail to compile
#   # EXPECTED: RUNTIME_ERROR  - Test should crash at runtime
#   # EXPECTED: SKIP           - Test is skipped (known issue)
#   # EXPECTED_OUTPUT:         - Expected stdout (lines starting with #)
#     # line1
#     # line2

set -e

BUILD_DIR="build/output"

# Per-test wall-clock limit, so a hanging program cannot stall the whole run.
# GNU timeout is `timeout` on Linux and `gtimeout` from coreutils on macOS; if
# neither is installed the tests run unbounded, exactly as before.
DESI_TEST_TIMEOUT_SECS="${DESI_TEST_TIMEOUT_SECS:-120}"
if command -v timeout >/dev/null 2>&1; then
    DESI_TEST_TIMEOUT="timeout ${DESI_TEST_TIMEOUT_SECS}"
elif command -v gtimeout >/dev/null 2>&1; then
    DESI_TEST_TIMEOUT="gtimeout ${DESI_TEST_TIMEOUT_SECS}"
else
    DESI_TEST_TIMEOUT=""
fi
mkdir -p "$BUILD_DIR"

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

# Function to extract expected output from test file
extract_expected_output() {
    local file="$1"
    local in_expected=false
    local output=""
    
    while IFS= read -r line; do
        if [[ "$line" == "# EXPECTED_OUTPUT:" ]]; then
            in_expected=true
            continue
        fi
        if [[ "$in_expected" == true ]]; then
            # Lines starting with # are expected output (strip the "# " prefix)
            if [[ "$line" =~ ^#\  ]]; then
                output+="${line:2}"$'\n'
            elif [[ "$line" == "#" ]]; then
                # Empty line in expected output
                output+=$'\n'
            else
                # End of expected output section
                break
            fi
        fi
    done < "$file"
    
    # Remove trailing newline for comparison
    echo -n "$output"
}

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
    
    # Tests marked "# REQUIRES: database" need a live PostgreSQL/MySQL, so they
    # are skipped unless DESI_DB_TESTS=1 says the servers are there. Without an
    # opt-in they were plain SKIPs, which meant nothing exercised the ORM: 495
    # green tests once coexisted with a save() that did not exist.
    if head -n 5 "$f" | grep -q "# REQUIRES: database"; then
        if [ "$DESI_DB_TESTS" != "1" ]; then
            echo "[$TOTAL_COUNT] Testing: $f  ⊘ SKIPPED (needs a database; set DESI_DB_TESTS=1)"
            PASSED_COUNT=$((PASSED_COUNT + 1))
            continue
        fi
    # Check if this test should be skipped
    elif head -n 5 "$f" | grep -q -E "(# EXPECTED: SKIP|# SKIPPED)"; then
        echo "[$TOTAL_COUNT] Testing: $f  ⊘ SKIPPED"
        PASSED_COUNT=$((PASSED_COUNT + 1))
        continue
    fi
    
    echo "[$TOTAL_COUNT] Testing: $f"
    
    # Check if this test is expected to fail
    EXPECTED_FAIL=$(head -n 1 "$f" | grep "# EXPECTED:" || true)
    
    # Check if this test has expected output
    HAS_EXPECTED_OUTPUT=$(grep -l "# EXPECTED_OUTPUT:" "$f" 2>/dev/null || true)
    
    # Build and run
    COMPILE_SUCCESS=false
    RUNTIME_SUCCESS=false
    TIMED_OUT=false
    
    COMPILE_LOG="build/output/compile_output.log"
    RUNTIME_LOG="build/output/runtime_output.log"
    
    if ./build-desi.sh "$f" "test_exec" > "$COMPILE_LOG" 2>&1; then
        COMPILE_SUCCESS=true
        # Bound each run. Without this one hanging program stalls the whole
        # suite indefinitely — 330_recursion_limit does exactly that on Linux,
        # where an uncaught recursion-depth panic fails to terminate the
        # process, and the suite simply stops making progress.
        set +e
        $DESI_TEST_TIMEOUT ./build/output/test_exec > "$RUNTIME_LOG" 2>&1
        RUN_RC=$?
        set -e
        if [ $RUN_RC -eq 0 ]; then
            RUNTIME_SUCCESS=true
        elif [ $RUN_RC -eq 124 ]; then
            # Killed by timeout. This must never satisfy EXPECTED: RUNTIME_ERROR
            # — a program that hangs has not produced the error the test wants,
            # and counting it as a pass is how a hang hides as a green tick.
            TIMED_OUT=true
            echo "TIMEOUT after ${DESI_TEST_TIMEOUT_SECS}s" >> "$RUNTIME_LOG"
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
            if [[ "$TIMED_OUT" == true ]]; then
                echo "  ❌ FAILED (timed out; expected a runtime error, not a hang)"
                FAILED_TESTS+=("$f (timeout)")
            elif [[ "$COMPILE_SUCCESS" == true && "$RUNTIME_SUCCESS" == false ]]; then
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
            # Check expected output if specified
            if [[ -n "$HAS_EXPECTED_OUTPUT" ]]; then
                EXPECTED=$(extract_expected_output "$f")
                ACTUAL=$(cat "$RUNTIME_LOG")
                
                if [[ "$EXPECTED" == "$ACTUAL" ]]; then
                    echo "  ✓ PASSED"
                    PASSED_COUNT=$((PASSED_COUNT + 1))
                else
                    echo "  ❌ FAILED (output mismatch)"
                    echo "    Expected:"
                    echo "$EXPECTED" | sed 's/^/      /'
                    echo "    Actual:"
                    echo "$ACTUAL" | sed 's/^/      /'
                    FAILED_TESTS+=("$f (output)")
                fi
            else
                # No expected output specified, just check it runs
                echo "  ✓ PASSED"
                PASSED_COUNT=$((PASSED_COUNT + 1))
            fi
        elif [[ "$COMPILE_SUCCESS" == false ]]; then
            echo "  ❌ FAILED (compile error)"
            cat "$COMPILE_LOG" | sed 's/^/    /' # Indent output
            FAILED_TESTS+=("$f (compile)")
        else
            echo "  ❌ FAILED (runtime error)"
            cat "$COMPILE_LOG" "$RUNTIME_LOG" 2>/dev/null | sed 's/^/    /' # Indent output
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
