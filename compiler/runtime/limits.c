/*
 * Desi Runtime Limits
 * 
 * Provides configurable runtime safety limits:
 * - Recursion depth tracking with panic on overflow
 */

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include "exception.h"

/* Platform-specific thread-local storage */
#ifdef _WIN32
    #define DESI_THREAD_LOCAL __declspec(thread)
#else
    #define DESI_THREAD_LOCAL __thread
#endif

/* Never-inline marker: the overflow path must stay OUT of callers.
 * This file is also compiled to LLVM bitcode for release-build LTO —
 * if the whole guard inlined, the cold path's 256-byte message buffer
 * would land in every user function's stack frame (allocas hoist to
 * function entry), making recursion measurably slower. */
#ifdef _MSC_VER
    #define DESI_NOINLINE __declspec(noinline)
#else
    #define DESI_NOINLINE __attribute__((noinline))
#endif

/* Thread-local call depth counter */
static DESI_THREAD_LOCAL int64_t __call_depth = 0;

/* Configurable limits (global) */
static int64_t __max_recursion = 1000;

/*
 * Cold path: build the panic message and raise. Kept out-of-line so the
 * hot guard below inlines to just increment + compare + rare call.
 */
DESI_NOINLINE static void __desi_recursion_overflow(const char* func_name) {
    char msg[256];
    snprintf(msg, sizeof(msg), "maximum recursion depth exceeded (%lld) in '%s'",
             (long long)__max_recursion, func_name ? func_name : "<unknown>");
    __desi_raise(DESI_EXC_RUNTIME_ERROR, msg, "RuntimeError");
}

/*
 * Call enter - increment depth and check limit
 * Called at the start of every user function.
 * func_name is used in the panic message if recursion depth is exceeded.
 */
void __desi_call_enter(const char* func_name) {
    __call_depth++;
    if (__call_depth > __max_recursion) {
        __desi_recursion_overflow(func_name);
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
