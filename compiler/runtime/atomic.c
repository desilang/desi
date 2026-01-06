/*
 * Desi Runtime Atomic Operations Implementation
 * 
 * Lock-free atomic operations using C11 stdatomic.
 */

#include "atomic.h"
#include <stdlib.h>

/*
 * Create a new atomic integer with initial value
 */
AtomicInt* atomic_int_new(int64_t initial) {
    AtomicInt* a = malloc(sizeof(AtomicInt));
    if (!a) return NULL;
    atomic_init(&a->value, initial);
    return a;
}

/*
 * Destroy the atomic integer
 */
void atomic_int_destroy(AtomicInt* a) {
    if (a) free(a);
}

/*
 * Load the current value atomically
 */
int64_t atomic_int_load(AtomicInt* a) {
    if (!a) return 0;
    return atomic_load(&a->value);
}

/*
 * Store a new value atomically
 */
void atomic_int_store(AtomicInt* a, int64_t value) {
    if (!a) return;
    atomic_store(&a->value, value);
}

/*
 * Add delta and return the NEW value
 */
int64_t atomic_int_add(AtomicInt* a, int64_t delta) {
    if (!a) return 0;
    return atomic_fetch_add(&a->value, delta) + delta;
}

/*
 * Subtract delta and return the NEW value
 */
int64_t atomic_int_sub(AtomicInt* a, int64_t delta) {
    if (!a) return 0;
    return atomic_fetch_sub(&a->value, delta) - delta;
}

/*
 * Increment by 1 and return the NEW value
 */
int64_t atomic_int_inc(AtomicInt* a) {
    return atomic_int_add(a, 1);
}

/*
 * Decrement by 1 and return the NEW value
 */
int64_t atomic_int_dec(AtomicInt* a) {
    return atomic_int_sub(a, 1);
}

/*
 * Compare-and-swap: if current == expected, set to desired and return true
 */
bool atomic_int_compare_exchange(AtomicInt* a, int64_t expected, int64_t desired) {
    if (!a) return false;
    return atomic_compare_exchange_strong(&a->value, &expected, desired);
}

/*
 * Exchange: atomically replace value and return the OLD value
 */
int64_t atomic_int_exchange(AtomicInt* a, int64_t new_value) {
    if (!a) return 0;
    return atomic_exchange(&a->value, new_value);
}
