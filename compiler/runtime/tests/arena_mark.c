/* Standalone exercise of arena mark/rewind/release, compiled against arena.c.
 *
 * The three calls mirror how codegen brackets a loop:
 *
 *     __arena_mark(a);          before the loop
 *     ... loop body ...
 *     __arena_rewind(a);        at the latch, once per iteration
 *     __arena_release(a);       at the exit block, however the loop was left
 *
 * rewind deliberately leaves the mark in place, because the latch returns to it
 * on every pass. release retires it, and sits on the exit block so that
 * breaking out of the middle of the loop retires it too.
 *
 * The properties that matter:
 *   1. A rewind releases only what was allocated after the mark.
 *   2. A rewinding loop reuses the same bytes instead of growing every pass.
 *   3. Rewinding past a chunk boundary does not strand the chunks it crossed.
 *   4. Marks nest, and overflowing the mark stack stays paired and safe.
 *   5. Data written before a mark survives everything done after it.
 */
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

void*  __arena_new(size_t);
void*  __arena_alloc(void*, size_t);
void   __arena_mark(void*);
void   __arena_rewind(void*);
void   __arena_release(void*);
void   __arena_destroy(void*);
size_t __arena_used(void*);
size_t __arena_capacity(void*);

static int failures = 0;
static void check(const char* what, int ok) {
    printf("%-58s %s\n", what, ok ? "ok" : "FAILED");
    if (!ok) failures++;
}

int main(void) {
    void* a = __arena_new(0);

    /* 1. A rewind returns to the mark, not to empty. */
    __arena_alloc(a, 1000);              /* allocated before the loop */
    size_t before = __arena_used(a);
    __arena_mark(a);
    __arena_alloc(a, 4000);
    check("used grows after the mark", __arena_used(a) > before);
    __arena_rewind(a);
    check("rewind returns to exactly the marked position",
          __arena_used(a) == before);
    __arena_release(a);
    check("release leaves the arena at the marked position",
          __arena_used(a) == before);

    /* 2. The loop shape. One mark, a rewind per iteration, one release.
     *    Allocate far more in total than the arena holds. */
    size_t cap_after_first = 0;
    __arena_mark(a);
    for (int i = 0; i < 5000; i++) {
        for (int j = 0; j < 20; j++) __arena_alloc(a, 256);
        __arena_rewind(a);
        if (i == 0) cap_after_first = __arena_capacity(a);
    }
    __arena_release(a);
    check("capacity is stable across 5000 rewound iterations",
          __arena_capacity(a) == cap_after_first);
    check("used returns to the pre-loop figure", __arena_used(a) == before);

    /* 3. Rewinding across a chunk boundary. Force several chunks, rewind to a
     *    mark taken in the first, then refill: the crossed chunks must be
     *    reused rather than stranded. */
    __arena_mark(a);
    size_t cap_before_big = __arena_capacity(a);
    for (int i = 0; i < 400; i++) __arena_alloc(a, 1024); /* ~400KB */
    check("allocating past capacity added chunks",
          __arena_capacity(a) > cap_before_big);
    size_t cap_grown = __arena_capacity(a);
    __arena_rewind(a);
    check("rewind across chunks restores used", __arena_used(a) == before);
    for (int i = 0; i < 400; i++) __arena_alloc(a, 1024);
    check("refilling reuses the crossed chunks rather than allocating more",
          __arena_capacity(a) == cap_grown);
    __arena_release(a);

    /* 4. Nested loops: an inner mark/release pair inside an outer one. */
    size_t l0 = __arena_used(a);
    __arena_mark(a);                       /* outer loop */
    __arena_alloc(a, 500);
    size_t l1 = __arena_used(a);
    __arena_mark(a);                       /* inner loop */
    __arena_alloc(a, 500);
    __arena_rewind(a);
    check("inner rewind returns to the inner mark", __arena_used(a) == l1);
    __arena_release(a);                    /* inner loop exits */
    check("inner release does not disturb the outer position",
          __arena_used(a) == l1);
    __arena_rewind(a);
    check("outer rewind returns to the outer mark", __arena_used(a) == l0);
    __arena_release(a);

    /* 5. Breaking out of a loop: mark, allocate, leave without reaching the
     *    latch. The exit block still releases. */
    size_t b0 = __arena_used(a);
    for (int i = 0; i < 1000; i++) {
        __arena_mark(a);
        __arena_alloc(a, 128);
        /* no rewind -- pretend the body broke out on its first pass */
        __arena_release(a);
    }
    check("breaking out of a loop 1000 times does not drift",
          __arena_used(a) == b0);

    /* 6. Overflowing the mark stack stays paired: 100 nested marks, 100
     *    releases, and the arena ends where it started. */
    size_t deep0 = __arena_used(a);
    for (int i = 0; i < 100; i++) { __arena_mark(a); __arena_alloc(a, 64); }
    for (int i = 0; i < 100; i++) __arena_release(a);
    check("100 nested marks/releases stay paired and do not corrupt",
          __arena_used(a) == deep0);

    /* 7. Stray calls with no mark outstanding must not underflow. */
    __arena_rewind(a);
    __arena_release(a);
    __arena_release(a);
    check("stray rewind/release are harmless", __arena_used(a) == deep0);
    __arena_destroy(a);

    /* 8. Data integrity: values written before a mark are readable after a
     *    rewind, and the rewound region is genuinely handed out again. */
    void* b = __arena_new(0);
    int* keep = (int*)__arena_alloc(b, sizeof(int) * 4);
    for (int i = 0; i < 4; i++) keep[i] = 0xABC0 + i;
    __arena_mark(b);
    void* first = __arena_alloc(b, 64);
    __arena_rewind(b);
    void* second = __arena_alloc(b, 64);
    check("the rewound bytes are handed out again", first == second);
    int intact = 1;
    for (int i = 0; i < 4; i++) if (keep[i] != 0xABC0 + i) intact = 0;
    check("data allocated before the mark is untouched by the rewind", intact);
    __arena_release(b);
    __arena_destroy(b);

#ifdef DESI_ARENA_POISON
    /* The poison build has to actually poison, or a clean suite run under it
     * proves nothing. */
    void* c = __arena_new(0);
    __arena_mark(c);
    unsigned char* scratch = (unsigned char*)__arena_alloc(c, 32);
    memset(scratch, 0x11, 32);
    __arena_rewind(c);
    int poisoned = 1;
    for (int i = 0; i < 32; i++) if (scratch[i] != 0xDD) poisoned = 0;
    check("a rewind scribbles over the bytes it released", poisoned);
    __arena_release(c);
    __arena_destroy(c);
#endif

    printf("\n%s\n", failures ? "FAILURES" : "all arena properties hold");
    return failures ? 1 : 0;
}
