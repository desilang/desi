/*
 * Desi Runtime Scheduler Implementation
 * 
 * M:N threading with work-stealing for lightweight concurrency.
 */

#include "scheduler.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

/* Global scheduler instance */
static Scheduler g_scheduler = {0};

/* Thread-local current task */
static __thread Task* current_task = NULL;
static __thread Worker* current_worker = NULL;

/* Forward declarations */
static void* worker_thread_fn(void* arg);
static Task* steal_task(Worker* thief);
static void workqueue_init(WorkQueue* q);
static void workqueue_push(WorkQueue* q, Task* t);
static Task* workqueue_pop(WorkQueue* q);
static Task* workqueue_steal(WorkQueue* q);

/*
 * Initialize work queue
 */
static void workqueue_init(WorkQueue* q) {
    q->head = NULL;
    q->tail = NULL;
    q->count = 0;
    pthread_mutex_init(&q->lock, NULL);
    pthread_cond_init(&q->not_empty, NULL);
}

/*
 * Push task to back of queue (producer side)
 */
static void workqueue_push(WorkQueue* q, Task* t) {
    pthread_mutex_lock(&q->lock);
    t->next = NULL;
    if (q->tail) {
        q->tail->next = t;
    } else {
        q->head = t;
    }
    q->tail = t;
    q->count++;
    pthread_cond_signal(&q->not_empty);
    pthread_mutex_unlock(&q->lock);
}

/*
 * Pop task from front of queue (owner side - LIFO for cache locality)
 */
static Task* workqueue_pop(WorkQueue* q) {
    pthread_mutex_lock(&q->lock);
    Task* t = q->head;
    if (t) {
        q->head = t->next;
        if (!q->head) q->tail = NULL;
        q->count--;
        t->next = NULL;
    }
    pthread_mutex_unlock(&q->lock);
    return t;
}

/*
 * Steal task from front (thief side - FIFO for fairness)
 */
static Task* workqueue_steal(WorkQueue* q) {
    /* Same as pop for now, but could be lock-free FIFO */
    return workqueue_pop(q);
}

/*
 * Initialize the scheduler
 */
void scheduler_init(int n_workers) {
    if (n_workers <= 0) {
        /* Default to number of CPU cores */
        n_workers = 4; /* TODO: detect actual cores */
    }
    
    g_scheduler.n_workers = n_workers;
    g_scheduler.workers = calloc(n_workers, sizeof(Worker));
    g_scheduler.running = true;
    workqueue_init(&g_scheduler.global_queue);
    pthread_mutex_init(&g_scheduler.lock, NULL);
    
    /* Start worker threads */
    for (int i = 0; i < n_workers; i++) {
        Worker* w = &g_scheduler.workers[i];
        w->id = i;
        w->running = true;
        workqueue_init(&w->local_queue);
        pthread_create(&w->thread, NULL, worker_thread_fn, w);
    }
}

/*
 * Shutdown scheduler
 */
void scheduler_shutdown(void) {
    g_scheduler.running = false;
    
    /* Wake up all workers */
    for (int i = 0; i < g_scheduler.n_workers; i++) {
        pthread_cond_broadcast(&g_scheduler.workers[i].local_queue.not_empty);
    }
    pthread_cond_broadcast(&g_scheduler.global_queue.not_empty);
    
    /* Join all worker threads */
    for (int i = 0; i < g_scheduler.n_workers; i++) {
        g_scheduler.workers[i].running = false;
        pthread_join(g_scheduler.workers[i].thread, NULL);
    }
    
    free(g_scheduler.workers);
    g_scheduler.workers = NULL;
    g_scheduler.n_workers = 0;
}

/*
 * Create and spawn a new task
 */
void scheduler_spawn(void (*fn)(void* ctx), void* ctx, const char* name) {
    Task* t = malloc(sizeof(Task));
    t->fn = fn;
    t->ctx = ctx;
    t->state = TASK_READY;
    t->next = NULL;
    t->cancelled = false;
    t->name = name ? strdup(name) : NULL;
    
    scheduler_spawn_task(t);
}

/*
 * Spawn from existing Task struct
 */
void scheduler_spawn_task(Task* task) {
    task->state = TASK_READY;
    
    /* If we have a current worker, add to its local queue */
    if (current_worker) {
        workqueue_push(&current_worker->local_queue, task);
    } else {
        /* Otherwise add to global queue */
        workqueue_push(&g_scheduler.global_queue, task);
    }
}

/*
 * Try to steal a task from another worker
 */
static Task* steal_task(Worker* thief) {
    /* Try global queue first */
    Task* t = workqueue_steal(&g_scheduler.global_queue);
    if (t) return t;
    
    /* Try to steal from other workers */
    for (int i = 0; i < g_scheduler.n_workers; i++) {
        if (i == thief->id) continue;
        t = workqueue_steal(&g_scheduler.workers[i].local_queue);
        if (t) return t;
    }
    
    return NULL;
}

/*
 * Worker thread main loop
 */
static void* worker_thread_fn(void* arg) {
    Worker* w = (Worker*)arg;
    current_worker = w;
    
    while (w->running && g_scheduler.running) {
        /* Try local queue first */
        Task* t = workqueue_pop(&w->local_queue);
        
        /* If empty, try stealing */
        if (!t) {
            t = steal_task(w);
        }
        
        if (t) {
            /* Execute the task */
            current_task = t;
            t->state = TASK_RUNNING;
            
            if (t->fn && !t->cancelled) {
                t->fn(t->ctx);
            }
            
            t->state = TASK_DONE;
            current_task = NULL;
            
            /* Free task */
            if (t->name) free(t->name);
            free(t);
        } else {
            /* No work, wait briefly */
            pthread_mutex_lock(&w->local_queue.lock);
            struct timespec ts;
            clock_gettime(CLOCK_REALTIME, &ts);
            ts.tv_nsec += 1000000; /* 1ms timeout */
            if (ts.tv_nsec >= 1000000000) {
                ts.tv_sec++;
                ts.tv_nsec -= 1000000000;
            }
            pthread_cond_timedwait(&w->local_queue.not_empty, 
                                   &w->local_queue.lock, &ts);
            pthread_mutex_unlock(&w->local_queue.lock);
        }
    }
    
    return NULL;
}

/*
 * Run worker (for main thread to participate)
 */
void scheduler_run_worker(Worker* w) {
    worker_thread_fn(w);
}

/*
 * Get current executing task
 */
Task* scheduler_current_task(void) {
    return current_task;
}

/*
 * Check if current task is cancelled
 */
bool scheduler_is_cancelled(void) {
    return current_task && current_task->cancelled;
}

/*
 * Yield to other tasks
 */
void scheduler_yield(void) {
    /* For now, just a thread yield */
    sched_yield();
}

/*
 * Wait for all tasks to complete
 */
void scheduler_wait_all(void) {
    /* Simple busy-wait for now */
    while (1) {
        bool all_empty = true;
        
        if (g_scheduler.global_queue.count > 0) {
            all_empty = false;
        }
        
        for (int i = 0; i < g_scheduler.n_workers; i++) {
            if (g_scheduler.workers[i].local_queue.count > 0) {
                all_empty = false;
                break;
            }
        }
        
        if (all_empty) break;
        
        scheduler_yield();
    }
}

/*
 * Get number of workers
 */
int scheduler_num_workers(void) {
    return g_scheduler.n_workers;
}
