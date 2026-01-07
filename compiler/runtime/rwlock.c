/*
 * Desi Runtime RwLock Implementation
 * 
 * Reader-writer lock: multiple concurrent readers OR one exclusive writer.
 */

#include "rwlock.h"
#include <stdlib.h>
#include <stdio.h>

/*
 * Create a new RwLock protecting initial_value
 */
RwLock* rwlock_new(void* initial_value) {
    RwLock* rw = malloc(sizeof(RwLock));
    if (!rw) return NULL;
    
    DESI_RWLOCK_INIT(rw->lock);
    rw->value = initial_value;
    
    return rw;
}

/*
 * Destroy the RwLock (does NOT free the protected value)
 */
void rwlock_destroy(RwLock* rw) {
    if (!rw) return;
    DESI_RWLOCK_DESTROY(rw->lock);
    free(rw);
}

/*
 * Acquire read lock (blocking)
 * Returns a ReadGuard for RAII cleanup
 */
ReadGuard* rwlock_read(RwLock* rw) {
    if (!rw) return NULL;
    
    DESI_RWLOCK_RDLOCK(rw->lock);
    
    ReadGuard* guard = malloc(sizeof(ReadGuard));
    if (!guard) {
        DESI_RWLOCK_RDUNLOCK(rw->lock);
        return NULL;
    }
    
    guard->rwlock = rw;
    guard->value = rw->value;
    
    return guard;
}

/*
 * Acquire write lock (blocking)
 * Returns a WriteGuard for RAII cleanup
 */
WriteGuard* rwlock_write(RwLock* rw) {
    if (!rw) return NULL;
    
    DESI_RWLOCK_WRLOCK(rw->lock);
    
    WriteGuard* guard = malloc(sizeof(WriteGuard));
    if (!guard) {
        DESI_RWLOCK_WRUNLOCK(rw->lock);
        return NULL;
    }
    
    guard->rwlock = rw;
    guard->value = rw->value;
    
    return guard;
}

/*
 * Try to acquire read lock (non-blocking)
 * Returns NULL if lock not available
 */
ReadGuard* rwlock_try_read(RwLock* rw) {
    if (!rw) return NULL;
    
    if (!DESI_RWLOCK_TRYRDLOCK(rw->lock)) {
        return NULL;  /* Lock not available */
    }
    
    ReadGuard* guard = malloc(sizeof(ReadGuard));
    if (!guard) {
        DESI_RWLOCK_RDUNLOCK(rw->lock);
        return NULL;
    }
    
    guard->rwlock = rw;
    guard->value = rw->value;
    
    return guard;
}

/*
 * Try to acquire write lock (non-blocking)
 * Returns NULL if lock not available
 */
WriteGuard* rwlock_try_write(RwLock* rw) {
    if (!rw) return NULL;
    
    if (!DESI_RWLOCK_TRYWRLOCK(rw->lock)) {
        return NULL;  /* Lock not available */
    }
    
    WriteGuard* guard = malloc(sizeof(WriteGuard));
    if (!guard) {
        DESI_RWLOCK_WRUNLOCK(rw->lock);
        return NULL;
    }
    
    guard->rwlock = rw;
    guard->value = rw->value;
    
    return guard;
}

/*
 * Release read lock (guard cleanup)
 */
void read_guard_unlock(ReadGuard* guard) {
    if (!guard) return;
    if (guard->rwlock) {
        DESI_RWLOCK_RDUNLOCK(guard->rwlock->lock);
    }
    free(guard);
}

/*
 * Release write lock (guard cleanup)
 */
void write_guard_unlock(WriteGuard* guard) {
    if (!guard) return;
    if (guard->rwlock) {
        DESI_RWLOCK_WRUNLOCK(guard->rwlock->lock);
    }
    free(guard);
}

/*
 * Get protected value from read guard (read-only access)
 */
void* read_guard_value(ReadGuard* guard) {
    if (!guard) return NULL;
    return guard->value;
}

/*
 * Get protected value from write guard (read-write access)
 */
void* write_guard_value(WriteGuard* guard) {
    if (!guard) return NULL;
    return guard->value;
}
