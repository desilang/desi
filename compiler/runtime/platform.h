/*
 * Desi Runtime Platform Abstraction
 * 
 * Provides cross-platform threading primitives.
 * POSIX (Linux/macOS): pthreads
 * Windows: Windows API (SRWLOCK, CONDITION_VARIABLE)
 */

#ifndef DESI_PLATFORM_H
#define DESI_PLATFORM_H

#include <stdbool.h>
#include <stdint.h>

/* Thread-local storage. MSVC has never accepted the GCC/Clang `__thread`
   spelling, so anything shared across threads must go through this. */
#ifndef DESI_THREAD_LOCAL
  #if defined(_MSC_VER)
    #define DESI_THREAD_LOCAL __declspec(thread)
  #else
    #define DESI_THREAD_LOCAL __thread
  #endif
#endif

#ifdef _WIN32
    #ifndef WIN32_LEAN_AND_MEAN
        #define WIN32_LEAN_AND_MEAN
    #endif
    #include <windows.h>
    
    /* Mutex: SRWLOCK for exclusive access */
    typedef SRWLOCK DesiPlatformMutex;
    #define DESI_MUTEX_INIT(m)       InitializeSRWLock(&(m))
    #define DESI_MUTEX_DESTROY(m)    /* SRWLOCK needs no destruction */
    #define DESI_MUTEX_LOCK(m)       AcquireSRWLockExclusive(&(m))
    #define DESI_MUTEX_UNLOCK(m)     ReleaseSRWLockExclusive(&(m))
    #define DESI_MUTEX_TRYLOCK(m)    TryAcquireSRWLockExclusive(&(m))
    
    /* Static initializers, for file-scope locks that are never explicitly
       initialized (the connection pool uses these). */
    #define DESI_MUTEX_STATIC_INIT   SRWLOCK_INIT
    #define DESI_COND_STATIC_INIT    CONDITION_VARIABLE_INIT

    /* Condition Variable */
    typedef CONDITION_VARIABLE DesiPlatformCond;
    #define DESI_COND_INIT(c)        InitializeConditionVariable(&(c))
    #define DESI_COND_DESTROY(c)     /* CV needs no destruction */
    #define DESI_COND_WAIT(c, m)     SleepConditionVariableSRW(&(c), &(m), INFINITE, 0)
    #define DESI_COND_SIGNAL(c)      WakeConditionVariable(&(c))
    #define DESI_COND_BROADCAST(c)   WakeAllConditionVariable(&(c))
    
    /* Thread */
    typedef HANDLE DesiPlatformThread;
    typedef DWORD (WINAPI *DesiThreadFunc)(LPVOID);
    
    /* RwLock: SRWLOCK supports read/write */
    typedef SRWLOCK DesiPlatformRwLock;
    #define DESI_RWLOCK_INIT(rw)         InitializeSRWLock(&(rw))
    #define DESI_RWLOCK_DESTROY(rw)      /* SRWLOCK needs no destruction */
    #define DESI_RWLOCK_RDLOCK(rw)       AcquireSRWLockShared(&(rw))
    #define DESI_RWLOCK_WRLOCK(rw)       AcquireSRWLockExclusive(&(rw))
    #define DESI_RWLOCK_RDUNLOCK(rw)     ReleaseSRWLockShared(&(rw))
    #define DESI_RWLOCK_WRUNLOCK(rw)     ReleaseSRWLockExclusive(&(rw))
    #define DESI_RWLOCK_TRYRDLOCK(rw)    TryAcquireSRWLockShared(&(rw))
    #define DESI_RWLOCK_TRYWRLOCK(rw)    TryAcquireSRWLockExclusive(&(rw))
    
#else
    #include <pthread.h>
    #include <time.h>   /* clock_gettime / struct timespec for the timed wait */
    #include <errno.h>  /* ETIMEDOUT */

    /* Mutex: pthread_mutex_t */
    typedef pthread_mutex_t DesiPlatformMutex;
    #define DESI_MUTEX_INIT(m)       pthread_mutex_init(&(m), NULL)
    #define DESI_MUTEX_DESTROY(m)    pthread_mutex_destroy(&(m))
    #define DESI_MUTEX_LOCK(m)       pthread_mutex_lock(&(m))
    #define DESI_MUTEX_UNLOCK(m)     pthread_mutex_unlock(&(m))
    #define DESI_MUTEX_TRYLOCK(m)    (pthread_mutex_trylock(&(m)) == 0)
    
    /* Static initializers, for file-scope locks that are never explicitly
       initialized (the connection pool uses these). */
    #define DESI_MUTEX_STATIC_INIT   PTHREAD_MUTEX_INITIALIZER
    #define DESI_COND_STATIC_INIT    PTHREAD_COND_INITIALIZER

    /* Condition Variable */
    typedef pthread_cond_t DesiPlatformCond;
    #define DESI_COND_INIT(c)        pthread_cond_init(&(c), NULL)
    #define DESI_COND_DESTROY(c)     pthread_cond_destroy(&(c))
    #define DESI_COND_WAIT(c, m)     pthread_cond_wait(&(c), &(m))
    #define DESI_COND_SIGNAL(c)      pthread_cond_signal(&(c))
    #define DESI_COND_BROADCAST(c)   pthread_cond_broadcast(&(c))
    
    /* Thread */
    typedef pthread_t DesiPlatformThread;
    typedef void* (*DesiThreadFunc)(void*);
    
    /* RwLock: pthread_rwlock_t */
    typedef pthread_rwlock_t DesiPlatformRwLock;
    #define DESI_RWLOCK_INIT(rw)         pthread_rwlock_init(&(rw), NULL)
    #define DESI_RWLOCK_DESTROY(rw)      pthread_rwlock_destroy(&(rw))
    #define DESI_RWLOCK_RDLOCK(rw)       pthread_rwlock_rdlock(&(rw))
    #define DESI_RWLOCK_WRLOCK(rw)       pthread_rwlock_wrlock(&(rw))
    #define DESI_RWLOCK_RDUNLOCK(rw)     pthread_rwlock_unlock(&(rw))
    #define DESI_RWLOCK_WRUNLOCK(rw)     pthread_rwlock_unlock(&(rw))
    #define DESI_RWLOCK_TRYRDLOCK(rw)    (pthread_rwlock_tryrdlock(&(rw)) == 0)
    #define DESI_RWLOCK_TRYWRLOCK(rw)    (pthread_rwlock_trywrlock(&(rw)) == 0)

#endif

/*
 * Timed waiting.
 *
 * Expressed as a relative timeout in milliseconds because the two platforms
 * disagree: Win32's SleepConditionVariableSRW takes a relative timeout while
 * pthread_cond_timedwait takes an absolute deadline. Deriving the deadline
 * here keeps every caller portable — and callers that need one can build it
 * from desi_now_ms().
 *
 * Functions rather than macros: the POSIX side needs a local timespec.
 */

/* Milliseconds since an unspecified epoch. Only differences are meaningful. */
static inline int64_t desi_now_ms(void) {
#ifdef _WIN32
    return (int64_t)GetTickCount64();
#else
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    return (int64_t)ts.tv_sec * 1000 + (int64_t)ts.tv_nsec / 1000000;
#endif
}

/* Wait for a signal or timeout. Returns 1 if signalled, 0 if it timed out.
   Must be called with the mutex held; the mutex is held again on return. */
static inline int desi_cond_timedwait_ms(DesiPlatformCond* cond,
                                         DesiPlatformMutex* mutex,
                                         int64_t timeout_ms) {
    if (timeout_ms < 0) timeout_ms = 0;
#ifdef _WIN32
    if (SleepConditionVariableSRW(cond, mutex, (DWORD)timeout_ms, 0)) {
        return 1;
    }
    /* Anything other than a timeout is still a wakeup as far as the caller is
       concerned: it re-checks its own predicate either way. */
    return GetLastError() == ERROR_TIMEOUT ? 0 : 1;
#else
    struct timespec deadline;
    clock_gettime(CLOCK_REALTIME, &deadline);
    deadline.tv_sec += (time_t)(timeout_ms / 1000);
    deadline.tv_nsec += (long)(timeout_ms % 1000) * 1000000L;
    if (deadline.tv_nsec >= 1000000000L) {
        deadline.tv_sec += 1;
        deadline.tv_nsec -= 1000000000L;
    }
    return pthread_cond_timedwait(cond, mutex, &deadline) == ETIMEDOUT ? 0 : 1;
#endif
}

#endif /* DESI_PLATFORM_H */
