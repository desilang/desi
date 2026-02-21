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
#include <pthread.h>
#include <stdint.h>

/* ---------- Future struct ---------- */

typedef struct {
    pthread_mutex_t lock;
    pthread_cond_t  done_cv;
    int             completed;
    int64_t         value;       /* result (i64, or intptr_t for ptrs) */
    pthread_t       thread;
    int             thread_started;
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
    pthread_mutex_init(&f->lock, NULL);
    pthread_cond_init(&f->done_cv, NULL);
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
    pthread_mutex_lock(&f->lock);
    f->value = val;
    f->completed = 1;
    pthread_cond_signal(&f->done_cv);
    pthread_mutex_unlock(&f->lock);
}

/*
 * Block until a future is completed, then return its value.
 * If already completed, returns immediately.
 */
int64_t __await_blocking(void* fut) {
    DesiFuture* f = (DesiFuture*)fut;
    pthread_mutex_lock(&f->lock);
    while (!f->completed) {
        pthread_cond_wait(&f->done_cv, &f->lock);
    }
    int64_t val = f->value;
    pthread_mutex_unlock(&f->lock);

    /* Join the thread if it was started to clean up resources */
    if (f->thread_started) {
        pthread_join(f->thread, NULL);
    }

    /* Free the future — after await, it's consumed */
    pthread_mutex_destroy(&f->lock);
    pthread_cond_destroy(&f->done_cv);
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
static void* __future_thread_entry(void* raw_ctx) {
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
    return NULL;
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
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
}

void __future_spawn_1(void* future, void* body_fn, int64_t a0) {
    FutureSpawnCtx* ctx = (FutureSpawnCtx*)malloc(sizeof(FutureSpawnCtx) + sizeof(int64_t));
    ctx->future = future;
    ctx->body_fn = body_fn;
    ctx->argc = 1;
    ctx->args[0] = a0;
    DesiFuture* f = (DesiFuture*)future;
    f->thread_started = 1;
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
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
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
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
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
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
    pthread_create(&f->thread, NULL, __future_thread_entry, ctx);
}

/*
 * Legacy: __future_register_poll — stub for old poll-based model.
 * The LLVM IR still references this during transition.
 */
void __future_register_poll(void* fut, void* poll_fn, void* frame) {
    (void)fut; (void)poll_fn; (void)frame;
    /* no-op — thread model doesn't use polling */
}
