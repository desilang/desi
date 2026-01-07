/*
 * Desi Runtime TaskGroup Implementation
 * Cross-platform: Uses platform.h macros for POSIX/Windows.
 */

#include "taskgroup.h"
#include <stdlib.h>
#include <stdio.h>

/* Forward declare scheduler functions */
extern void scheduler_spawn(void (*fn)(void* ctx), void* ctx, void* unused);
extern void scheduler_init(int n_workers);
extern int scheduler_num_workers(void);

/* Ensure scheduler is initialized (lazy init) */
static void ensure_scheduler_init(void) {
    static int initialized = 0;
    if (!initialized) {
        scheduler_init(4);  /* Default 4 workers */
        initialized = 1;
    }
}

#define INITIAL_CAPACITY 8

/*
 * Create a new task group
 */
TaskGroup* taskgroup_new(void) {
    /* Ensure scheduler is running */
    ensure_scheduler_init();
    TaskGroup* g = malloc(sizeof(TaskGroup));
    if (!g) return NULL;
    
    g->tasks = malloc(sizeof(Task*) * INITIAL_CAPACITY);
    if (!g->tasks) {
        free(g);
        return NULL;
    }
    
    g->capacity = INITIAL_CAPACITY;
    g->count = 0;
    g->pending = 0;
    g->cancelled = false;
    g->error = NULL;
    
    DESI_MUTEX_INIT(g->lock);
    DESI_COND_INIT(g->all_done);
    
    return g;
}

/*
 * Wrapper task function that notifies group on completion
 */
typedef struct {
    TaskGroup* group;
    void (*fn)(void* ctx);
    void* ctx;
} GroupTaskCtx;

static void group_task_wrapper(void* arg) {
    GroupTaskCtx* gtc = (GroupTaskCtx*)arg;
    TaskGroup* g = gtc->group;
    
    /* Check for cancellation before running */
    if (!taskgroup_is_cancelled(g)) {
        gtc->fn(gtc->ctx);
    }
    
    /* Mark task as done */
    DESI_MUTEX_LOCK(g->lock);
    g->pending--;
    if (g->pending == 0) {
        DESI_COND_BROADCAST(g->all_done);
    }
    DESI_MUTEX_UNLOCK(g->lock);
    
    free(gtc);
}

/*
 * Spawn a task in this group
 */
void taskgroup_spawn(TaskGroup* g, void (*fn)(void* ctx), void* ctx) {
    if (!g || !fn) return;
    
    DESI_MUTEX_LOCK(g->lock);
    
    /* Expand array if needed */
    if (g->count >= g->capacity) {
        int64_t new_cap = g->capacity * 2;
        Task** new_tasks = realloc(g->tasks, sizeof(Task*) * new_cap);
        if (!new_tasks) {
            DESI_MUTEX_UNLOCK(g->lock);
            return;
        }
        g->tasks = new_tasks;
        g->capacity = new_cap;
    }
    
    g->count++;
    g->pending++;
    
    DESI_MUTEX_UNLOCK(g->lock);
    
    /* Create wrapper context */
    GroupTaskCtx* gtc = malloc(sizeof(GroupTaskCtx));
    if (!gtc) {
        DESI_MUTEX_LOCK(g->lock);
        g->count--;
        g->pending--;
        DESI_MUTEX_UNLOCK(g->lock);
        return;
    }
    
    gtc->group = g;
    gtc->fn = fn;
    gtc->ctx = ctx;
    
    /* Spawn through the scheduler */
    scheduler_spawn(group_task_wrapper, gtc, NULL);
}

/*
 * Wait for all tasks in the group to complete
 */
void taskgroup_wait(TaskGroup* g) {
    if (!g) return;
    
    DESI_MUTEX_LOCK(g->lock);
    while (g->pending > 0) {
        DESI_COND_WAIT(g->all_done, g->lock);
    }
    DESI_MUTEX_UNLOCK(g->lock);
}

/*
 * Cancel all tasks in the group
 */
void taskgroup_cancel(TaskGroup* g) {
    if (!g) return;
    
    DESI_MUTEX_LOCK(g->lock);
    g->cancelled = true;
    DESI_MUTEX_UNLOCK(g->lock);
    
    /* Note: Tasks check is_cancelled() at their own await points */
}

/*
 * Check if cancelled
 */
bool taskgroup_is_cancelled(TaskGroup* g) {
    if (!g) return true;
    
    DESI_MUTEX_LOCK(g->lock);
    bool cancelled = g->cancelled;
    DESI_MUTEX_UNLOCK(g->lock);
    
    return cancelled;
}

/*
 * Set an error (first error wins)
 */
void taskgroup_set_error(TaskGroup* g, void* error) {
    if (!g || !error) return;
    
    DESI_MUTEX_LOCK(g->lock);
    if (!g->error) {
        g->error = error;
    }
    DESI_MUTEX_UNLOCK(g->lock);
}

/*
 * Get the first error
 */
void* taskgroup_get_error(TaskGroup* g) {
    if (!g) return NULL;
    
    DESI_MUTEX_LOCK(g->lock);
    void* error = g->error;
    DESI_MUTEX_UNLOCK(g->lock);
    
    return error;
}

/*
 * Called when a task completes
 */
void taskgroup_task_done(TaskGroup* g, Task* t) {
    if (!g) return;
    
    DESI_MUTEX_LOCK(g->lock);
    g->pending--;
    if (g->pending == 0) {
        DESI_COND_BROADCAST(g->all_done);
    }
    DESI_MUTEX_UNLOCK(g->lock);
}

/*
 * Destroy the task group
 */
void taskgroup_destroy(TaskGroup* g) {
    if (!g) return;
    
    /* Wait for any remaining tasks */
    taskgroup_wait(g);
    
    DESI_MUTEX_DESTROY(g->lock);
    DESI_COND_DESTROY(g->all_done);
    
    if (g->tasks) {
        free(g->tasks);
    }
    free(g);
}
