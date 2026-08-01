/*
 * Desi Decimal Wrapper - Simplified API for libmpdec
 * Provides easy-to-use functions for decimal arithmetic in Desi language.
 */

#include "include/mpdecimal.h"
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include "../exception.h"

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
//
// Invalid text used to abort the process with no message: the default context
// traps on a conversion error, so decimal(user_input) — which is the whole
// reason the string form exists — killed the program on the first typo. The
// conversion is done with the quiet form and a bad parse is reported as an
// ordinary Desi RuntimeError instead.
mpd_t* __decimal_new(const char* str) {
    ensure_context();
    mpd_t* result = mpd_new(&desi_ctx);
    if (!result) return NULL;

    uint32_t status = 0;
    mpd_qset_string(result, str ? str : "", &desi_ctx, &status);
    if (status & MPD_Conversion_syntax) {
        char msg[160];
        snprintf(msg, sizeof(msg), "cannot make a decimal from %s",
                 str ? str : "an empty string");
        mpd_del(result);
        __desi_raise(DESI_EXC_RUNTIME_ERROR, msg, "RuntimeError");
        return NULL;
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

/* Convert a decimal to an int, truncating toward zero.
 * Rounding is deliberate and documented: int(2.9d) is 2, matching int(2.9). */
int64_t __decimal_to_int(mpd_t* d) {
    if (!d) return 0;
    ensure_context();
    mpd_t* truncated = mpd_new(&desi_ctx);
    mpd_trunc(truncated, d, &desi_ctx);
    char* s = mpd_to_sci(truncated, 0);
    int64_t out = 0;
    if (s) {
        out = strtoll(s, NULL, 10);
        mpd_free(s);
    }
    mpd_del(truncated);
    return out;
}

/* Convert a decimal to a double. Goes through the string form so the value is
 * the one the decimal actually holds, rather than an intermediate binary
 * approximation — precision beyond a double's range is lost either way. */
double __decimal_to_float(mpd_t* d) {
    if (!d) return 0.0;
    char* s = mpd_to_sci(d, 0);
    double out = 0.0;
    if (s) {
        out = strtod(s, NULL);
        mpd_free(s);
    }
    return out;
}

/* Build a decimal from a double, via the shortest representation that
 * round-trips, so decimal(0.1) is 0.1 and not 0.1000000000000000055511151231. */
mpd_t* __decimal_from_float(double value) {
    char buf[64];
    snprintf(buf, sizeof(buf), "%.17g", value);
    /* %.17g always round-trips; trim it back to the shortest form that does. */
    for (int prec = 1; prec < 17; prec++) {
        char shorter[64];
        snprintf(shorter, sizeof(shorter), "%.*g", prec, value);
        if (strtod(shorter, NULL) == value) {
            return __decimal_new(shorter);
        }
    }
    return __decimal_new(buf);
}

/* Truthiness: zero is false, everything else true — same rule Python uses for
 * Decimal, and the same rule int and float already follow here. */
int32_t __decimal_to_bool(mpd_t* d) {
    if (!d) return 0;
    return mpd_iszero(d) ? 0 : 1;
}
