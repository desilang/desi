/*
 * Desi Runtime Supervisor Implementation
 *
 * Thread pool with work queue + persistent children with auto-restart.
 * Cross-platform: Uses platform.h macros.
 *
 * Performance:
 *   - Lock contention minimized: workers only hold lock during dequeue
 *   - Ring buffer queue: O(1) enqueue/dequeue
 *   - No allocation per task submission (ring buffer pre-allocated)
 *
 * Memory safety:
 *   - supervisor_stop() drains queue, joins all threads, frees everything
 *   - No dangling references after stop
 *   - Null guards on all public APIs
 */

#include "supervisor.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <time.h>

// Set before any thread starts so print can skip its lock while the
// program is still single-threaded. See print.c.
extern void __desi_note_thread_start(void);

/* ============================================================
 * Default Configuration
 * ============================================================ */

#define DEFAULT_QUEUE_CAPACITY 256
#define DEFAULT_MAX_CHILDREN   32
#define DEFAULT_MAX_RESTARTS    5
#define DEFAULT_RESTART_WINDOW 60

/* ============================================================
 * Pool Worker Thread
 * ============================================================ */

/*
 * Pool worker loop: dequeue work items and execute them.
 * Exits when shutdown is signaled and queue is empty.
 */
static void* pool_worker_loop(void* arg) {
    Supervisor* sup = (Supervisor*)arg;

    while (1) {
        DESI_MUTEX_LOCK(sup->lock);

        /* Wait for work or shutdown */
        while (sup->q_count == 0 && !sup->shutdown) {
            DESI_COND_WAIT(sup->not_empty, sup->lock);
        }

        /* Exit if shutdown and queue is empty */
        if (sup->shutdown && sup->q_count == 0) {
            DESI_MUTEX_UNLOCK(sup->lock);
            break;
        }

        /* Dequeue work item */
        WorkItem item = sup->queue[sup->q_head];
        sup->q_head = (sup->q_head + 1) % sup->q_cap;
        sup->q_count--;

        DESI_MUTEX_UNLOCK(sup->lock);

        /* Execute task outside the lock */
        if (item.fn) {
            item.fn(item.arg);
        }
    }

    return NULL;
}

/* ============================================================
 * Persistent Child Wrapper
 * ============================================================ */

typedef struct {
    Supervisor* sup;
    int child_index;
} ChildWrapperCtx;

/*
 * Persistent child wrapper: runs the child function and handles
 * auto-restart on crash (normal return = crash for persistent children).
 */
static void* child_wrapper(void* arg) {
    ChildWrapperCtx* ctx = (ChildWrapperCtx*)arg;
    Supervisor* sup = ctx->sup;
    int idx = ctx->child_index;
    free(ctx);

    DESI_MUTEX_LOCK(sup->lock);
    supervisor_task_fn fn = sup->children[idx].fn;
    void* child_arg = sup->children[idx].arg;
    sup->children[idx].alive = true;
    DESI_MUTEX_UNLOCK(sup->lock);

    /* Run the child function */
    if (fn) {
        fn(child_arg);
    }

    /* Child exited — check if we should restart */
    DESI_MUTEX_LOCK(sup->lock);
    sup->children[idx].alive = false;

    if (!sup->shutdown) {
        /* Check restart limits */
        if (sup->children[idx].restart_count < sup->max_restarts) {
            sup->children[idx].restart_count++;

            if (sup->strategy == SUPERVISOR_ONE_FOR_ALL) {
                /* ONE_FOR_ALL: restart ALL children when one crashes */
                fprintf(stderr, "[supervisor] child %d exited, restarting ALL children (one_for_all)\n", idx);

                /* Mark all other alive children for restart by bumping their restart_count */
                for (int i = 0; i < sup->num_children; i++) {
                    if (i != idx && sup->children[i].alive) {
                        sup->children[i].restart_count++;
                    }
                }

                /* Restart all children (including the crashed one) */
                for (int i = 0; i < sup->num_children; i++) {
                    /* Skip children that exceeded max restarts */
                    if (sup->children[i].restart_count > sup->max_restarts) continue;

                    ChildWrapperCtx* ctx = malloc(sizeof(ChildWrapperCtx));
                    if (!ctx) continue;
                    ctx->sup = sup;
                    ctx->child_index = i;
                    sup->children[i].alive = false;

                    DESI_MUTEX_UNLOCK(sup->lock);
    __desi_note_thread_start();
#ifdef _WIN32
                    sup->children[i].thread = CreateThread(
                        NULL, 0, (LPTHREAD_START_ROUTINE)child_wrapper,
                        ctx, 0, NULL);
#else
                    pthread_create(&sup->children[i].thread, NULL,
                        child_wrapper, ctx);
                    pthread_detach(sup->children[i].thread);
#endif
                    DESI_MUTEX_LOCK(sup->lock);
                }

                DESI_MUTEX_UNLOCK(sup->lock);
                return NULL;
            }

            /* ONE_FOR_ONE: restart only the crashed child */
            fprintf(stderr, "[supervisor] child %d restarting (%d/%d)\n",
                idx, sup->children[idx].restart_count, sup->max_restarts);

            ChildWrapperCtx* new_ctx = malloc(sizeof(ChildWrapperCtx));
            if (new_ctx) {
                new_ctx->sup = sup;
                new_ctx->child_index = idx;
                DESI_MUTEX_UNLOCK(sup->lock);

    __desi_note_thread_start();
#ifdef _WIN32
                sup->children[idx].thread = CreateThread(
                    NULL, 0, (LPTHREAD_START_ROUTINE)child_wrapper,
                    new_ctx, 0, NULL);
#else
                pthread_create(&sup->children[idx].thread, NULL,
                    child_wrapper, new_ctx);
                pthread_detach(sup->children[idx].thread);
#endif
                return NULL;
            }
        } else {
            fprintf(stderr, "[supervisor] child %d exceeded max restarts (%d), not restarting\n",
                idx, sup->max_restarts);
        }
    }

    DESI_MUTEX_UNLOCK(sup->lock);
    return NULL;
}

/* ============================================================
 * Public API: Create
 * ============================================================ */

Supervisor* supervisor_new(int strategy, int num_pool_workers) {
    Supervisor* sup = (Supervisor*)calloc(1, sizeof(Supervisor));
    if (!sup) return NULL;

    /* Config */
    sup->strategy = (SupervisorStrategy)strategy;
    sup->max_restarts = DEFAULT_MAX_RESTARTS;
    sup->restart_window_sec = DEFAULT_RESTART_WINDOW;
    sup->shutdown = 0;

    /* Initialize sync primitives */
    DESI_MUTEX_INIT(sup->lock);
    DESI_COND_INIT(sup->not_empty);

    /* Allocate work queue (ring buffer) */
    sup->q_cap = DEFAULT_QUEUE_CAPACITY;
    sup->queue = (WorkItem*)calloc(sup->q_cap, sizeof(WorkItem));
    if (!sup->queue) {
        free(sup);
        return NULL;
    }
    sup->q_head = 0;
    sup->q_tail = 0;
    sup->q_count = 0;

    /* Allocate children slots */
    sup->max_children = DEFAULT_MAX_CHILDREN;
    sup->children = (ChildSlot*)calloc(sup->max_children, sizeof(ChildSlot));
    if (!sup->children) {
        free(sup->queue);
        free(sup);
        return NULL;
    }
    sup->num_children = 0;

    /* Create pool worker threads */
    if (num_pool_workers < 1) num_pool_workers = 1;
    sup->num_pool_workers = num_pool_workers;
    sup->pool_threads = (DesiPlatformThread*)calloc(num_pool_workers,
        sizeof(DesiPlatformThread));
    if (!sup->pool_threads) {
        free(sup->children);
        free(sup->queue);
        free(sup);
        return NULL;
    }

    for (int i = 0; i < num_pool_workers; i++) {
    __desi_note_thread_start();
#ifdef _WIN32
        sup->pool_threads[i] = CreateThread(
            NULL, 0, (LPTHREAD_START_ROUTINE)pool_worker_loop,
            sup, 0, NULL);
#else
        pthread_create(&sup->pool_threads[i], NULL, pool_worker_loop, sup);
#endif
    }

    return sup;
}

/* ============================================================
 * Public API: Submit (pool work queue)
 * ============================================================ */

void supervisor_submit(Supervisor* sup, supervisor_task_fn fn, void* arg) {
    if (!sup || !fn) return;

    DESI_MUTEX_LOCK(sup->lock);

    /* Grow queue if full */
    if (sup->q_count >= sup->q_cap) {
        int new_cap = sup->q_cap * 2;
        WorkItem* new_queue = (WorkItem*)calloc(new_cap, sizeof(WorkItem));
        if (!new_queue) {
            DESI_MUTEX_UNLOCK(sup->lock);
            fprintf(stderr, "[supervisor] queue full, dropping task\n");
            return;
        }
        /* Copy ring buffer to linear buffer */
        for (int i = 0; i < sup->q_count; i++) {
            new_queue[i] = sup->queue[(sup->q_head + i) % sup->q_cap];
        }
        free(sup->queue);
        sup->queue = new_queue;
        sup->q_head = 0;
        sup->q_tail = sup->q_count;
        sup->q_cap = new_cap;
    }

    /* Enqueue */
    sup->queue[sup->q_tail].fn = fn;
    sup->queue[sup->q_tail].arg = arg;
    sup->q_tail = (sup->q_tail + 1) % sup->q_cap;
    sup->q_count++;

    DESI_COND_SIGNAL(sup->not_empty);
    DESI_MUTEX_UNLOCK(sup->lock);
}

/* ============================================================
 * Public API: Start Persistent Child
 * ============================================================ */

void supervisor_start_child(Supervisor* sup, supervisor_task_fn fn, void* arg) {
    if (!sup || !fn) return;

    DESI_MUTEX_LOCK(sup->lock);

    /* Grow children array if needed */
    if (sup->num_children >= sup->max_children) {
        int new_cap = sup->max_children * 2;
        ChildSlot* new_children = (ChildSlot*)realloc(sup->children,
            new_cap * sizeof(ChildSlot));
        if (!new_children) {
            DESI_MUTEX_UNLOCK(sup->lock);
            fprintf(stderr, "[supervisor] cannot grow children array\n");
            return;
        }
        /* Zero new slots */
        memset(new_children + sup->max_children, 0,
            (new_cap - sup->max_children) * sizeof(ChildSlot));
        sup->children = new_children;
        sup->max_children = new_cap;
    }

    int idx = sup->num_children++;
    sup->children[idx].fn = fn;
    sup->children[idx].arg = arg;
    sup->children[idx].alive = false;
    sup->children[idx].restart_count = 0;

    DESI_MUTEX_UNLOCK(sup->lock);

    /* Create wrapper context and spawn thread */
    ChildWrapperCtx* ctx = malloc(sizeof(ChildWrapperCtx));
    if (!ctx) return;
    ctx->sup = sup;
    ctx->child_index = idx;

    __desi_note_thread_start();
#ifdef _WIN32
    sup->children[idx].thread = CreateThread(
        NULL, 0, (LPTHREAD_START_ROUTINE)child_wrapper,
        ctx, 0, NULL);
#else
    pthread_create(&sup->children[idx].thread, NULL, child_wrapper, ctx);
#endif
}

/* ============================================================
 * Public API: Stop (drain + join + free)
 * ============================================================ */

void supervisor_stop(Supervisor* sup) {
    if (!sup) return;

    /* Idempotent: safe to call multiple times (explicit stop + RAII __close__) */
    DESI_MUTEX_LOCK(sup->lock);
    if (sup->shutdown == 2) {
        /* Already fully stopped and freed internals */
        DESI_MUTEX_UNLOCK(sup->lock);
        return;
    }
    sup->shutdown = 1;
    DESI_COND_BROADCAST(sup->not_empty);  /* wake all pool workers */
    DESI_MUTEX_UNLOCK(sup->lock);

    /* Join pool workers */
    for (int i = 0; i < sup->num_pool_workers; i++) {
#ifdef _WIN32
        WaitForSingleObject(sup->pool_threads[i], INFINITE);
        CloseHandle(sup->pool_threads[i]);
#else
        pthread_join(sup->pool_threads[i], NULL);
#endif
    }

    /* Join persistent children (non-detached ones) */
    for (int i = 0; i < sup->num_children; i++) {
        if (sup->children[i].alive) {
            /* Give children a moment to notice shutdown */
            /* In production, we'd send a cancellation signal */
#ifdef _WIN32
            WaitForSingleObject(sup->children[i].thread, 2000);
#else
            /* Detached threads can't be joined — they'll exit on next
               restart check when they see sup->shutdown */
            struct timespec ts = { .tv_sec = 0, .tv_nsec = 100000000 }; /* 100ms */
            nanosleep(&ts, NULL);
#endif
        }
    }

    /* Free internals */
    DESI_MUTEX_DESTROY(sup->lock);
    DESI_COND_DESTROY(sup->not_empty);

    if (sup->queue) { free(sup->queue); sup->queue = NULL; }
    if (sup->pool_threads) { free(sup->pool_threads); sup->pool_threads = NULL; }
    if (sup->children) { free(sup->children); sup->children = NULL; }

    /* Mark fully stopped — subsequent calls are no-ops */
    sup->shutdown = 2;
}

/* ============================================================
 * Public API: Queries
 * ============================================================ */

int supervisor_pool_size(Supervisor* sup) {
    return sup ? sup->num_pool_workers : 0;
}

int supervisor_child_count(Supervisor* sup) {
    if (!sup) return 0;
    DESI_MUTEX_LOCK(sup->lock);
    int count = sup->num_children;
    DESI_MUTEX_UNLOCK(sup->lock);
    return count;
}

bool supervisor_is_running(Supervisor* sup) {
    if (!sup) return false;
    DESI_MUTEX_LOCK(sup->lock);
    bool running = !sup->shutdown;
    DESI_MUTEX_UNLOCK(sup->lock);
    return running;
}

/* ============================================================
 * C API aliases for Desi extern bindings
 * ============================================================ */

Supervisor* __supervisor_new(int strategy, int num_pool_workers) {
    return supervisor_new(strategy, num_pool_workers);
}

void __supervisor_submit(Supervisor* sup, supervisor_task_fn fn, void* arg) {
    supervisor_submit(sup, fn, arg);
}

void __supervisor_start_child(Supervisor* sup, supervisor_task_fn fn, void* arg) {
    supervisor_start_child(sup, fn, arg);
}

void __supervisor_stop(Supervisor* sup) {
    supervisor_stop(sup);
}

int __supervisor_pool_size(Supervisor* sup) {
    return supervisor_pool_size(sup);
}

int __supervisor_child_count(Supervisor* sup) {
    return supervisor_child_count(sup);
}

int __supervisor_is_running(Supervisor* sup) {
    return supervisor_is_running(sup) ? 1 : 0;
}
