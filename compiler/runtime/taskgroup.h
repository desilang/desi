/*
 * Desi Runtime TaskGroup Implementation
 * 
 * Structured concurrency: all spawned tasks complete before scope exits.
 * Provides automatic cleanup and cancellation propagation.
 */

#ifndef DESI_TASKGROUP_H
#define DESI_TASKGROUP_H

#include <pthread.h>
#include <stdbool.h>
#include <stdint.h>

/* Forward declare from scheduler.h */
typedef struct Task Task;

/*
 * TaskGroup manages a collection of spawned tasks.
 * All tasks in the group must complete before the group can be destroyed.
 */
typedef struct TaskGroup {
    Task** tasks;               /* Array of spawned tasks */
    int64_t capacity;           /* Allocated size */
    int64_t count;              /* Number of tasks */
    
    int64_t pending;            /* Number of tasks not yet completed */
    pthread_mutex_t lock;       /* Protects all fields */
    pthread_cond_t all_done;    /* Signaled when pending reaches 0 */
    
    bool cancelled;             /* Cancellation flag */
    void* error;                /* First error from child task (if any) */
} TaskGroup;

/* Create a new task group */
TaskGroup* taskgroup_new(void);

/* Spawn a task in this group */
void taskgroup_spawn(TaskGroup* g, void (*fn)(void* ctx), void* ctx);

/* Wait for all tasks in the group to complete */
void taskgroup_wait(TaskGroup* g);

/* Cancel all tasks in the group */
void taskgroup_cancel(TaskGroup* g);

/* Check if the group has been cancelled */
bool taskgroup_is_cancelled(TaskGroup* g);

/* Set an error in the group (called by child task on failure) */
void taskgroup_set_error(TaskGroup* g, void* error);

/* Get the first error (if any) */
void* taskgroup_get_error(TaskGroup* g);

/* Destroy the task group (must call wait first) */
void taskgroup_destroy(TaskGroup* g);

/* Called by a task when it completes (internal) */
void taskgroup_task_done(TaskGroup* g, Task* t);

#endif /* DESI_TASKGROUP_H */
