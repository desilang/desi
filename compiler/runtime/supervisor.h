/*
 * Desi Runtime Supervisor
 *
 * General-purpose supervisor with thread pool, work queue, and auto-restart.
 * Inspired by Elixir/OTP supervision trees.
 *
 * Strategies:
 *   ONE_FOR_ONE  - restart only the crashed worker
 *   ONE_FOR_ALL  - restart all workers if one crashes
 *
 * Usage as worker pool:
 *   Supervisor* sup = supervisor_new(SUPERVISOR_ONE_FOR_ONE, 4);
 *   supervisor_submit(sup, my_fn, my_arg);  // enqueue work
 *   supervisor_stop(sup);                    // drain + join + free
 *
 * Usage as persistent children:
 *   Supervisor* sup = supervisor_new(SUPERVISOR_ONE_FOR_ONE, 4);
 *   supervisor_start_child(sup, worker_fn, ctx);   // long-running
 *   supervisor_stop(sup);                           // signal + join + free
 *
 * Cross-platform: Uses platform.h macros for POSIX/Windows.
 */

#ifndef DESI_SUPERVISOR_H
#define DESI_SUPERVISOR_H

#include "platform.h"
#include <stdbool.h>
#include <stdint.h>

/* ---- Restart Strategies ---- */

typedef enum {
    SUPERVISOR_ONE_FOR_ONE = 0,   /* restart only crashed child */
    SUPERVISOR_ONE_FOR_ALL = 1    /* restart all if one crashes */
} SupervisorStrategy;

/* ---- Task function type ---- */

typedef void (*supervisor_task_fn)(void* arg);

/* ---- Work Queue Item ---- */

typedef struct {
    supervisor_task_fn fn;
    void* arg;
} WorkItem;

/* ---- Child Slot (for persistent children) ---- */

typedef struct {
    supervisor_task_fn fn;       /* function to run */
    void* arg;                   /* argument */
    DesiPlatformThread thread;   /* thread handle */
    bool alive;                  /* is thread running? */
    int restart_count;           /* times restarted */
} ChildSlot;

/* ---- Supervisor ---- */

typedef struct Supervisor {
    /* Work queue (for pool-style submit) */
    WorkItem* queue;
    int q_cap;
    int q_head;
    int q_tail;
    int q_count;

    /* Pool workers (dequeue + execute tasks) */
    DesiPlatformThread* pool_threads;
    int num_pool_workers;

    /* Persistent children (long-running, auto-restarted) */
    ChildSlot* children;
    int num_children;
    int max_children;

    /* Config */
    SupervisorStrategy strategy;
    int max_restarts;            /* max restarts per window (default 5) */
    int restart_window_sec;      /* time window in seconds (default 60) */

    /* Sync */
    DesiPlatformMutex lock;
    DesiPlatformCond  not_empty; /* signal pool workers when work arrives */
    int shutdown;                /* 1 = shutting down */
} Supervisor;

/* ---- Public API ---- */

/* Create a new supervisor with strategy and pool size */
Supervisor* supervisor_new(int strategy, int num_pool_workers);

/* Submit a task to the work queue (pool workers pick it up) */
void supervisor_submit(Supervisor* sup, supervisor_task_fn fn, void* arg);

/* Start a persistent child (auto-restarted on crash) */
void supervisor_start_child(Supervisor* sup, supervisor_task_fn fn, void* arg);

/* Stop the supervisor: drain queue, signal shutdown, join all threads, free */
void supervisor_stop(Supervisor* sup);

/* Get number of active pool workers */
int supervisor_pool_size(Supervisor* sup);

/* Get number of persistent children */
int supervisor_child_count(Supervisor* sup);

/* Check if supervisor is running */
bool supervisor_is_running(Supervisor* sup);

#endif /* DESI_SUPERVISOR_H */
