/*
 * Desi Runtime — Futures (async/await)
 *
 * Thread-based future implementation using pthreads.
 * An async def f(x) compiles to:
 *   1. f(x) → creates Future, spawns f_body on thread, returns Future ptr
 *   2. f_body(ctx) → unpacks args from ctx, runs body, calls __future_complete
 *   3. await fut → calls __await_blocking(fut) → blocks until complete
 *
 * All values are transported as int64_t (i64). Pointers (str, list, etc.)
 * are cast to intptr_t and back.
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include "platform.h"

// Set before any thread starts so print can skip its lock while the
// program is still single-threaded. See print.c.
extern void __desi_note_thread_start(void);

/* ---------- Future struct ---------- */

typedef struct {
    DesiPlatformMutex lock;
    DesiPlatformCond  done_cv;
    int               completed;
    int64_t           value;
    DesiPlatformThread thread;
    int               thread_started;
} DesiFuture;

/* ---------- API ---------- */

/*
 * Allocate and initialize a new Future.
 * Returns an opaque ptr.
 */
void* __future_new(void) {
    DesiFuture* f = (DesiFuture*)calloc(1, sizeof(DesiFuture));
    if (!f) {
        fprintf(stderr, "Desi panic: out of memory allocating future\n");
        exit(1);
    }
    DESI_MUTEX_INIT(f->lock);
    DESI_COND_INIT(f->done_cv);
    f->completed = 0;
    f->value = 0;
    f->thread_started = 0;
    return (void*)f;
}

/*
 * Complete a future with a value.
 * Called by the body function on the background thread.
 * Signals any thread blocked in __await_blocking.
 */
void __future_complete(void* fut, int64_t val) {
    DesiFuture* f = (DesiFuture*)fut;
    DESI_MUTEX_LOCK(f->lock);
    f->value = val;
    f->completed = 1;
    DESI_COND_SIGNAL(f->done_cv);
    DESI_MUTEX_UNLOCK(f->lock);
}

/*
 * Block until a future is completed, then return its value.
 * If already completed, returns immediately.
 */
int64_t __await_blocking(void* fut) {
    DesiFuture* f = (DesiFuture*)fut;
    DESI_MUTEX_LOCK(f->lock);
    while (!f->completed) {
        DESI_COND_WAIT(f->done_cv, f->lock);
    }
    int64_t val = f->value;
    DESI_MUTEX_UNLOCK(f->lock);

    /* Join the thread if it was started to clean up resources */
    if (f->thread_started) {
#ifdef _WIN32
        WaitForSingleObject(f->thread, INFINITE);
        CloseHandle(f->thread);
#else
        pthread_join(f->thread, NULL);
#endif
    }

    /* Free the future — after await, it's consumed */
    DESI_MUTEX_DESTROY(f->lock);
    DESI_COND_DESTROY(f->done_cv);
    free(f);

    return val;
}

/*
 * Context struct passed to the thread entry function.
 * Holds the future pointer, the body function pointer, and packed arguments.
 */
typedef struct {
    void*    future;
    void*    body_fn;
    int      argc;
    int64_t  args[];    /* flexible array of arguments */
} FutureSpawnCtx;

/*
 * Thread entry point.
 * Unpacks the context, calls the body function with args, then completes the future.
 * The body function signature varies by argc:
 *   0 args: int64_t body(void* future)
 *   1 arg:  int64_t body(void* future, int64_t a0)
 *   2 args: int64_t body(void* future, int64_t a0, int64_t a1)
 *   etc.
 */
#ifdef _WIN32
static DWORD WINAPI __future_thread_entry(LPVOID raw_ctx) {
#else
static void* __future_thread_entry(void* raw_ctx) {
#endif
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)raw_ctx;
    void* future = ctx->future;
    int64_t result = 0;

    /* Call body function based on argc */
    switch (ctx->argc) {
        case 0: {
            int64_t (*fn)(void*) = (int64_t (*)(void*))ctx->body_fn;
            result = fn(future);
            break;
        }
        case 1: {
            int64_t (*fn)(void*, int64_t) = (int64_t (*)(void*, int64_t))ctx->body_fn;
            result = fn(future, ctx->args[0]);
            break;
        }
        case 2: {
            int64_t (*fn)(void*, int64_t, int64_t) = (int64_t (*)(void*, int64_t, int64_t))ctx->body_fn;
            result = fn(future, ctx->args[0], ctx->args[1]);
            break;
        }
        case 3: {
            int64_t (*fn)(void*, int64_t, int64_t, int64_t) =
                (int64_t (*)(void*, int64_t, int64_t, int64_t))ctx->body_fn;
            result = fn(future, ctx->args[0], ctx->args[1], ctx->args[2]);
            break;
        }
        case 4: {
            int64_t (*fn)(void*, int64_t, int64_t, int64_t, int64_t) =
                (int64_t (*)(void*, int64_t, int64_t, int64_t, int64_t))ctx->body_fn;
            result = fn(future, ctx->args[0], ctx->args[1], ctx->args[2], ctx->args[3]);
            break;
        }
        default: {
            fprintf(stderr, "Desi panic: async function with %d args not supported (max 4)\n", ctx->argc);
            exit(1);
        }
    }

    __future_complete(future, result);
    free(ctx);
#ifdef _WIN32
    return 0;
#else
    return NULL;
#endif
}

/*
 * Spawn a body function on a background thread.
 * The body function receives (future_ptr, arg0, arg1, ...) and should
 * return the result as int64_t. __future_complete is called automatically.
 *
 * Usage from LLVM IR:
 *   call void @__future_spawn(ptr %future, ptr @body_fn, i32 argc, ...)
 *
 * But since varargs are complex in LLVM IR, we instead provide fixed-arity
 * spawn functions:
 */
void __future_spawn_0(void* future, void* body_fn) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 0;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    __desi_note_thread_start();
#ifdef _WIN32
    f->thread = CreateThread(NULL, 0, __future_thread_entry, ctx, 0, NULL);
#else
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
#endif
}

void __future_spawn_1(void* future, void* body_fn, int64_t a0) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx) + sizeof(int64_t));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 1;
    ctx->args[0] = a0;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    __desi_note_thread_start();
#ifdef _WIN32
    f->thread = CreateThread(NULL, 0, __future_thread_entry, ctx, 0, NULL);
#else
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
#endif
}

void __future_spawn_2(void* future, void* body_fn, int64_t a0, int64_t a1) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx) + 2 * sizeof(int64_t));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 2;
    ctx->args[0] = a0;
    ctx->args[1] = a1;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    __desi_note_thread_start();
#ifdef _WIN32
    f->thread = CreateThread(NULL, 0, __future_thread_entry, ctx, 0, NULL);
#else
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
#endif
}

void __future_spawn_3(void* future, void* body_fn, int64_t a0, int64_t a1, int64_t a2) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx) + 3 * sizeof(int64_t));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 3;
    ctx->args[0] = a0;
    ctx->args[1] = a1;
    ctx->args[2] = a2;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    __desi_note_thread_start();
#ifdef _WIN32
    f->thread = CreateThread(NULL, 0, __future_thread_entry, ctx, 0, NULL);
#else
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
#endif
}

void __future_spawn_4(void* future, void* body_fn, int64_t a0, int64_t a1, int64_t a2, int64_t a3) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx) + 4 * sizeof(int64_t));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 4;
    ctx->args[0] = a0;
    ctx->args[1] = a1;
    ctx->args[2] = a2;
    ctx->args[3] = a3;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    __desi_note_thread_start();
#ifdef _WIN32
    f->thread = CreateThread(NULL, 0, __future_thread_entry, ctx, 0, NULL);
#else
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
#endif
}

/*
 * Legacy: __future_register_poll — stub for old poll-based model.
 * The LLVM IR still references this during transition.
 */
void __future_register_poll(void* fut, void* poll_fn, void* frame) {
    (void)fut; (void)poll_fn; (void)frame;
    /* no-op — thread model doesn't use polling */
}
