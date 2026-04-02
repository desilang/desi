/*
 * db_connections.c — Named Connection Registry for Multi-DB Support
 *
 * Allows Desi programs to manage multiple named database connections:
 *
 *   __db_register_conn("default", "postgres", host, port, db, user, pass)
 *   __db_register_conn("analytics", "postgres", host2, port2, db2, user2, pass2)
 *   __db_use_conn("analytics")    // switch active connection
 *   ... queries go to analytics DB ...
 *   __db_use_conn("default")      // switch back
 *
 * Each named connection stores its own driver type and connection pointer.
 * Switching connections swaps the global g_pg/g_my pointer to the named
 * connection's struct.
 *
 * The "default" connection is automatically registered on first db.connect().
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ============================================================
// Configuration
// ============================================================

#define CONN_MAX_ENTRIES  16
#define CONN_NAME_MAX     64

// ============================================================
// Connection Entry
// ============================================================

typedef enum {
    CONN_DRIVER_NONE  = 0,
    CONN_DRIVER_PG    = 1,
    CONN_DRIVER_MYSQL = 2,
} ConnDriver;

typedef struct {
    char      name[CONN_NAME_MAX];
    ConnDriver driver;
    void*     conn_ptr;   // PGConn* or MYConn* (opaque)
    int       active;     // 1 if this entry is populated
} ConnEntry;

static ConnEntry g_conn_entries[CONN_MAX_ENTRIES];
static int       g_conn_count = 0;
static int       g_conn_current = -1;  // index of currently active entry

// ============================================================
// Driver accessor functions (defined in driver files)
// ============================================================

extern int32_t __pg_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __pg_close(void);
extern void*   __pg_get_conn_ptr(void);
extern void    __pg_set_conn_ptr(void* conn);

extern int32_t __my_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __my_close(void);
extern void*   __my_get_conn_ptr(void);
extern void    __my_set_conn_ptr(void* conn);

// ============================================================
// Internal helpers
// ============================================================

// Find entry by name, returns index or -1
static int conn_find(const char* name) {
    if (!name) return -1;
    for (int i = 0; i < g_conn_count; i++) {
        if (g_conn_entries[i].active && strcmp(g_conn_entries[i].name, name) == 0) {
            return i;
        }
    }
    return -1;
}

// Parse driver string to enum
static ConnDriver conn_parse_driver(const char* driver) {
    if (!driver) return CONN_DRIVER_NONE;
    if (strcmp(driver, "postgres") == 0 || strcmp(driver, "pg") == 0 ||
        strcmp(driver, "postgresql") == 0) {
        return CONN_DRIVER_PG;
    }
    if (strcmp(driver, "mysql") == 0 || strcmp(driver, "my") == 0 ||
        strcmp(driver, "mariadb") == 0) {
        return CONN_DRIVER_MYSQL;
    }
    return CONN_DRIVER_NONE;
}

// ============================================================
// Register — connect to a DB and store under a name
// ============================================================

int32_t __db_register_conn(const char* name, const char* driver,
                           const char* host, int32_t port,
                           const char* dbname, const char* user,
                           const char* password) {
    if (!name || !driver) {
        fprintf(stderr, "[conn] error: name and driver are required\n");
        return -1;
    }

    // Check if name already exists
    int existing = conn_find(name);
    if (existing >= 0) {
        fprintf(stderr, "[conn] error: connection '%s' already registered\n", name);
        return -1;
    }

    if (g_conn_count >= CONN_MAX_ENTRIES) {
        fprintf(stderr, "[conn] error: maximum %d connections reached\n", CONN_MAX_ENTRIES);
        return -1;
    }

    ConnDriver drv = conn_parse_driver(driver);
    if (drv == CONN_DRIVER_NONE) {
        fprintf(stderr, "[conn] error: unknown driver '%s'\n", driver);
        return -1;
    }

    // Connect using the appropriate driver
    int32_t rc;
    if (drv == CONN_DRIVER_PG) {
        rc = __pg_connect(host, port, dbname, user, password);
        if (rc < 0) {
            fprintf(stderr, "[conn] error: failed to connect '%s' (postgres)\n", name);
            return -1;
        }
    } else {
        rc = __my_connect(host, port, dbname, user, password);
        if (rc < 0) {
            fprintf(stderr, "[conn] error: failed to connect '%s' (mysql)\n", name);
            return -1;
        }
    }

    // Store the connection
    int idx = g_conn_count;
    strncpy(g_conn_entries[idx].name, name, CONN_NAME_MAX - 1);
    g_conn_entries[idx].name[CONN_NAME_MAX - 1] = '\0';
    g_conn_entries[idx].driver = drv;
    g_conn_entries[idx].active = 1;

    // Steal the connection pointer from global
    if (drv == CONN_DRIVER_PG) {
        g_conn_entries[idx].conn_ptr = __pg_get_conn_ptr();
    } else {
        g_conn_entries[idx].conn_ptr = __my_get_conn_ptr();
    }

    g_conn_count++;

    // Auto-activate if this is the first connection
    if (g_conn_count == 1) {
        g_conn_current = idx;
        // Leave the global pointer set (it's already pointing to this conn)
    } else {
        // Don't clear global — keep current active connection unchanged
    }

    return 0;
}

// ============================================================
// Use — switch the active connection by name
// ============================================================

int32_t __db_use_conn(const char* name) {
    int idx = conn_find(name);
    if (idx < 0) {
        fprintf(stderr, "[conn] error: connection '%s' not found\n", name ? name : "(null)");
        return -1;
    }

    ConnEntry* entry = &g_conn_entries[idx];

    // Clear previous global pointer
    if (g_conn_current >= 0 && g_conn_current < g_conn_count) {
        ConnEntry* prev = &g_conn_entries[g_conn_current];
        if (prev->driver == CONN_DRIVER_PG) {
            __pg_set_conn_ptr(NULL);
        } else if (prev->driver == CONN_DRIVER_MYSQL) {
            __my_set_conn_ptr(NULL);
        }
    }

    // Set new active connection
    if (entry->driver == CONN_DRIVER_PG) {
        __pg_set_conn_ptr(entry->conn_ptr);
    } else {
        __my_set_conn_ptr(entry->conn_ptr);
    }

    g_conn_current = idx;
    return 0;
}

// ============================================================
// Close — disconnect a named connection
// ============================================================

int32_t __db_close_conn(const char* name) {
    int idx = conn_find(name);
    if (idx < 0) {
        fprintf(stderr, "[conn] error: connection '%s' not found\n", name ? name : "(null)");
        return -1;
    }

    ConnEntry* entry = &g_conn_entries[idx];

    // Close the actual connection
    if (entry->driver == CONN_DRIVER_PG) {
        __pg_set_conn_ptr(entry->conn_ptr);
        __pg_close();
        free(entry->conn_ptr);
        __pg_set_conn_ptr(NULL);
    } else {
        __my_set_conn_ptr(entry->conn_ptr);
        __my_close();
        free(entry->conn_ptr);
        __my_set_conn_ptr(NULL);
    }

    entry->conn_ptr = NULL;
    entry->active = 0;

    // If this was the current, clear it
    if (g_conn_current == idx) {
        g_conn_current = -1;
    }

    return 0;
}

// ============================================================
// Close All — disconnect all named connections
// ============================================================

int32_t __db_close_all_conns(void) {
    for (int i = 0; i < g_conn_count; i++) {
        if (g_conn_entries[i].active && g_conn_entries[i].conn_ptr) {
            ConnEntry* entry = &g_conn_entries[i];
            if (entry->driver == CONN_DRIVER_PG) {
                __pg_set_conn_ptr(entry->conn_ptr);
                __pg_close();
                free(entry->conn_ptr);
            } else {
                __my_set_conn_ptr(entry->conn_ptr);
                __my_close();
                free(entry->conn_ptr);
            }
            entry->conn_ptr = NULL;
            entry->active = 0;
        }
    }

    // Clear globals
    __pg_set_conn_ptr(NULL);
    __my_set_conn_ptr(NULL);

    g_conn_count = 0;
    g_conn_current = -1;
    return 0;
}

// ============================================================
// Info / Stats
// ============================================================

// Get the name of the currently active connection
char* __db_current_conn(void) {
    if (g_conn_current < 0 || g_conn_current >= g_conn_count) {
        return strdup("none");
    }
    return strdup(g_conn_entries[g_conn_current].name);
}

// Get the number of registered connections
int32_t __db_conn_count(void) {
    int count = 0;
    for (int i = 0; i < g_conn_count; i++) {
        if (g_conn_entries[i].active) count++;
    }
    return count;
}

// List all connection names (comma-separated)
char* __db_list_conns(void) {
    char buf[1024] = "";
    int pos = 0;
    int first = 1;
    for (int i = 0; i < g_conn_count; i++) {
        if (!g_conn_entries[i].active) continue;
        if (!first) pos += snprintf(buf + pos, sizeof(buf) - pos, ",");
        pos += snprintf(buf + pos, sizeof(buf) - pos, "%s", g_conn_entries[i].name);
        first = 0;
    }
    return strdup(buf);
}
