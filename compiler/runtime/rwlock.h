/*
 * Desi Runtime RwLock Header
 * 
 * Reader-writer lock: multiple concurrent readers OR one exclusive writer.
 */

#ifndef DESI_RWLOCK_H
#define DESI_RWLOCK_H

#include "platform.h"
#include <stdbool.h>

/*
 * RwLock structure - protects a value with read/write access
 */
typedef struct RwLock {
    DesiPlatformRwLock lock;
    void* value;           /* Pointer to protected value (boxed) */
} RwLock;

/*
 * ReadGuard - RAII handle for read access
 */
typedef struct ReadGuard {
    RwLock* rwlock;
    void* value;           /* Pointer to value (read-only access) */
} ReadGuard;

/*
 * WriteGuard - RAII handle for write access
 */
typedef struct WriteGuard {
    RwLock* rwlock;
    void* value;           /* Pointer to value (read-write access) */
} WriteGuard;

/* RwLock functions */
RwLock* rwlock_new(void* initial_value);
void rwlock_destroy(RwLock* rw);

/* Locking */
ReadGuard* rwlock_read(RwLock* rw);
WriteGuard* rwlock_write(RwLock* rw);
ReadGuard* rwlock_try_read(RwLock* rw);
WriteGuard* rwlock_try_write(RwLock* rw);

/* Unlocking (guard cleanup) */
void read_guard_unlock(ReadGuard* guard);
void write_guard_unlock(WriteGuard* guard);

/* Guard value access */
void* read_guard_value(ReadGuard* guard);
void* write_guard_value(WriteGuard* guard);

#endif /* DESI_RWLOCK_H */
