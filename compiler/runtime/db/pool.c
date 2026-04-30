/*
 * db_pool.c — Connection Pool for Desi ORM
 *
 * Manages a fixed-size pool of pre-connected database connections.
 * Each slot holds a driver-specific connection struct (PGConn* or MYConn*).
 *
 * Usage flow:
 *   __db_pool_init("postgres", host, port, db, user, pass, 5)
 *   __db_pool_acquire()   // sets active connection from pool
 *   ... use db.query(), db.execute(), ORM methods ...
 *   __db_pool_release()   // returns connection to pool
 *   __db_pool_close()     // closes all connections
 *
 * Thread-safety: mutex + condition variable for concurrent acquire/release.
 * Blocking: acquire() uses pthread_cond_timedwait — zero CPU while waiting,
 *           instant wakeup when a slot is released. 30-second timeout.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <pthread.h>
#include <unistd.h>
#include <time.h>
#include <errno.h>

// ============================================================
// Pool Configuration
// ============================================================

#define POOL_MAX_SLOTS           32
#define POOL_ACQUIRE_TIMEOUT_SEC 30   // seconds

// ============================================================
// Pool Slot
// ============================================================

typedef enum {
    SLOT_FREE = 0,     // available for acquire
    SLOT_IN_USE = 1,   // currently acquired by a caller
} SlotState;

typedef struct {
    void*     conn;     // PGConn* or MYConn* (opaque to pool)
    SlotState state;
} PoolSlot;

// ============================================================
// Pool State
// ============================================================

typedef enum {
    POOL_DRIVER_NONE = 0,
    POOL_DRIVER_PG   = 1,
    POOL_DRIVER_MYSQL = 2,
} PoolDriver;

static PoolSlot     g_pool_slots[POOL_MAX_SLOTS];
static int          g_pool_size = 0;
static PoolDriver   g_pool_driver = POOL_DRIVER_NONE;
static int          g_pool_initialized = 0;
static int          g_pool_active_slot = -1;  // currently active slot index
static pthread_mutex_t g_pool_mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t  g_pool_cond  = PTHREAD_COND_INITIALIZER;

// ============================================================
// Driver-specific helpers (forward declarations)
// ============================================================

// PGConn management
extern int32_t __pg_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __pg_close(void);

// MYConn management
extern int32_t __my_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __my_close(void);

// These are the global pointers we swap for pool operations.
// Defined in db_postgres.c and db_mysql.c respectively.
// The pool treats connections as opaque void* — type knowledge
// stays in the driver files where it belongs.

// Pool accessor functions (defined in the driver files)
extern void* __pg_get_conn_ptr(void);
extern void  __pg_set_conn_ptr(void* conn);
extern void* __my_get_conn_ptr(void);
extern void  __my_set_conn_ptr(void* conn);

// Reconnect functions (use stored params in conn struct)
extern int32_t __pg_reconnect(void);
extern int32_t __my_reconnect(void);

// Lightweight ping (SELECT 1)
extern int32_t __pg_query(const char*);
extern int32_t __my_query(const char*);
extern int32_t __pg_is_connected(void);
extern int32_t __my_is_connected(void);

// ============================================================
// Pool Init — create N connections to the same database
// ============================================================

int32_t __db_pool_init(const char* driver, const char* host, int32_t port,
                       const char* dbname, const char* user, const char* password,
                       int32_t pool_size) {
    if (g_pool_initialized) {
        fprintf(stderr, "[pool] error: pool already initialized\n");
        return -1;
    }
    if (pool_size <= 0 || pool_size > POOL_MAX_SLOTS) {
        fprintf(stderr, "[pool] error: pool_size must be 1-%d, got %d\n",
                POOL_MAX_SLOTS, pool_size);
        return -1;
    }

    // Determine driver
    if (strcmp(driver, "postgres") == 0 || strcmp(driver, "pg") == 0 ||
        strcmp(driver, "postgresql") == 0) {
        g_pool_driver = POOL_DRIVER_PG;
    } else if (strcmp(driver, "mysql") == 0 || strcmp(driver, "my") == 0 ||
               strcmp(driver, "mariadb") == 0) {
        g_pool_driver = POOL_DRIVER_MYSQL;
    } else {
        fprintf(stderr, "[pool] error: unknown driver '%s'\n", driver);
        return -1;
    }

    // Create connections
    int created = 0;
    for (int i = 0; i < pool_size; i++) {
        int32_t rc;

        if (g_pool_driver == POOL_DRIVER_PG) {
            // Connect creates/allocates g_pg, then we steal the pointer
            rc = __pg_connect(host, port, dbname, user, password);
            if (rc < 0) {
                fprintf(stderr, "[pool] error: failed to create PG connection %d\n", i);
                break;
            }
            // Steal the connection struct from the global pointer
            g_pool_slots[i].conn = __pg_get_conn_ptr();
            g_pool_slots[i].state = SLOT_FREE;
            // Clear global so next connect allocates a fresh struct
            __pg_set_conn_ptr(NULL);
        } else {
            rc = __my_connect(host, port, dbname, user, password);
            if (rc < 0) {
                fprintf(stderr, "[pool] error: failed to create MySQL connection %d\n", i);
                break;
            }
            g_pool_slots[i].conn = __my_get_conn_ptr();
            g_pool_slots[i].state = SLOT_FREE;
            __my_set_conn_ptr(NULL);
        }

        created++;
    }

    if (created == 0) {
        fprintf(stderr, "[pool] error: could not create any connections\n");
        g_pool_driver = POOL_DRIVER_NONE;
        return -1;
    }

    g_pool_size = created;
    g_pool_initialized = 1;
    g_pool_active_slot = -1;

    fprintf(stderr, "[pool] initialized %d/%d %s connections\n",
            created, pool_size,
            g_pool_driver == POOL_DRIVER_PG ? "postgres" : "mysql");
    return created;
}

// ============================================================
// Acquire — get a free connection from pool, set as active
// ============================================================

// Ping a pooled connection (must be set as active global ptr first).
// Returns 0 if alive, -1 if dead.
static int pool_ping_slot(void) {
    if (g_pool_driver == POOL_DRIVER_PG) {
        if (!__pg_is_connected()) return -1;
        return __pg_query("SELECT 1") >= 0 ? 0 : -1;
    } else {
        if (!__my_is_connected()) return -1;
        return __my_query("SELECT 1") >= 0 ? 0 : -1;
    }
}

// Attempt to reconnect the active pooled connection.
// On success, update the pool slot with the new conn pointer.
static int pool_reconnect_slot(int slot) {
    int32_t rc;
    fprintf(stderr, "[pool] slot %d dead, attempting reconnect...\n", slot);
    if (g_pool_driver == POOL_DRIVER_PG) {
        rc = __pg_reconnect();
        if (rc >= 0) {
            g_pool_slots[slot].conn = __pg_get_conn_ptr();
            fprintf(stderr, "[pool] slot %d reconnected\n", slot);
        }
    } else {
        rc = __my_reconnect();
        if (rc >= 0) {
            g_pool_slots[slot].conn = __my_get_conn_ptr();
            fprintf(stderr, "[pool] slot %d reconnected\n", slot);
        }
    }
    return rc;
}

// Compute absolute deadline for pthread_cond_timedwait.
static void pool_deadline(struct timespec *ts) {
    clock_gettime(CLOCK_REALTIME, ts);
    ts->tv_sec += POOL_ACQUIRE_TIMEOUT_SEC;
}

// Try to claim a free slot. Returns slot index or -1.
// MUST be called with g_pool_mutex held. Unlocks mutex on success.
static int pool_try_claim(void) {
    for (int i = 0; i < g_pool_size; i++) {
        if (g_pool_slots[i].state == SLOT_FREE) {
            g_pool_slots[i].state = SLOT_IN_USE;
            g_pool_active_slot = i;

            // Swap the global driver pointer to this connection
            if (g_pool_driver == POOL_DRIVER_PG) {
                __pg_set_conn_ptr(g_pool_slots[i].conn);
            } else {
                __my_set_conn_ptr(g_pool_slots[i].conn);
            }

            pthread_mutex_unlock(&g_pool_mutex);

            // Health check outside the lock (I/O is slow)
            if (pool_ping_slot() < 0) {
                if (pool_reconnect_slot(i) < 0) {
                    // Reconnect failed — put slot back and let caller retry
                    fprintf(stderr, "[pool] slot %d reconnect failed, skipping\n", i);
                    pthread_mutex_lock(&g_pool_mutex);
                    g_pool_slots[i].state = SLOT_FREE;
                    g_pool_active_slot = -1;
                    // Signal so other waiters can try this slot
                    pthread_cond_signal(&g_pool_cond);
                    return -1;  // caller retries with mutex held
                }
            }

            return i;
        }
    }
    return -1;
}

int32_t __db_pool_acquire(void) {
    if (!g_pool_initialized) {
        fprintf(stderr, "[pool] error: pool not initialized\n");
        return -1;
    }

    struct timespec deadline;
    pool_deadline(&deadline);

    pthread_mutex_lock(&g_pool_mutex);

    for (;;) {
        // Pool may have been closed while we were waiting
        if (!g_pool_initialized) {
            pthread_mutex_unlock(&g_pool_mutex);
            fprintf(stderr, "[pool] error: pool closed while waiting\n");
            return -1;
        }

        // Scan for a free slot
        int slot = pool_try_claim();  // unlocks mutex on success
        if (slot >= 0) return slot;

        // pool_try_claim returns -1 with mutex held when:
        //   (a) no free slot found, or
        //   (b) a free slot was found but reconnect failed (slot re-freed)
        // Either way, wait for a signal from release() or close().

        int rc = pthread_cond_timedwait(&g_pool_cond, &g_pool_mutex, &deadline);
        if (rc == ETIMEDOUT) {
            pthread_mutex_unlock(&g_pool_mutex);
            fprintf(stderr, "[pool] error: acquire timed out after %ds\n",
                    POOL_ACQUIRE_TIMEOUT_SEC);
            return -1;
        }
        // Spurious wakeup or signal — loop back and re-scan slots
    }
}

// ============================================================
// Release — return the active connection to pool
// ============================================================

int32_t __db_pool_release(void) {
    if (!g_pool_initialized) return -1;

    pthread_mutex_lock(&g_pool_mutex);

    if (g_pool_active_slot < 0 || g_pool_active_slot >= g_pool_size) {
        pthread_mutex_unlock(&g_pool_mutex);
        fprintf(stderr, "[pool] warning: release called with no active slot\n");
        return -1;
    }

    g_pool_slots[g_pool_active_slot].state = SLOT_FREE;

    // Clear the global pointer (prevents use-after-release bugs)
    if (g_pool_driver == POOL_DRIVER_PG) {
        __pg_set_conn_ptr(NULL);
    } else {
        __my_set_conn_ptr(NULL);
    }

    g_pool_active_slot = -1;

    // Wake one thread waiting in acquire()
    pthread_cond_signal(&g_pool_cond);
    pthread_mutex_unlock(&g_pool_mutex);
    return 0;
}

// ============================================================
// Close — disconnect and free all pool connections
// ============================================================

int32_t __db_pool_close(void) {
    if (!g_pool_initialized) return 0;

    pthread_mutex_lock(&g_pool_mutex);

    for (int i = 0; i < g_pool_size; i++) {
        if (g_pool_slots[i].conn) {
            // Set as active so the driver's close function works
            if (g_pool_driver == POOL_DRIVER_PG) {
                __pg_set_conn_ptr(g_pool_slots[i].conn);
                __pg_close();
                // __pg_close doesn't free the struct, just closes fd
                free(g_pool_slots[i].conn);
            } else {
                __my_set_conn_ptr(g_pool_slots[i].conn);
                __my_close();
                free(g_pool_slots[i].conn);
            }
            g_pool_slots[i].conn = NULL;
            g_pool_slots[i].state = SLOT_FREE;
        }
    }

    // Clear global pointer
    if (g_pool_driver == POOL_DRIVER_PG) {
        __pg_set_conn_ptr(NULL);
    } else {
        __my_set_conn_ptr(NULL);
    }

    g_pool_size = 0;
    g_pool_driver = POOL_DRIVER_NONE;
    g_pool_initialized = 0;
    g_pool_active_slot = -1;

    // Wake ALL threads waiting in acquire() so they see pool is closed
    pthread_cond_broadcast(&g_pool_cond);
    pthread_mutex_unlock(&g_pool_mutex);
    return 0;
}

// ============================================================
// Stats
// ============================================================

int32_t __db_pool_size(void) {
    return g_pool_size;
}

int32_t __db_pool_available(void) {
    if (!g_pool_initialized) return 0;

    pthread_mutex_lock(&g_pool_mutex);
    int avail = 0;
    for (int i = 0; i < g_pool_size; i++) {
        if (g_pool_slots[i].state == SLOT_FREE) avail++;
    }
    pthread_mutex_unlock(&g_pool_mutex);
    return avail;
}

int32_t __db_pool_active_slot(void) {
    return g_pool_active_slot;
}

// Is the pool initialized? Used by dispatch for auto-acquire/release.
int32_t __db_pool_initialized(void) {
    return g_pool_initialized;
}
