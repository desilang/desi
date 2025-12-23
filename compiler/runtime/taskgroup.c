/*
 * Desi Runtime TaskGroup Implementation
 */

#include "taskgroup.h"
#include "scheduler.h"
#include <stdlib.h>
#include <stdio.h>

#define INITIAL_CAPACITY 8

/*
 * Create a new task group
 */
TaskGroup* taskgroup_new(void) {
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
    
    pthread_mutex_init(&g->lock, NULL);
    pthread_cond_init(&g->all_done, NULL);
    
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
    pthread_mutex_lock(&g->lock);
    g->pending--;
    if (g->pending == 0) {
        pthread_cond_broadcast(&g->all_done);
    }
    pthread_mutex_unlock(&g->lock);
    
    free(gtc);
}

/*
 * Spawn a task in this group
 */
void taskgroup_spawn(TaskGroup* g, void (*fn)(void* ctx), void* ctx) {
    if (!g || !fn) return;
    
    pthread_mutex_lock(&g->lock);
    
    /* Expand array if needed */
    if (g->count >= g->capacity) {
        int64_t new_cap = g->capacity * 2;
        Task** new_tasks = realloc(g->tasks, sizeof(Task*) * new_cap);
        if (!new_tasks) {
            pthread_mutex_unlock(&g->lock);
            return;
        }
        g->tasks = new_tasks;
        g->capacity = new_cap;
    }
    
    g->count++;
    g->pending++;
    
    pthread_mutex_unlock(&g->lock);
    
    /* Create wrapper context */
    GroupTaskCtx* gtc = malloc(sizeof(GroupTaskCtx));
    if (!gtc) {
        pthread_mutex_lock(&g->lock);
        g->count--;
        g->pending--;
        pthread_mutex_unlock(&g->lock);
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
    
    pthread_mutex_lock(&g->lock);
    while (g->pending > 0) {
        pthread_cond_wait(&g->all_done, &g->lock);
    }
    pthread_mutex_unlock(&g->lock);
}

/*
 * Cancel all tasks in the group
 */
void taskgroup_cancel(TaskGroup* g) {
    if (!g) return;
    
    pthread_mutex_lock(&g->lock);
    g->cancelled = true;
    pthread_mutex_unlock(&g->lock);
    
    /* Note: Tasks check is_cancelled() at their own await points */
}

/*
 * Check if cancelled
 */
bool taskgroup_is_cancelled(TaskGroup* g) {
    if (!g) return true;
    
    pthread_mutex_lock(&g->lock);
    bool cancelled = g->cancelled;
    pthread_mutex_unlock(&g->lock);
    
    return cancelled;
}

/*
 * Set an error (first error wins)
 */
void taskgroup_set_error(TaskGroup* g, void* error) {
    if (!g || !error) return;
    
    pthread_mutex_lock(&g->lock);
    if (!g->error) {
        g->error = error;
    }
    pthread_mutex_unlock(&g->lock);
}

/*
 * Get the first error
 */
void* taskgroup_get_error(TaskGroup* g) {
    if (!g) return NULL;
    
    pthread_mutex_lock(&g->lock);
    void* error = g->error;
    pthread_mutex_unlock(&g->lock);
    
    return error;
}

/*
 * Called when a task completes
 */
void taskgroup_task_done(TaskGroup* g, Task* t) {
    if (!g) return;
    
    pthread_mutex_lock(&g->lock);
    g->pending--;
    if (g->pending == 0) {
        pthread_cond_broadcast(&g->all_done);
    }
    pthread_mutex_unlock(&g->lock);
}

/*
 * Destroy the task group
 */
void taskgroup_destroy(TaskGroup* g) {
    if (!g) return;
    
    /* Wait for any remaining tasks */
    taskgroup_wait(g);
    
    pthread_mutex_destroy(&g->lock);
    pthread_cond_destroy(&g->all_done);
    
    if (g->tasks) {
        free(g->tasks);
    }
    free(g);
}
