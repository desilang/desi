// builtins.c - Standard builtin functions for Desi
// These are common utility functions available in the prelude

#include <stdint.h>
#include <stdbool.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "list.h"
#include "exception.h"

// ==================== Type[T] Runtime Support ====================
// DesiTypeInfo represents runtime type information for Type[T]

typedef struct DesiTypeInfo {
    uint64_t id;           // Unique type ID (hash of type name)
    const char* name;      // Type name string (e.g., "int", "list[str]")
    size_t size;           // Size in bytes (0 for unsized types)
} DesiTypeInfo;

// Create a type info object (called from lowering for type_of)
DesiTypeInfo* __desi_type_new(uint64_t id, const char* name, size_t size) {
    DesiTypeInfo* t = malloc(sizeof(DesiTypeInfo));
    if (!t) {
        fprintf(stderr, "panic: failed to allocate type info\n");
        exit(1);
    }
    t->id = id;
    t->name = name;  // Static string, no copy needed
    t->size = size;
    return t;
}

// Get the type name as a string
const char* __desi_type_name(DesiTypeInfo* t) {
    return t ? t->name : "unknown";
}

// Get the type size
size_t __desi_type_size(DesiTypeInfo* t) {
    return t ? t->size : 0;
}

// Compare two types for equality
bool __desi_type_equal(DesiTypeInfo* a, DesiTypeInfo* b) {
    if (!a || !b) return false;
    return a->id == b->id;
}


// Panic function for integer division by zero
// Raises ZeroDivisionError — catchable via try/except
void __panic_divzero(void) {
    __desi_raise(DESI_EXC_ZERO_DIVISION, "integer division by zero", "ZeroDivisionError");
}

// Panic function for unwrapping None (Option.Nothing)
// Raises RuntimeError — catchable via try/except
void __panic_unwrap_none(void) {
    __desi_raise(DESI_EXC_RUNTIME_ERROR, "called unwrap() on a None value", "RuntimeError");
}

// Panic function for unwrapping Err (Result.Err)
// Raises RuntimeError — catchable via try/except
void __panic_unwrap_err(void) {
    __desi_raise(DESI_EXC_RUNTIME_ERROR, "called unwrap() on an Err value", "RuntimeError");
}

// Panic function for unwrap_err on Ok (Result.Ok)
// Raises RuntimeError — catchable via try/except
void __panic_unwrap_ok(void) {
    __desi_raise(DESI_EXC_RUNTIME_ERROR, "called unwrap_err() on an Ok value", "RuntimeError");
}

// Panic function for expect() with custom message
// Raises RuntimeError — catchable via try/except
void __panic_expect(const char* msg) {
    __desi_raise(DESI_EXC_RUNTIME_ERROR, msg ? msg : "expect() failed", "RuntimeError");
}

// Panic with user-provided message (legacy 'raise' fallback)
// Raises base Exception — catchable via try/except
void __desi_panic(const char* msg) {
    __desi_raise(DESI_EXC_EXCEPTION, msg ? msg : "(null)", "Exception");
}

// Assert failure handler
void __desi_assert_fail(const char* msg) {
    fprintf(stderr, "assertion failed: %s\n", msg);
    exit(1);
}

// Assert equality failure: shows expected vs actual
void __desi_assert_eq_fail(const char* expected, const char* actual, const char* context) {
    fprintf(stderr, "assertion failed: %s\n", context);
    fprintf(stderr, "  expected: %s\n", expected ? expected : "(null)");
    fprintf(stderr, "    actual: %s\n", actual ? actual : "(null)");
    exit(1);
}

// Assert inequality failure: shows the duplicate value
void __desi_assert_ne_fail(const char* value, const char* context) {
    fprintf(stderr, "assertion failed: %s\n", context);
    fprintf(stderr, "  values should differ but both are: %s\n", value ? value : "(null)");
    exit(1);
}

// Assert equality check for integers
void __desi_assert_eq_int(int32_t expected, int32_t actual, const char* context) {
    if (expected != actual) {
        char exp_buf[32], act_buf[32];
        snprintf(exp_buf, sizeof(exp_buf), "%d", expected);
        snprintf(act_buf, sizeof(act_buf), "%d", actual);
        __desi_assert_eq_fail(exp_buf, act_buf, context);
    }
}

// Assert inequality check for integers
void __desi_assert_ne_int(int32_t a, int32_t b, const char* context) {
    if (a == b) {
        char buf[32];
        snprintf(buf, sizeof(buf), "%d", a);
        __desi_assert_ne_fail(buf, context);
    }
}

// Assert equality check for strings
void __desi_assert_eq_str(const char* expected, const char* actual, const char* context) {
    if (!expected) expected = "";
    if (!actual) actual = "";
    if (strcmp(expected, actual) != 0) {
        __desi_assert_eq_fail(expected, actual, context);
    }
}

// Assert inequality check for strings
void __desi_assert_ne_str(const char* a, const char* b, const char* context) {
    if (!a) a = "";
    if (!b) b = "";
    if (strcmp(a, b) == 0) {
        __desi_assert_ne_fail(a, context);
    }
}

// Assert equality check for booleans
void __desi_assert_eq_bool(int expected, int actual, const char* context) {
    if (expected != actual) {
        __desi_assert_eq_fail(expected ? "true" : "false", actual ? "true" : "false", context);
    }
}

// Assert inequality check for booleans
void __desi_assert_ne_bool(int a, int b, const char* context) {
    if (a == b) {
        __desi_assert_ne_fail(a ? "true" : "false", context);
    }
}

// Math helper functions for float special values
#include <math.h>

// is_nan - check if float is NaN
bool __math_is_nan(double x) {
    return isnan(x) ? 1 : 0;
}

// is_inf - check if float is infinity (positive or negative)
bool __math_is_inf(double x) {
    return isinf(x) ? 1 : 0;
}

// is_finite - check if float is a normal number (not NaN or infinity)
bool __math_is_finite(double x) {
    return isfinite(x) ? 1 : 0;
}

// sum - sum of all elements in a list of integers
int64_t list_sum_int(DesiList* l) {
    if (!l) return 0;
    int64_t total = 0;
    for (size_t i = 0; i < l->length; i++) {
        // Elements stored as pointers, cast to i64
        int64_t val = (int64_t)(intptr_t)l->data[i];
        total += val;
    }
    return total;
}

// min - minimum value in a list of integers
int64_t list_min_int(DesiList* l) {
    if (!l || l->length == 0) return 0;
    int64_t min_val = (int64_t)(intptr_t)l->data[0];
    for (size_t i = 1; i < l->length; i++) {
        int64_t val = (int64_t)(intptr_t)l->data[i];
        if (val < min_val) min_val = val;
    }
    return min_val;
}

// max - maximum value in a list of integers
int64_t list_max_int(DesiList* l) {
    if (!l || l->length == 0) return 0;
    int64_t max_val = (int64_t)(intptr_t)l->data[0];
    for (size_t i = 1; i < l->length; i++) {
        int64_t val = (int64_t)(intptr_t)l->data[i];
        if (val > max_val) max_val = val;
    }
    return max_val;
}

// any - returns true if any element is truthy (for bool list)
bool list_any_builtin(DesiList* l) {
    if (!l) return false;
    for (size_t i = 0; i < l->length; i++) {
        // For bool list, elements are 0 or 1 stored as pointers
        if ((intptr_t)l->data[i] != 0) return true;
    }
    return false;
}

// all - returns true if all elements are truthy (for bool list)
bool list_all_builtin(DesiList* l) {
    if (!l) return true;  // Empty list -> all() is true (vacuous truth)
    for (size_t i = 0; i < l->length; i++) {
        if ((intptr_t)l->data[i] == 0) return false;
    }
    return true;
}

// Comparison function for qsort (ascending order for integers)
static int compare_int_asc(const void* a, const void* b) {
    intptr_t ia = (intptr_t)(*(void**)a);
    intptr_t ib = (intptr_t)(*(void**)b);
    if (ia < ib) return -1;
    if (ia > ib) return 1;
    return 0;
}

// sorted - returns a new sorted list (ascending order)
DesiList* list_sorted_int(DesiList* l) {
    if (!l) return NULL;
    
    // Create a copy of the list
    DesiList* result = list_copy(l);
    if (!result) return NULL;
    
    // Sort the copy using qsort
    if (result->length > 1) {
        qsort(result->data, result->length, sizeof(void*), compare_int_asc);
    }
    
    return result;
}

// ==================== Prelude Builtins ====================

/* chr(n: int) → str — Encode a Unicode codepoint as a UTF-8 string. */
char* __desi_chr(int32_t codepoint) {
    char buf[8];
    int len = 0;

    if (codepoint < 0) {
        char* s = (char*)malloc(2);
        s[0] = '?'; s[1] = '\0';
        return s;
    } else if (codepoint < 0x80) {
        buf[0] = (char)codepoint;
        len = 1;
    } else if (codepoint < 0x800) {
        buf[0] = (char)(0xC0 | (codepoint >> 6));
        buf[1] = (char)(0x80 | (codepoint & 0x3F));
        len = 2;
    } else if (codepoint < 0x10000) {
        buf[0] = (char)(0xE0 | (codepoint >> 12));
        buf[1] = (char)(0x80 | ((codepoint >> 6) & 0x3F));
        buf[2] = (char)(0x80 | (codepoint & 0x3F));
        len = 3;
    } else if (codepoint < 0x110000) {
        buf[0] = (char)(0xF0 | (codepoint >> 18));
        buf[1] = (char)(0x80 | ((codepoint >> 12) & 0x3F));
        buf[2] = (char)(0x80 | ((codepoint >> 6) & 0x3F));
        buf[3] = (char)(0x80 | (codepoint & 0x3F));
        len = 4;
    } else {
        buf[0] = '?';
        len = 1;
    }

    char* s = (char*)malloc(len + 1);
    memcpy(s, buf, len);
    s[len] = '\0';
    return s;
}

/* ord(s: str) → int — Decode first UTF-8 char to Unicode codepoint. */
int32_t __desi_ord(const char* s) {
    if (!s || !*s) return 0;
    unsigned char c = (unsigned char)s[0];
    if (c < 0x80) return c;
    if ((c & 0xE0) == 0xC0 && s[1])
        return ((c & 0x1F) << 6) | (s[1] & 0x3F);
    if ((c & 0xF0) == 0xE0 && s[1] && s[2])
        return ((c & 0x0F) << 12) | ((s[1] & 0x3F) << 6) | (s[2] & 0x3F);
    if ((c & 0xF8) == 0xF0 && s[1] && s[2] && s[3])
        return ((c & 0x07) << 18) | ((s[1] & 0x3F) << 12) |
               ((s[2] & 0x3F) << 6) | (s[3] & 0x3F);
    return c;
}

/* hex(n: int) → str — Format as "0x..." */
char* __desi_hex(int32_t n) {
    char buf[32];
    if (n < 0)
        snprintf(buf, sizeof(buf), "-0x%x", (unsigned int)(-(int64_t)n));
    else
        snprintf(buf, sizeof(buf), "0x%x", (unsigned int)n);
    return strdup(buf);
}

/* oct(n: int) → str — Format as "0o..." */
char* __desi_oct(int32_t n) {
    char buf[32];
    if (n < 0)
        snprintf(buf, sizeof(buf), "-0o%o", (unsigned int)(-(int64_t)n));
    else
        snprintf(buf, sizeof(buf), "0o%o", (unsigned int)n);
    return strdup(buf);
}

/* bin(n: int) → str — Format as "0b..." */
char* __desi_bin(int32_t n) {
    char buf[48];
    int pos = 0;
    uint32_t val;
    if (n < 0) {
        buf[pos++] = '-';
        val = (uint32_t)(-(int64_t)n);
    } else {
        val = (uint32_t)n;
    }
    buf[pos++] = '0';
    buf[pos++] = 'b';
    if (val == 0) {
        buf[pos++] = '0';
    } else {
        int started = 0;
        for (int i = 31; i >= 0; i--) {
            if (val & (1U << i)) started = 1;
            if (started) buf[pos++] = (val & (1U << i)) ? '1' : '0';
        }
    }
    buf[pos] = '\0';
    return strdup(buf);
}

/* abs(n: int) → int */
int32_t __desi_abs_int(int32_t n) {
    return n < 0 ? -n : n;
}

/* abs(n: float) → float */
double __desi_abs_float(double n) {
    return fabs(n);
}

/* round(n: float, digits: int) → float */
double __desi_round(double n, int32_t digits) {
    if (digits == 0) return round(n);
    double factor = pow(10.0, (double)digits);
    return round(n * factor) / factor;
}

/* pow(base: int, exp: int) → int */
int32_t __desi_pow(int32_t base, int32_t exp) {
    if (exp < 0) return 0;
    int32_t result = 1;
    for (int32_t i = 0; i < exp; i++) result *= base;
    return result;
}

/* todo(msg: str) → never — panics with "not implemented" */
void __desi_todo(const char* msg) {
    if (msg && *msg)
        fprintf(stderr, "not implemented: %s\n", msg);
    else
        fprintf(stderr, "not implemented\n");
    exit(1);
}

/* hash(value: ptr) → int — FNV-1a hash of the pointer value */
int32_t __desi_hash(const void* ptr) {
    uint64_t h = 14695981039346656037ULL;
    uint64_t val = (uint64_t)(uintptr_t)ptr;
    for (int i = 0; i < 8; i++) {
        h ^= (val & 0xFF);
        h *= 1099511628211ULL;
        val >>= 8;
    }
    return (int32_t)(h & 0x7FFFFFFF);
}

/* id(value: ptr) → int — pointer identity as integer */
int32_t __desi_id(const void* ptr) {
    return (int32_t)((uintptr_t)ptr & 0x7FFFFFFF);
}

/* default_repr(obj: ptr, type_name: str) → str
 * Returns "<TypeName at 0xADDRESS>" like Python's default __repr__.
 * Used when printing objects that don't define __str__ or __repr__. */
const char* __desi_default_repr(const void* obj, const char* type_name) {
    char buf[128];
    snprintf(buf, sizeof(buf), "<%s at 0x%lx>", type_name, (unsigned long)(uintptr_t)obj);
    return strdup(buf);
}
