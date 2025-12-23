/*
 * Desi Runtime Scheduler
 * 
 * M:N threading model: many tasks mapped to few OS threads.
 * Implements a work-stealing scheduler for lightweight concurrency.
 */

#ifndef DESI_SCHEDULER_H
#define DESI_SCHEDULER_H

#include <stdint.h>
#include <stdbool.h>
#include <pthread.h>

/* Task states */
typedef enum {
    TASK_READY,      /* Waiting to run */
    TASK_RUNNING,    /* Currently executing */
    TASK_SUSPENDED,  /* Waiting on I/O or lock */
    TASK_DONE        /* Completed */
} TaskState;

/* A task is a unit of concurrent work */
typedef struct Task {
    void (*fn)(void* ctx);     /* Function to execute */
    void* ctx;                  /* Context/closure data */
    TaskState state;            /* Current state */
    struct Task* next;          /* For queue linking */
    char* name;                 /* Optional debug name */
    bool cancelled;             /* Cancellation flag */
} Task;

/* Thread-local work queue (lock-free deque would be better, but start simple) */
typedef struct WorkQueue {
    Task* head;
    Task* tail;
    pthread_mutex_t lock;
    pthread_cond_t not_empty;
    int count;
} WorkQueue;

/* A worker is an OS thread that executes tasks */
typedef struct Worker {
    pthread_t thread;
    WorkQueue local_queue;
    int id;
    bool running;
} Worker;

/* Global scheduler state */
typedef struct Scheduler {
    Worker* workers;
    int n_workers;
    WorkQueue global_queue;
    bool running;
    pthread_mutex_t lock;
} Scheduler;

/* Initialize the scheduler with n_workers OS threads */
void scheduler_init(int n_workers);

/* Shutdown the scheduler and wait for all tasks */
void scheduler_shutdown(void);

/* Spawn a new task (adds to current worker's queue or global queue) */
void scheduler_spawn(void (*fn)(void* ctx), void* ctx, const char* name);

/* Spawn a task from a Task struct */
void scheduler_spawn_task(Task* task);

/* Run the scheduler's main loop (called internally by workers) */
void scheduler_run_worker(Worker* w);

/* Get current task (for cancellation checks) */
Task* scheduler_current_task(void);

/* Check if current task is cancelled */
bool scheduler_is_cancelled(void);

/* Yield execution to other tasks */
void scheduler_yield(void);

/* Wait for all spawned tasks to complete (barrier) */
void scheduler_wait_all(void);

/* Get number of workers */
int scheduler_num_workers(void);

#endif /* DESI_SCHEDULER_H */
