/*
 * Desi Runtime Mutex Implementation
 * 
 * Provides thread-safe mutual exclusion with RAII guard pattern.
 * Cross-platform: Uses pthreads on POSIX (Linux/macOS) and SRWLOCK on Windows.
 */

#ifndef DESI_MUTEX_H
#define DESI_MUTEX_H

#include <stdbool.h>
#include <stdint.h>

#ifdef _WIN32
    #ifndef WIN32_LEAN_AND_MEAN
        #define WIN32_LEAN_AND_MEAN
    #endif
    #include <windows.h>
    typedef SRWLOCK PlatformMutex;
#else
    #include <pthread.h>
    typedef pthread_mutex_t PlatformMutex;
#endif

/*
 * Mutex wraps a platform lock and holds a pointer to the protected value.
 * The value pointer allows us to return a typed guard.
 */
typedef struct {
    PlatformMutex lock;
    void* value;           /* Pointer to the protected value */
    bool initialized;
} DesiMutex;

/*
 * MutexGuard represents an acquired lock.
 * While it exists, the mutex is held.
 * When dropped, the mutex is released.
 */
typedef struct {
    DesiMutex* mutex;      /* The mutex we're holding */
    void* value;           /* Direct access to the value */
} MutexGuard;

/* Create a new mutex protecting the given value */
DesiMutex* mutex_new(void* value);

/* Acquire the lock, returns a guard. Blocks until available. */
MutexGuard* mutex_lock(DesiMutex* m);

/* Try to acquire the lock without blocking. Returns NULL if not available. */
MutexGuard* mutex_try_lock(DesiMutex* m);

/* Release the lock (called when guard is dropped) */
void mutex_unlock(MutexGuard* guard);

/* Get the value from a guard (for dereferencing) */
void* mutex_guard_get(MutexGuard* guard);

/* Set the value through a guard */
void mutex_guard_set(MutexGuard* guard, void* new_value);

/* Destroy a mutex (should only be called when no guards exist) */
void mutex_destroy(DesiMutex* m);

#endif /* DESI_MUTEX_H */
