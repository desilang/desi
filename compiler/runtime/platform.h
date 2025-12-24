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
    
#endif

#endif /* DESI_PLATFORM_H */
