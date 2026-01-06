/*
 * Desi Runtime Semaphore Header
 * 
 * Counting semaphore: allows up to N concurrent acquires.
 */

#ifndef DESI_SEMAPHORE_H
#define DESI_SEMAPHORE_H

#include "platform.h"
#include <stdbool.h>
#include <stdint.h>

/*
 * Semaphore structure - counting semaphore
 */
typedef struct Semaphore {
    DesiPlatformMutex mutex;
    DesiPlatformCond cond;
    int64_t count;     /* Current available permits */
    int64_t max_count; /* Maximum permits (0 = unbounded) */
} Semaphore;

/* Semaphore functions */
Semaphore* semaphore_new(int64_t initial_count);
void semaphore_destroy(Semaphore* sem);

/* Acquire (decrement) - blocks if count is 0 */
void semaphore_acquire(Semaphore* sem);

/* Release (increment) - wakes one waiting thread */
void semaphore_release(Semaphore* sem);

/* Try to acquire without blocking - returns true if acquired */
bool semaphore_try_acquire(Semaphore* sem);

/* Get current count (for debugging) */
int64_t semaphore_count(Semaphore* sem);

#endif /* DESI_SEMAPHORE_H */
