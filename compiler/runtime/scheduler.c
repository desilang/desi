/*
 * Desi Runtime Scheduler Implementation
 * 
 * M:N threading with work-stealing for lightweight concurrency.
 * Cross-platform using platform.h abstractions.
 */

#include "scheduler.h"
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

/* Platform-specific thread-local storage */
#ifdef _WIN32
    #define DESI_THREAD_LOCAL __declspec(thread)
#else
    #define DESI_THREAD_LOCAL __thread
#endif

/* Global scheduler instance */
static Scheduler g_scheduler = {0};

/* Thread-local current task */
static DESI_THREAD_LOCAL Task* current_task = NULL;
static DESI_THREAD_LOCAL Worker* current_worker = NULL;

/* Forward declarations */
#ifdef _WIN32
static DWORD WINAPI worker_thread_fn(LPVOID arg);
#else
static void* worker_thread_fn(void* arg);
#endif
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
    DESI_MUTEX_INIT(q->lock);
    DESI_COND_INIT(q->not_empty);
}

/*
 * Push task to back of queue (producer side)
 */
static void workqueue_push(WorkQueue* q, Task* t) {
    DESI_MUTEX_LOCK(q->lock);
    t->next = NULL;
    if (q->tail) {
        q->tail->next = t;
    } else {
        q->head = t;
    }
    q->tail = t;
    q->count++;
    DESI_COND_SIGNAL(q->not_empty);
    DESI_MUTEX_UNLOCK(q->lock);
}

/*
 * Pop task from front of queue (owner side - LIFO for cache locality)
 */
static Task* workqueue_pop(WorkQueue* q) {
    DESI_MUTEX_LOCK(q->lock);
    Task* t = q->head;
    if (t) {
        q->head = t->next;
        if (!q->head) q->tail = NULL;
        q->count--;
        t->next = NULL;
    }
    DESI_MUTEX_UNLOCK(q->lock);
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
 * Platform-specific thread yield
 */
static void platform_yield(void) {
#ifdef _WIN32
    SwitchToThread();
#else
    sched_yield();
#endif
}

/*
 * Platform-specific timed wait (returns after timeout or signal)
 */
static void platform_cond_timedwait_ms(DesiPlatformCond* cond, DesiPlatformMutex* mutex, int ms) {
#ifdef _WIN32
    SleepConditionVariableSRW(cond, mutex, (DWORD)ms, 0);
#else
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    ts.tv_nsec += ms * 1000000;
    while (ts.tv_nsec >= 1000000000) {
        ts.tv_sec++;
        ts.tv_nsec -= 1000000000;
    }
    pthread_cond_timedwait(cond, mutex, &ts);
#endif
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
    DESI_MUTEX_INIT(g_scheduler.lock);
    
    /* Start worker threads */
    for (int i = 0; i < n_workers; i++) {
        Worker* w = &g_scheduler.workers[i];
        w->id = i;
        w->running = true;
        workqueue_init(&w->local_queue);
        
#ifdef _WIN32
        w->thread = CreateThread(NULL, 0, worker_thread_fn, w, 0, NULL);
#else
        pthread_create(&w->thread, NULL, worker_thread_fn, w);
#endif
    }
}

/*
 * Shutdown scheduler
 */
void scheduler_shutdown(void) {
    g_scheduler.running = false;
    
    /* Wake up all workers */
    for (int i = 0; i < g_scheduler.n_workers; i++) {
        DESI_COND_BROADCAST(g_scheduler.workers[i].local_queue.not_empty);
    }
    DESI_COND_BROADCAST(g_scheduler.global_queue.not_empty);
    
    /* Join all worker threads */
    for (int i = 0; i < g_scheduler.n_workers; i++) {
        g_scheduler.workers[i].running = false;
#ifdef _WIN32
        WaitForSingleObject(g_scheduler.workers[i].thread, INFINITE);
        CloseHandle(g_scheduler.workers[i].thread);
#else
        pthread_join(g_scheduler.workers[i].thread, NULL);
#endif
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
#ifdef _WIN32
    t->name = name ? _strdup(name) : NULL;
#else
    t->name = name ? strdup(name) : NULL;
#endif
    
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
#ifdef _WIN32
static DWORD WINAPI worker_thread_fn(LPVOID arg) {
#else
static void* worker_thread_fn(void* arg) {
#endif
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
            /* No work, wait briefly (1ms timeout) */
            DESI_MUTEX_LOCK(w->local_queue.lock);
            platform_cond_timedwait_ms(&w->local_queue.not_empty, 
                                       &w->local_queue.lock, 1);
            DESI_MUTEX_UNLOCK(w->local_queue.lock);
        }
    }
    
#ifdef _WIN32
    return 0;
#else
    return NULL;
#endif
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
    platform_yield();
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
