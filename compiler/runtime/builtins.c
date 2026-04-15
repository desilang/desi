// builtins.c - Standard builtin functions for Desi
// These are common utility functions available in the prelude

#include <stdint.h>
#include <stdbool.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "list.h"

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
// Future: When hot-reload is implemented, this becomes process-local
// and supervisors can restart the process gracefully
void __panic_divzero(void) {
    fprintf(stderr, "panic: integer division by zero\n");
    exit(1);
}

// Panic function for unwrapping None (Option.Nothing)
void __panic_unwrap_none(void) {
    fprintf(stderr, "panic: called unwrap() on a None value\n");
    exit(1);
}

// Panic function for unwrapping Err (Result.Err)
void __panic_unwrap_err(void) {
    fprintf(stderr, "panic: called unwrap() on an Err value\n");
    exit(1);
}

// Panic function for unwrap_err on Ok (Result.Ok)
void __panic_unwrap_ok(void) {
    fprintf(stderr, "panic: called unwrap_err() on an Ok value\n");
    exit(1);
}

// Panic function for expect() with custom message
void __panic_expect(const char* msg) {
    fprintf(stderr, "panic: %s\n", msg);
    exit(1);
}

// Panic with user-provided message (used by 'raise' statement)
void __desi_panic(const char* msg) {
    fprintf(stderr, "panic: %s\n", msg ? msg : "(null)");
    exit(1);
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
int __math_is_nan(double x) {
    return isnan(x) ? 1 : 0;
}

// is_inf - check if float is infinity (positive or negative)
int __math_is_inf(double x) {
    return isinf(x) ? 1 : 0;
}

// is_finite - check if float is a normal number (not NaN or infinity)
int __math_is_finite(double x) {
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
