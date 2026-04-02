/*
 * db_transaction.c — Transaction support for Desi db module
 *
 * BEGIN/COMMIT/ROLLBACK + SAVEPOINT for nested transactions.
 * Works with both PostgreSQL and MySQL via the dispatch layer.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ---- External: dispatch layer ----
extern int32_t __db_execute_stmt(const char* sql);
extern int32_t __db_is_connected(void);

// ============================================================
// Transaction state
// ============================================================

static int g_in_transaction = 0;

int32_t __db_begin(void) {
    if (!__db_is_connected()) return -1;
    if (g_in_transaction) {
        fprintf(stderr, "[tx] Already in transaction\n");
        return -1;
    }
    int r = __db_execute_stmt("BEGIN");
    if (r >= 0) {
        g_in_transaction = 1;
    }
    return r;
}

int32_t __db_commit(void) {
    if (!__db_is_connected()) return -1;
    if (!g_in_transaction) {
        fprintf(stderr, "[tx] No active transaction to commit\n");
        return -1;
    }
    int r = __db_execute_stmt("COMMIT");
    g_in_transaction = 0;
    return r;
}

int32_t __db_rollback_tx(void) {
    if (!__db_is_connected()) return -1;
    if (!g_in_transaction) {
        fprintf(stderr, "[tx] No active transaction to rollback\n");
        return -1;
    }
    int r = __db_execute_stmt("ROLLBACK");
    g_in_transaction = 0;
    return r;
}

int32_t __db_in_transaction(void) {
    return g_in_transaction;
}

// ============================================================
// Savepoints (nested transactions)
// ============================================================

int32_t __db_savepoint(const char* name) {
    if (!__db_is_connected() || !g_in_transaction) return -1;
    char sql[256];
    snprintf(sql, sizeof(sql), "SAVEPOINT %s", name);
    return __db_execute_stmt(sql);
}

int32_t __db_savepoint_rollback(const char* name) {
    if (!__db_is_connected() || !g_in_transaction) return -1;
    char sql[256];
    snprintf(sql, sizeof(sql), "ROLLBACK TO SAVEPOINT %s", name);
    return __db_execute_stmt(sql);
}

int32_t __db_savepoint_release(const char* name) {
    if (!__db_is_connected() || !g_in_transaction) return -1;
    char sql[256];
    snprintf(sql, sizeof(sql), "RELEASE SAVEPOINT %s", name);
    return __db_execute_stmt(sql);
}
