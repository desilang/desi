/*
 * Desi Runtime Atomic Operations Header
 * 
 * Lock-free atomic operations using C11 stdatomic.
 * Atomic<int> provides thread-safe integer operations.
 */

#ifndef DESI_ATOMIC_H
#define DESI_ATOMIC_H

#include <stdint.h>
#include <stdbool.h>
#include <stdatomic.h>

/*
 * Atomic integer (i64) container
 */
typedef struct AtomicInt {
    _Atomic int64_t value;
} AtomicInt;

/* AtomicInt functions */
AtomicInt* atomic_int_new(int64_t initial);
void atomic_int_destroy(AtomicInt* a);

/* Load the current value atomically */
int64_t atomic_int_load(AtomicInt* a);

/* Store a new value atomically */
void atomic_int_store(AtomicInt* a, int64_t value);

/* Add and return the NEW value */
int64_t atomic_int_add(AtomicInt* a, int64_t delta);

/* Subtract and return the NEW value */
int64_t atomic_int_sub(AtomicInt* a, int64_t delta);

/* Increment by 1 and return the NEW value */
int64_t atomic_int_inc(AtomicInt* a);

/* Decrement by 1 and return the NEW value */
int64_t atomic_int_dec(AtomicInt* a);

/* Compare-and-swap: if current == expected, set to desired and return true */
bool atomic_int_compare_exchange(AtomicInt* a, int64_t expected, int64_t desired);

/* Exchange: atomically replace value and return the OLD value */
int64_t atomic_int_exchange(AtomicInt* a, int64_t new_value);

#endif /* DESI_ATOMIC_H */
