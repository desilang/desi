/**
 * Desi Reference Counting Runtime
 *
 * Rc/Weak object layout:
 * - refcount at offset 0 (4 bytes)
 * - weakcount at offset 4 (4 bytes)
 * - inner pointer at offset 8 (8 bytes on 64-bit)
 *
 * Both rc[T] and arc[T] lower to these functions, so all counter updates
 * are atomic — arc[T] is documented as thread-safe and shares this path.
 *
 * Lifetime scheme (same as Rust's Arc): all strong references collectively
 * hold ONE weak count. The inner value is freed exactly once (when the last
 * strong ref drops) and the header is freed exactly once (when the last
 * weak ref drops, which includes the collective strong-held one). This
 * avoids the double-free/use-after-free race between __rc_dec and
 * __weak_dec that a "check the other counter after dropping mine" scheme
 * has under concurrency.
 */

#include <stdlib.h>
#include <stdint.h>

#ifdef _WIN32
#include <intrin.h>
#endif

// Rc object header - stored before the inner value
typedef struct {
    volatile int32_t refcount;
    volatile int32_t weakcount;
    void* inner;
} DesiRc;

/* ---- Atomic primitives (MSVC intrinsics / GCC-Clang builtins) ---- */

// Returns the value *before* the addition.
static int32_t rc_fetch_add(volatile int32_t* p, int32_t v) {
#ifdef _WIN32
    return (int32_t)_InterlockedExchangeAdd((volatile long*)p, (long)v);
#else
    return __atomic_fetch_add(p, v, __ATOMIC_ACQ_REL);
#endif
}

static int32_t rc_load(volatile int32_t* p) {
#ifdef _WIN32
    return (int32_t)_InterlockedExchangeAdd((volatile long*)p, 0);
#else
    return __atomic_load_n(p, __ATOMIC_ACQUIRE);
#endif
}

// Compare-and-swap; returns nonzero if *p was `expected` and is now `desired`.
static int rc_cas(volatile int32_t* p, int32_t expected, int32_t desired) {
#ifdef _WIN32
    return _InterlockedCompareExchange((volatile long*)p, (long)desired, (long)expected) == (long)expected;
#else
    return __atomic_compare_exchange_n(p, &expected, desired, 0,
                                       __ATOMIC_ACQ_REL, __ATOMIC_ACQUIRE);
#endif
}

/**
 * Create a new Rc wrapping the given inner pointer.
 * Initial refcount = 1; weakcount = 1 (the collective weak held by all
 * strong references — released when the last strong ref drops).
 */
void* __rc_new(void* inner) {
    DesiRc* rc = (DesiRc*)malloc(sizeof(DesiRc));
    if (rc == NULL) return NULL;
    rc->refcount = 1;
    rc->weakcount = 1;
    rc->inner = inner;
    return (void*)rc;
}

/**
 * Increment the reference count.
 */
void __rc_inc(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc_fetch_add(&rc->refcount, 1);
}

/**
 * Decrement the weak reference count; frees the header when it hits 0.
 * (Shared by __rc_dec's collective-weak release and weak-pointer drops.)
 */
void __weak_dec(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    if (rc_fetch_add(&rc->weakcount, -1) == 1) {
        free(rc);
    }
}

/**
 * Decrement the reference count.
 * The thread that drops the last strong ref frees the inner value, then
 * releases the collective weak count (which frees the header if no weak
 * pointers remain).
 */
void __rc_dec(void* rc_ptr) {
    if (rc_ptr == NULL) return;
    DesiRc* rc = (DesiRc*)rc_ptr;
    if (rc_fetch_add(&rc->refcount, -1) == 1) {
        if (rc->inner != NULL) {
            free(rc->inner);
            rc->inner = NULL;
        }
        __weak_dec(rc_ptr);
    }
}

/**
 * Clone an Rc - increments refcount and returns the same pointer.
 * This is how multiple owners share the same value.
 */
void* __rc_clone(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    __rc_inc(rc_ptr);
    return rc_ptr;
}

/**
 * Get the inner pointer from an Rc.
 */
void* __rc_get(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    return rc->inner;
}

/**
 * Get the current reference count (for debugging/testing).
 */
int32_t __rc_count(void* rc_ptr) {
    if (rc_ptr == NULL) return 0;
    DesiRc* rc = (DesiRc*)rc_ptr;
    return rc_load(&rc->refcount);
}

// === Weak Pointer Support ===

/**
 * Create a new Weak pointer from an Rc pointer.
 * Increments the weakcount and returns the same control block pointer.
 */
void* __weak_new(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    rc_fetch_add(&rc->weakcount, 1);
    return rc_ptr;
}

/**
 * Try to upgrade a Weak reference to a strong Rc.
 * Returns the rc_ptr if refcount > 0 (and increments refcount).
 * Returns NULL if the referenced object has already been deallocated.
 * CAS loop so the check and the increment are one atomic step — a plain
 * "if (refcount > 0) refcount++" races with a concurrent final __rc_dec.
 */
void* __weak_upgrade(void* rc_ptr) {
    if (rc_ptr == NULL) return NULL;
    DesiRc* rc = (DesiRc*)rc_ptr;
    for (;;) {
        int32_t cur = rc_load(&rc->refcount);
        if (cur <= 0) return NULL;
        if (rc_cas(&rc->refcount, cur, cur + 1)) {
            return rc_ptr;
        }
    }
}

/**
 * Get the current weak reference count (for debugging/testing).
 * Excludes the collective weak held by strong references.
 */
int32_t __weak_count(void* rc_ptr) {
    if (rc_ptr == NULL) return 0;
    DesiRc* rc = (DesiRc*)rc_ptr;
    int32_t w = rc_load(&rc->weakcount);
    if (rc_load(&rc->refcount) > 0) {
        w -= 1;
    }
    return w < 0 ? 0 : w;
}
