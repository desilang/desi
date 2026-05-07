/*
 * testing.c — Test assertion framework for Desi stdlib
 *
 * Provides assertion functions for desic test command.
 * On failure, prints diagnostic with file/line info and aborts.
 *
 * Public API:
 *   __testing_assert_eq(a, b, msg)         → assert a == b
 *   __testing_assert_ne(a, b, msg)         → assert a != b
 *   __testing_assert_true(val, msg)        → assert val is true
 *   __testing_assert_false(val, msg)       → assert val is false
 *   __testing_assert_gt(a, b, msg)         → assert a > b
 *   __testing_assert_lt(a, b, msg)         → assert a < b
 *   __testing_assert_gte(a, b, msg)        → assert a >= b
 *   __testing_assert_lte(a, b, msg)        → assert a <= b
 *   __testing_assert_contains(haystack, needle, msg)
 *   __testing_assert_not_contains(haystack, needle, msg)
 *   __testing_assert_starts_with(str, prefix, msg)
 *   __testing_assert_ends_with(str, suffix, msg)
 *   __testing_assert_nil(ptr, msg)         → assert ptr is NULL
 *   __testing_assert_not_nil(ptr, msg)     → assert ptr is not NULL
 *   __testing_assert_close(a, b, tol, msg) → assert |a-b| < tol
 *   __testing_fail(msg)                    → unconditional failure
 *   __testing_skip(msg)                    → skip test with message
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <stdint.h>

/* ---- Counters ---- */
static int _test_pass_count = 0;
static int _test_fail_count = 0;
static int _test_skip_count = 0;

/* ---- Failure handler ---- */
static void test_fail(const char* assertion, const char* detail, const char* msg) {
    _test_fail_count++;
    fprintf(stderr, "\033[31m  FAIL\033[0m: %s", assertion);
    if (detail && detail[0])
        fprintf(stderr, " (%s)", detail);
    if (msg && msg[0])
        fprintf(stderr, " — %s", msg);
    fprintf(stderr, "\n");
}

static void test_pass(void) {
    _test_pass_count++;
}

/* ============================================================
 * Integer assertions
 * ============================================================ */

void __testing_assert_eq_int(int64_t a, int64_t b, const char* msg) {
    if (a == b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "expected %lld, got %lld", (long long)b, (long long)a);
    test_fail("assert_eq", detail, msg ? msg : "");
}

void __testing_assert_ne_int(int64_t a, int64_t b, const char* msg) {
    if (a != b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "values should differ, both are %lld", (long long)a);
    test_fail("assert_ne", detail, msg ? msg : "");
}

void __testing_assert_gt(int64_t a, int64_t b, const char* msg) {
    if (a > b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "%lld not > %lld", (long long)a, (long long)b);
    test_fail("assert_gt", detail, msg ? msg : "");
}

void __testing_assert_lt(int64_t a, int64_t b, const char* msg) {
    if (a < b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "%lld not < %lld", (long long)a, (long long)b);
    test_fail("assert_lt", detail, msg ? msg : "");
}

void __testing_assert_gte(int64_t a, int64_t b, const char* msg) {
    if (a >= b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "%lld not >= %lld", (long long)a, (long long)b);
    test_fail("assert_gte", detail, msg ? msg : "");
}

void __testing_assert_lte(int64_t a, int64_t b, const char* msg) {
    if (a <= b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "%lld not <= %lld", (long long)a, (long long)b);
    test_fail("assert_lte", detail, msg ? msg : "");
}

/* ============================================================
 * Boolean assertions
 * ============================================================ */

void __testing_assert_true(int val, const char* msg) {
    if (val) { test_pass(); return; }
    test_fail("assert_true", "got false", msg ? msg : "");
}

void __testing_assert_false(int val, const char* msg) {
    if (!val) { test_pass(); return; }
    test_fail("assert_false", "got true", msg ? msg : "");
}

/* ============================================================
 * Float assertions
 * ============================================================ */

void __testing_assert_eq_float(double a, double b, const char* msg) {
    if (a == b) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "expected %g, got %g", b, a);
    test_fail("assert_eq", detail, msg ? msg : "");
}

void __testing_assert_close(double a, double b, double tolerance, const char* msg) {
    if (fabs(a - b) < tolerance) { test_pass(); return; }
    char detail[128];
    snprintf(detail, sizeof(detail), "|%g - %g| = %g >= tolerance %g", a, b, fabs(a-b), tolerance);
    test_fail("assert_close", detail, msg ? msg : "");
}

/* ============================================================
 * String assertions
 * ============================================================ */

void __testing_assert_eq_str(const char* a, const char* b, const char* msg) {
    if (a && b && strcmp(a, b) == 0) { test_pass(); return; }
    if (!a && !b) { test_pass(); return; }
    char detail[256];
    snprintf(detail, sizeof(detail), "expected \"%s\", got \"%s\"", b ? b : "(null)", a ? a : "(null)");
    test_fail("assert_eq", detail, msg ? msg : "");
}

void __testing_assert_ne_str(const char* a, const char* b, const char* msg) {
    if ((!a && b) || (a && !b) || (a && b && strcmp(a, b) != 0)) { test_pass(); return; }
    test_fail("assert_ne", "strings should differ", msg ? msg : "");
}

void __testing_assert_contains(const char* haystack, const char* needle, const char* msg) {
    if (haystack && needle && strstr(haystack, needle)) { test_pass(); return; }
    char detail[256];
    snprintf(detail, sizeof(detail), "\"%s\" not found in \"%s\"",
             needle ? needle : "(null)", haystack ? haystack : "(null)");
    test_fail("assert_contains", detail, msg ? msg : "");
}

void __testing_assert_not_contains(const char* haystack, const char* needle, const char* msg) {
    if (!haystack || !needle || !strstr(haystack, needle)) { test_pass(); return; }
    char detail[256];
    snprintf(detail, sizeof(detail), "\"%s\" found in \"%s\"", needle, haystack);
    test_fail("assert_not_contains", detail, msg ? msg : "");
}

void __testing_assert_starts_with(const char* str, const char* prefix, const char* msg) {
    if (str && prefix && strncmp(str, prefix, strlen(prefix)) == 0) { test_pass(); return; }
    char detail[256];
    snprintf(detail, sizeof(detail), "\"%s\" does not start with \"%s\"",
             str ? str : "(null)", prefix ? prefix : "(null)");
    test_fail("assert_starts_with", detail, msg ? msg : "");
}

void __testing_assert_ends_with(const char* str, const char* suffix, const char* msg) {
    if (str && suffix) {
        size_t slen = strlen(str), xlen = strlen(suffix);
        if (slen >= xlen && strcmp(str + slen - xlen, suffix) == 0) { test_pass(); return; }
    }
    char detail[256];
    snprintf(detail, sizeof(detail), "\"%s\" does not end with \"%s\"",
             str ? str : "(null)", suffix ? suffix : "(null)");
    test_fail("assert_ends_with", detail, msg ? msg : "");
}

/* ============================================================
 * Pointer / nil assertions
 * ============================================================ */

void __testing_assert_nil(void* ptr, const char* msg) {
    if (!ptr) { test_pass(); return; }
    test_fail("assert_nil", "expected nil, got non-nil", msg ? msg : "");
}

void __testing_assert_not_nil(void* ptr, const char* msg) {
    if (ptr) { test_pass(); return; }
    test_fail("assert_not_nil", "expected non-nil, got nil", msg ? msg : "");
}

/* ============================================================
 * Control flow
 * ============================================================ */

void __testing_fail(const char* msg) {
    test_fail("fail", "", msg ? msg : "explicit failure");
}

void __testing_skip(const char* msg) {
    _test_skip_count++;
    fprintf(stderr, "\033[33m  SKIP\033[0m: %s\n", msg ? msg : "");
}

/* ============================================================
 * Summary (called at end of test run)
 * ============================================================ */

int32_t __testing_summary(void) {
    fprintf(stderr, "\n");
    if (_test_fail_count == 0) {
        fprintf(stderr, "\033[32m✓ %d assertion(s) passed\033[0m", _test_pass_count);
    } else {
        fprintf(stderr, "\033[31m✗ %d passed, %d failed\033[0m", _test_pass_count, _test_fail_count);
    }
    if (_test_skip_count > 0) {
        fprintf(stderr, ", %d skipped", _test_skip_count);
    }
    fprintf(stderr, "\n");
    return _test_fail_count > 0 ? 1 : 0;
}

/* Reset counters (for multiple test files in same process) */
void __testing_reset(void) {
    _test_pass_count = 0;
    _test_fail_count = 0;
    _test_skip_count = 0;
}

int32_t __testing_pass_count(void) { return _test_pass_count; }
int32_t __testing_fail_count(void) { return _test_fail_count; }
