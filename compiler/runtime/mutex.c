/*
 * Desi Runtime Mutex Implementation
 * Cross-platform: pthreads on POSIX, SRWLOCK on Windows.
 */

#include "mutex.h"
#include <stdlib.h>
#include <stdio.h>

/*
 * Create a new mutex protecting the given value
 */
DesiMutex* mutex_new(void* value) {
    DesiMutex* m = malloc(sizeof(DesiMutex));
    if (!m) return NULL;
    
#ifdef _WIN32
    InitializeSRWLock(&m->lock);
    /* SRWLOCK doesn't need explicit init check - always succeeds */
#else
    if (pthread_mutex_init(&m->lock, NULL) != 0) {
        free(m);
        return NULL;
    }
#endif
    
    m->value = value;
    m->initialized = true;
    return m;
}

/*
 * Acquire the lock, blocking until available
 */
MutexGuard* mutex_lock(DesiMutex* m) {
    if (!m || !m->initialized) return NULL;
    
#ifdef _WIN32
    AcquireSRWLockExclusive(&m->lock);
#else
    pthread_mutex_lock(&m->lock);
#endif
    
    MutexGuard* guard = malloc(sizeof(MutexGuard));
    if (!guard) {
#ifdef _WIN32
        ReleaseSRWLockExclusive(&m->lock);
#else
        pthread_mutex_unlock(&m->lock);
#endif
        return NULL;
    }
    
    guard->mutex = m;
    guard->value = m->value;
    return guard;
}

/*
 * Try to acquire the lock without blocking
 */
MutexGuard* mutex_try_lock(DesiMutex* m) {
    if (!m || !m->initialized) return NULL;
    
#ifdef _WIN32
    if (!TryAcquireSRWLockExclusive(&m->lock)) {
        return NULL;  /* Lock not available */
    }
#else
    if (pthread_mutex_trylock(&m->lock) != 0) {
        return NULL;  /* Lock not available */
    }
#endif
    
    MutexGuard* guard = malloc(sizeof(MutexGuard));
    if (!guard) {
#ifdef _WIN32
        ReleaseSRWLockExclusive(&m->lock);
#else
        pthread_mutex_unlock(&m->lock);
#endif
        return NULL;
    }
    
    guard->mutex = m;
    guard->value = m->value;
    return guard;
}

/*
 * Release the lock
 */
void mutex_unlock(MutexGuard* guard) {
    if (!guard || !guard->mutex) return;
    
#ifdef _WIN32
    ReleaseSRWLockExclusive(&guard->mutex->lock);
#else
    pthread_mutex_unlock(&guard->mutex->lock);
#endif
    free(guard);
}

/*
 * Get the value from a guard
 */
void* mutex_guard_get(MutexGuard* guard) {
    if (!guard) return NULL;
    return guard->value;
}

/*
 * Set the value through a guard
 */
void mutex_guard_set(MutexGuard* guard, void* new_value) {
    if (!guard || !guard->mutex) return;
    guard->value = new_value;
    guard->mutex->value = new_value;
}

/*
 * Destroy a mutex
 */
void mutex_destroy(DesiMutex* m) {
    if (!m) return;
    
#ifdef _WIN32
    /* SRWLOCK doesn't need explicit destruction */
#else
    if (m->initialized) {
        pthread_mutex_destroy(&m->lock);
    }
#endif
    free(m);
}
