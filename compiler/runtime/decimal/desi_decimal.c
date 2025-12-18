/*
 * Desi Decimal Wrapper - Simplified API for libmpdec
 * Provides easy-to-use functions for decimal arithmetic in Desi language.
 */

#include "include/mpdecimal.h"
#include <string.h>

// Global decimal context (default precision for financial calculations)
static mpd_context_t desi_ctx;
static int ctx_initialized = 0;

static void ensure_context(void) {
    if (!ctx_initialized) {
        mpd_defaultcontext(&desi_ctx);
        mpd_qsetprec(&desi_ctx, 28);  // Python's default precision
        mpd_qsetround(&desi_ctx, MPD_ROUND_HALF_EVEN);  // Banker's rounding
        ctx_initialized = 1;
    }
}

// Create a new decimal from a string like "19.99"
mpd_t* __decimal_new(const char* str) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_set_string(result, str, &desi_ctx);
    }
    return result;
}

// Create a new decimal from an integer
mpd_t* __decimal_from_int(int64_t value) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_set_i64(result, value, &desi_ctx);
    }
    return result;
}

// Add two decimals: a + b
mpd_t* __decimal_add(mpd_t* a, mpd_t* b) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_add(result, a, b, &desi_ctx);
    }
    return result;
}

// Subtract two decimals: a - b
mpd_t* __decimal_sub(mpd_t* a, mpd_t* b) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_sub(result, a, b, &desi_ctx);
    }
    return result;
}

// Multiply two decimals: a * b
mpd_t* __decimal_mul(mpd_t* a, mpd_t* b) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_mul(result, a, b, &desi_ctx);
    }
    return result;
}

// Divide two decimals: a / b
mpd_t* __decimal_div(mpd_t* a, mpd_t* b) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_div(result, a, b, &desi_ctx);
    }
    return result;
}

// Compare two decimals: returns -1, 0, or 1
int __decimal_cmp(mpd_t* a, mpd_t* b) {
    ensure_context();
    return mpd_cmp(a, b, &desi_ctx);
}

// Convert decimal to string (caller must free)
char* __decimal_to_str(mpd_t* d) {
    return mpd_to_sci(d, 0);
}

// Free a decimal
void __decimal_free(mpd_t* d) {
    if (d) {
        mpd_del(d);
    }
}

// Negate a decimal
mpd_t* __decimal_neg(mpd_t* a) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_minus(result, a, &desi_ctx);
    }
    return result;
}

// Absolute value
mpd_t* __decimal_abs(mpd_t* a) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (result) {
        mpd_abs(result, a, &desi_ctx);
    }
    return result;
}
