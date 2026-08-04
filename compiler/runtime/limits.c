/*
 * Desi Runtime Limits
 *
 * Provides configurable runtime safety limits:
 * - Stack headroom checking with panic on exhaustion
 *
 * The guard used to be an exact call-depth counter: every user function
 * incremented a thread-local on entry and decremented it on return. That is
 * what it cost, measured on fib(32) with clang -O2:
 *
 *     out-of-line counter (as emitted)   16.8 ms
 *     the same counter, inlined          13.8 ms
 *     stack headroom check                2.5 ms
 *
 * Inlining barely helped and a plain global was no faster than the
 * thread-local, so neither the call nor the TLS was the cost — the paired
 * increment and decrement was. Checking headroom instead is stateless: there is
 * nothing to undo, so the exit hook disappears entirely, which is most of the
 * difference.
 *
 * It also guards the resource that actually runs out. A frame count is only a
 * proxy for stack space, and a poor one when frames differ in size.
 */

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include "exception.h"

#if defined(_WIN32)
    #include <windows.h>
#else
    #include <pthread.h>
#endif

/* Platform-specific thread-local storage */
#ifdef _WIN32
    #define DESI_THREAD_LOCAL __declspec(thread)
#else
    #define DESI_THREAD_LOCAL __thread
#endif

/* Configurable limit, in frames. Kept in the units the public API speaks
 * (set_recursion_limit / recursion_limit), and converted to a byte budget when
 * a thread computes its floor. */
static int64_t __max_recursion = 1000;

/* Bytes of stack one frame is assumed to take when turning the frame limit into
 * a byte budget. A Desi frame with a couple of locals is well under this; the
 * estimate only has to be the right order of magnitude, because the floor is
 * also clamped to the thread's real stack. */
#define DESI_FRAME_ESTIMATE 512

/* Headroom kept below the floor, covering two things.
 *
 * The panic path needs room to run: it formats a message and raises, and it has
 * to do that on the stack that just ran out.
 *
 * And the check reads the address of the *first* alloca in the frame, which is
 * the frame's high end — later allocas in the same function extend below it.
 * So a function can dip this far past the floor after being waved through. The
 * reserve therefore has to exceed the largest frame a Desi function can build.
 * Frames here are small by construction: locals of any size live on the heap,
 * and allocas are box and result slots of a few words each, all hoisted into
 * one entry block. 64 KB is far more than that shape can reach, and it is
 * charged once against the stack, not per call. */
#define DESI_STACK_RESERVE (64 * 1024)

/* The lowest stack address a thread may touch. Below it, the guard panics.
 *
 * Exported, and deliberately not static: the compiler emits the comparison
 * inline in every guarded function and loads this directly, so a call happens
 * only on the rare path.
 */
/* The two sentinels are chosen so the inline check needs exactly one compare.
 *
 * UNINIT is the highest address there is, so any real stack pointer compares
 * below it and the very first guarded call on a thread takes the cold path,
 * which is what computes the real floor. DISABLED is the lowest, so nothing
 * compares below it and the check never fires — that is the state for a
 * platform that will not report its stack bounds.
 *
 * Encoding both in the value itself is why the emitted guard is a load, a
 * compare and a branch, with no separate "is it set up yet" test. */
#define DESI_FLOOR_UNINIT   ((char*)UINTPTR_MAX)
#define DESI_FLOOR_DISABLED ((char*)0)

DESI_THREAD_LOCAL char* __desi_stack_floor = DESI_FLOOR_UNINIT;
#define __stack_floor __desi_stack_floor

/* Whether this thread's floor came from the real end of the stack rather than
 * from the configured frame budget. Only the message reads it: "you asked for
 * more frames than exist" and "you asked for ten million and the stack ran out"
 * are different situations, and saying "maximum recursion depth exceeded
 * (10000000)" for the second one would be a lie. */
static DESI_THREAD_LOCAL int __stack_floor_is_hard = 0;

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

/* Where this thread's stack ends. Returns the lowest usable address, or NULL
 * when the platform will not say — in which case the guard stands down rather
 * than guessing, because a wrong floor either panics on healthy programs or
 * never fires at all. */
static char* desi_stack_low_address(size_t* out_size) {
#if defined(_WIN32)
    /* Windows 8 and later. The pair is (low, high) of the committed region. */
    ULONG_PTR low = 0, high = 0;
    GetCurrentThreadStackLimits(&low, &high);
    if (low == 0 || high <= low) return NULL;
    if (out_size) *out_size = (size_t)(high - low);
    return (char*)low;
#elif defined(__APPLE__)
    pthread_t self = pthread_self();
    char* high = (char*)pthread_get_stackaddr_np(self); /* macOS returns the TOP */
    size_t size = pthread_get_stacksize_np(self);
    if (!high || size == 0) return NULL;
    if (out_size) *out_size = size;
    return high - size;
#else
    pthread_attr_t attr;
    void* addr = NULL;
    size_t size = 0;
    if (pthread_getattr_np(pthread_self(), &attr) != 0) return NULL;
    if (pthread_attr_getstack(&attr, &addr, &size) != 0) {
        pthread_attr_destroy(&attr);
        return NULL;
    }
    pthread_attr_destroy(&attr);
    if (!addr || size == 0) return NULL;
    if (out_size) *out_size = size;
    return (char*)addr; /* glibc reports the LOW address */
#endif
}

/*
 * Compute this thread's floor. Called once per thread, from the guard itself
 * on first use and from __desi_stack_guard_init at thread start.
 *
 * The floor is the higher of two bounds: the frame budget the user asked for,
 * and the real end of the stack less a reserve. So raising the limit lets a
 * program recurse deeper, but never past the point where it would fault.
 */
void __desi_stack_guard_init(void) {
    size_t size = 0;
    char* low = desi_stack_low_address(&size);
    if (!low) {
        /* Unknown bounds: stand the guard down rather than fabricate a limit.
         * The sentinel — rather than NULL — is what keeps the inline check on
         * its fast path, so this costs a load and a compare, not a call. */
        __stack_floor = DESI_FLOOR_DISABLED;
        return;
    }

    /* A stack too small to hold the reserve cannot be guarded this way: the
     * floor would sit above the stack top and every call would panic. Nothing
     * Desi creates has a stack that small — Windows threads default to 1 MB and
     * pthreads to 8 — but a host embedding the runtime on its own thread could,
     * and refusing to guard is the only honest answer when the reserve does not
     * fit. */
    if (size <= (size_t)(2 * DESI_STACK_RESERVE)) {
        __stack_floor = DESI_FLOOR_DISABLED;
        return;
    }

    char* hard = low + DESI_STACK_RESERVE;

    char probe;
    char* budget = (char*)0;
    {
        uint64_t want = (uint64_t)__max_recursion * DESI_FRAME_ESTIMATE;
        if (want > (uint64_t)size) {
            budget = hard;               /* asked for more than the stack holds */
        } else {
            budget = (char*)&probe - want;
            if (budget < hard) budget = hard;
        }
    }
    __stack_floor = budget;
    __stack_floor_is_hard = (budget == hard);
}

/*
 * Cold path: build the panic message and raise. Kept out-of-line so the
 * hot guard below stays small.
 */
DESI_NOINLINE static void __desi_recursion_overflow(const char* func_name) {
    char msg[256];
    const char* where = func_name ? func_name : "<unknown>";
    if (__stack_floor_is_hard) {
        /* The stack itself ran out, before the configured budget did. Naming
         * the frame limit here would be misleading — with the limit raised to
         * ten million, "maximum recursion depth exceeded (10000000)" says the
         * opposite of what happened. */
        snprintf(msg, sizeof(msg),
                 "out of stack in '%s' (recursion limit is %lld frames, but the "
                 "stack ran out first)", where, (long long)__max_recursion);
    } else {
        snprintf(msg, sizeof(msg), "maximum recursion depth exceeded (%lld) in '%s'",
                 (long long)__max_recursion, where);
    }
    __desi_raise(DESI_EXC_RUNTIME_ERROR, msg, "RuntimeError");
}

/*
 * The guard, called at the start of every user function that can recurse.
 *
 * One comparison against a thread-local in the common case. There is no
 * matching exit hook — nothing was changed on the way in, so nothing needs
 * undoing on the way out, which is where the old counter spent its time.
 */
void __desi_call_enter(const char* func_name) {
    char probe;
    char* floor = __stack_floor;
    if (floor == DESI_FLOOR_UNINIT) {
        /* First guarded call on this thread. Threads are created in several
         * places — futures, the scheduler, the supervisor pool, the websocket
         * pinger — and a library could make one too, so the floor is computed
         * on demand rather than at every thread entry, where one missed site
         * would silently leave a thread unguarded. */
        __desi_stack_guard_init();
        floor = __stack_floor;
    }
    if (floor != DESI_FLOOR_DISABLED && &probe < floor) {
        __desi_recursion_overflow(func_name);
    }
}

/*
 * Retained so old objects and the tail-call path keep linking. It has nothing
 * to undo now.
 */
void __desi_call_exit(void) {
}

/*
 * Set max recursion depth
 * Can be called at runtime to adjust limit
 */
void __desi_set_max_recursion(int64_t limit) {
    if (limit > 0) {
        __max_recursion = limit;
        /* The floor is derived from the limit, so it has to be recomputed —
         * otherwise raising the limit would not let the caller recurse any
         * deeper, which is the one thing this function is for. */
        __desi_stack_guard_init();
    }
}

/*
 * Current call depth, for sys.call_depth().
 *
 * An estimate now rather than an exact count. Nothing tracks frames any more —
 * see the note at the top of this file — so this reports how much stack has
 * been used, divided by the same frame estimate the limit is expressed in. It
 * answers the question the API is asked ("how deep am I, roughly") without
 * putting a counter back on every call.
 */
int64_t __desi_get_call_depth(void) {
    size_t size = 0;
    char* low = desi_stack_low_address(&size);
    if (!low || size == 0) return 0;
    char probe;
    char* high = low + size;
    if ((char*)&probe >= high) return 0;
    return (int64_t)((high - (char*)&probe) / DESI_FRAME_ESTIMATE);
}

/*
 * Get max recursion limit (for debugging)
 */
int64_t __desi_get_max_recursion(void) {
    return __max_recursion;
}
