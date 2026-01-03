// builtins.c - Standard builtin functions for Desi
// These are common utility functions available in the prelude

#include <stdint.h>
#include <stdbool.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include "list.h"

// Panic function for integer division by zero
// Future: When hot-reload is implemented, this becomes process-local
// and supervisors can restart the process gracefully
void __panic_divzero(void) {
    fprintf(stderr, "panic: integer division by zero\n");
    exit(1);
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
