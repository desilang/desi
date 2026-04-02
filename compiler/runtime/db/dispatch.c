/*
 * db_dispatch.c — Unified dispatch layer for PG/MySQL drivers
 *
 * Tracks which driver is active and routes db.connect/db.query/etc
 * to the appropriate backend. This lets Desi code use a single API:
 *
 *   db.connect("postgres", host, port, db, user, pass)
 *   db.query("SELECT * FROM users")
 *   db.execute("INSERT INTO ...")
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// Driver IDs
#define DRIVER_NONE  0
#define DRIVER_PG    1
#define DRIVER_MYSQL 2

static int g_active_driver = DRIVER_NONE;

// ---- Forward declarations to PG driver ----
extern int32_t __pg_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __pg_close(void);
extern int32_t __pg_is_connected(void);
extern char*   __pg_last_error(void);
extern int32_t __pg_query(const char*);
extern int32_t __pg_query_params(const char*, const char**, int);
extern int32_t __pg_execute(const char*);
extern int32_t __pg_row_count(void);
extern int32_t __pg_col_count(void);
extern char*   __pg_col_name(int32_t);
extern char*   __pg_get_value(int32_t, int32_t);
extern char*   __pg_get_field(int32_t, const char*);
extern int32_t __pg_set_debug(int32_t);
extern int32_t __pg_dump_results(void);
extern char*   __pg_connection_info(void);

// ---- Forward declarations to MySQL driver ----
extern int32_t __my_connect(const char*, int32_t, const char*, const char*, const char*);
extern int32_t __my_close(void);
extern int32_t __my_is_connected(void);
extern char*   __my_last_error(void);
extern int32_t __my_query(const char*);
extern int32_t __my_query_params(const char*, const char**, int);
extern int32_t __my_execute(const char*);
extern int32_t __my_row_count(void);
extern int32_t __my_col_count(void);
extern char*   __my_col_name(int32_t);
extern char*   __my_get_value(int32_t, int32_t);
extern char*   __my_get_field(int32_t, const char*);
extern int32_t __my_set_debug(int32_t);
extern int32_t __my_dump_results(void);
extern char*   __my_connection_info(void);

// ---- Forward declarations to CRUD layer ----
extern int32_t __crud_set_dialect(int32_t);

// ============================================================
// Unified connect: first arg is driver name
// ============================================================

int32_t __db_connect(const char* driver, const char* host, int32_t port,
                     const char* dbname, const char* user, const char* password) {
    if (!driver) {
        return -1;
    }

    if (strcmp(driver, "postgres") == 0 || strcmp(driver, "pg") == 0 ||
        strcmp(driver, "postgresql") == 0) {
        g_active_driver = DRIVER_PG;
        __crud_set_dialect(0);  // PG placeholders: $1, $2
        return __pg_connect(host, port, dbname, user, password);
    }

    if (strcmp(driver, "mysql") == 0 || strcmp(driver, "my") == 0 ||
        strcmp(driver, "mariadb") == 0) {
        g_active_driver = DRIVER_MYSQL;
        __crud_set_dialect(1);  // MySQL placeholders: ?
        return __my_connect(host, port, dbname, user, password);
    }

    // Unknown driver
    g_active_driver = DRIVER_NONE;
    return -1;
}

int32_t __db_close(void) {
    int32_t r;
    switch (g_active_driver) {
        case DRIVER_PG:    r = __pg_close(); break;
        case DRIVER_MYSQL: r = __my_close(); break;
        default: return -1;
    }
    g_active_driver = DRIVER_NONE;
    return r;
}

int32_t __db_is_connected(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_is_connected();
        case DRIVER_MYSQL: return __my_is_connected();
        default: return 0;
    }
}

char* __db_last_error(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_last_error();
        case DRIVER_MYSQL: return __my_last_error();
        default: return strdup("No driver active");
    }
}

int32_t __db_query_exec(const char* sql) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_query(sql);
        case DRIVER_MYSQL: return __my_query(sql);
        default: return -1;
    }
}

int32_t __db_execute_stmt(const char* sql) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_execute(sql);
        case DRIVER_MYSQL: return __my_execute(sql);
        default: return -1;
    }
}

// ============================================================
// Parameterized query dispatch
// ============================================================

int32_t __db_query_params(const char* sql, const char** params, int nparams) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_query_params(sql, params, nparams);
        case DRIVER_MYSQL: return __my_query_params(sql, params, nparams);
        default: return -1;
    }
}

int32_t __db_execute_params(const char* sql, const char** params, int nparams) {
    // For execute (INSERT/UPDATE/DELETE), use same parameterized path
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_query_params(sql, params, nparams);
        case DRIVER_MYSQL: return __my_query_params(sql, params, nparams);
        default: return -1;
    }
}

int32_t __db_row_count(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_row_count();
        case DRIVER_MYSQL: return __my_row_count();
        default: return 0;
    }
}

int32_t __db_col_count(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_col_count();
        case DRIVER_MYSQL: return __my_col_count();
        default: return 0;
    }
}

char* __db_col_name_at(int32_t idx) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_col_name(idx);
        case DRIVER_MYSQL: return __my_col_name(idx);
        default: return strdup("");
    }
}

char* __db_get_value_at(int32_t row, int32_t col) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_get_value(row, col);
        case DRIVER_MYSQL: return __my_get_value(row, col);
        default: return strdup("");
    }
}

char* __db_get_field_by(int32_t row, const char* name) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_get_field(row, name);
        case DRIVER_MYSQL: return __my_get_field(row, name);
        default: return strdup("");
    }
}

int32_t __db_debug_mode(int32_t enabled) {
    // Enable on both
    __pg_set_debug(enabled);
    __my_set_debug(enabled);
    return 0;
}

int32_t __db_dump(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_dump_results();
        case DRIVER_MYSQL: return __my_dump_results();
        default: fprintf(stderr, "No driver active\n"); return -1;
    }
}

char* __db_conn_info(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return __pg_connection_info();
        case DRIVER_MYSQL: return __my_connection_info();
        default: return strdup("driver=none");
    }
}

// Get active driver name
char* __db_driver(void) {
    switch (g_active_driver) {
        case DRIVER_PG:    return strdup("postgres");
        case DRIVER_MYSQL: return strdup("mysql");
        default: return strdup("none");
    }
}
