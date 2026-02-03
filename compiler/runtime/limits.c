/*
 * Desi Runtime Limits
 * 
 * Provides configurable runtime safety limits:
 * - Recursion depth tracking with panic on overflow
 */

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>

/* Platform-specific thread-local storage */
#ifdef _WIN32
    #define DESI_THREAD_LOCAL __declspec(thread)
#else
    #define DESI_THREAD_LOCAL __thread
#endif

/* Thread-local call depth counter */
static DESI_THREAD_LOCAL int64_t __call_depth = 0;

/* Configurable limits (global) */
static int64_t __max_recursion = 1000;

/*
 * Call enter - increment depth and check limit
 * Called at the start of every user function
 */
void __desi_call_enter(void) {
    __call_depth++;
    if (__call_depth > __max_recursion) {
        fprintf(stderr, "Desi panic: maximum recursion depth exceeded (%lld)\n", 
                (long long)__max_recursion);
        exit(1);
    }
}

/*
 * Call exit - decrement depth
 * Called before every return in user functions
 */
void __desi_call_exit(void) {
    __call_depth--;
}

/*
 * Set max recursion depth
 * Can be called at runtime to adjust limit
 */
void __desi_set_max_recursion(int64_t limit) {
    if (limit > 0) {
        __max_recursion = limit;
    }
}

/*
 * Get current recursion depth (for debugging)
 */
int64_t __desi_get_call_depth(void) {
    return __call_depth;
}

/*
 * Get max recursion limit (for debugging)
 */
int64_t __desi_get_max_recursion(void) {
    return __max_recursion;
}
