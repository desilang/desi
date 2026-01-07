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
    
    /* Condition Variable */
    typedef CONDITION_VARIABLE DesiPlatformCond;
    #define DESI_COND_INIT(c)        InitializeConditionVariable(&(c))
    #define DESI_COND_DESTROY(c)     /* CV needs no destruction */
    #define DESI_COND_WAIT(c, m)     SleepConditionVariableSRW(&(c), &(m), INFINITE, 0)
    #define DESI_COND_SIGNAL(c)      WakeConditionVariable(&(c))
    #define DESI_COND_BROADCAST(c)   WakeAllConditionVariable(&(c))
    
    /* Thread */
    typedef HANDLE DesiPlatformThread;
    typedef DWORD WINAPI (*DesiThreadFunc)(LPVOID);
    
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
    
    /* Mutex: pthread_mutex_t */
    typedef pthread_mutex_t DesiPlatformMutex;
    #define DESI_MUTEX_INIT(m)       pthread_mutex_init(&(m), NULL)
    #define DESI_MUTEX_DESTROY(m)    pthread_mutex_destroy(&(m))
    #define DESI_MUTEX_LOCK(m)       pthread_mutex_lock(&(m))
    #define DESI_MUTEX_UNLOCK(m)     pthread_mutex_unlock(&(m))
    #define DESI_MUTEX_TRYLOCK(m)    (pthread_mutex_trylock(&(m)) == 0)
    
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

#endif /* DESI_PLATFORM_H */
