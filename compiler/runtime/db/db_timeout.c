/*
 * db_timeout.c — Shared timeout globals for PG/MySQL drivers
 *
 * These globals must live in a single translation unit so that all
 * drivers (postgres.c, mysql.c, dispatch.c) share the SAME values.
 * The header (db_timeout.h) declares them as extern.
 */

#include <stdint.h>

// ============================================================
// Global configurable timeouts (milliseconds)
// ============================================================

int g_db_connect_timeout_ms = 5000;   // 5 seconds
int g_db_read_timeout_ms    = 30000;  // 30 seconds

// ============================================================
// Set timeouts — called from Desi API via dispatch
// ============================================================

int32_t db_set_timeouts(int32_t connect_ms, int32_t read_ms) {
    if (connect_ms > 0) g_db_connect_timeout_ms = connect_ms;
    if (read_ms > 0)    g_db_read_timeout_ms = read_ms;
    return 0;
}
