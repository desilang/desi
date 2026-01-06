/*
 * Desi Runtime Semaphore Implementation
 * 
 * Counting semaphore using mutex + condition variable.
 * Cross-platform: works on POSIX and Windows.
 */

#include "semaphore.h"
#include <stdlib.h>
#include <stdio.h>

/*
 * Create a new semaphore with initial count
 */
Semaphore* semaphore_new(int64_t initial_count) {
    Semaphore* sem = malloc(sizeof(Semaphore));
    if (!sem) return NULL;
    
    DESI_MUTEX_INIT(sem->mutex);
    DESI_COND_INIT(sem->cond);
    sem->count = initial_count > 0 ? initial_count : 0;
    sem->max_count = 0; /* Unbounded by default */
    
    return sem;
}

/*
 * Destroy the semaphore
 */
void semaphore_destroy(Semaphore* sem) {
    if (!sem) return;
    DESI_COND_DESTROY(sem->cond);
    DESI_MUTEX_DESTROY(sem->mutex);
    free(sem);
}

/*
 * Acquire (decrement) - blocks if count is 0
 */
void semaphore_acquire(Semaphore* sem) {
    if (!sem) return;
    
    DESI_MUTEX_LOCK(sem->mutex);
    
    /* Wait while count is 0 */
    while (sem->count <= 0) {
        DESI_COND_WAIT(sem->cond, sem->mutex);
    }
    
    /* Decrement count */
    sem->count--;
    
    DESI_MUTEX_UNLOCK(sem->mutex);
}

/*
 * Release (increment) - wakes one waiting thread
 */
void semaphore_release(Semaphore* sem) {
    if (!sem) return;
    
    DESI_MUTEX_LOCK(sem->mutex);
    
    /* Increment count */
    sem->count++;
    
    /* Wake one waiting thread */
    DESI_COND_SIGNAL(sem->cond);
    
    DESI_MUTEX_UNLOCK(sem->mutex);
}

/*
 * Try to acquire without blocking
 * Returns true if acquired, false otherwise
 */
bool semaphore_try_acquire(Semaphore* sem) {
    if (!sem) return false;
    
    bool acquired = false;
    
    DESI_MUTEX_LOCK(sem->mutex);
    
    if (sem->count > 0) {
        sem->count--;
        acquired = true;
    }
    
    DESI_MUTEX_UNLOCK(sem->mutex);
    
    return acquired;
}

/*
 * Get current count (for debugging)
 */
int64_t semaphore_count(Semaphore* sem) {
    if (!sem) return 0;
    
    DESI_MUTEX_LOCK(sem->mutex);
    int64_t count = sem->count;
    DESI_MUTEX_UNLOCK(sem->mutex);
    
    return count;
}
