/*
 * db_migrate.c — Migration system for Desi db module
 *
 * Tracks applied migrations in _desi_migrations table.
 * Compares registered ORM models against DB schema via information_schema.
 * Applies CREATE TABLE / ALTER TABLE ADD COLUMN as needed.
 * Supports rollback of the last migration version.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ---- External: dispatch layer ----
extern int32_t __db_query_exec(const char* sql);
extern int32_t __db_execute_stmt(const char* sql);
extern int32_t __db_row_count(void);
extern int32_t __db_col_count(void);
extern char*   __db_get_value_at(int32_t row, int32_t col);
extern char*   __db_get_field_by(int32_t row, const char* name);
extern int32_t __db_is_connected(void);

// ---- External: ORM model registry ----
extern int32_t __orm_model_count(void);
extern char*   __orm_create_table_sql(const char* table_name);
extern char*   __orm_add_column_sql(const char* table_name, const char* col_name);
extern int32_t __orm_field_count(const char* table_name);

// ---- External: active driver ----
extern char* __db_driver(void);

// We need access to model names and field names from the ORM registry.
// Since we can't directly access the static arrays, we add accessor functions.

// These are defined in db_orm.c:
extern const char* __orm_model_name(int32_t index);
extern const char* __orm_field_name(const char* table_name, int32_t field_index);

// ============================================================
// Migration tracking
// ============================================================

static int g_migrate_version = 0;

// Create _desi_migrations table if it doesn't exist
static int migrate_ensure_tracking(void) {
    char* drv = __db_driver();
    int is_pg = (strcmp(drv, "postgres") == 0);
    free(drv);

    const char* sql;
    if (is_pg) {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id SERIAL PRIMARY KEY, "
              "version INTEGER NOT NULL, "
              "table_name VARCHAR(128) NOT NULL, "
              "action VARCHAR(20) NOT NULL, "
              "sql_applied TEXT NOT NULL, "
              "sql_rollback TEXT NOT NULL, "
              "applied_at TIMESTAMPTZ DEFAULT NOW())";
    } else {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id INT AUTO_INCREMENT PRIMARY KEY, "
              "version INTEGER NOT NULL, "
              "table_name VARCHAR(128) NOT NULL, "
              "action VARCHAR(20) NOT NULL, "
              "sql_applied TEXT NOT NULL, "
              "sql_rollback TEXT NOT NULL, "
              "applied_at DATETIME DEFAULT CURRENT_TIMESTAMP) "
              "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4";
    }
    __db_execute_stmt(sql);

    // Get current max version
    int rows = __db_query_exec("SELECT COALESCE(MAX(version), 0) FROM _desi_migrations");
    if (rows > 0) {
        char* v = __db_get_value_at(0, 0);
        g_migrate_version = atoi(v);
        free(v);
    }
    return 0;
}

// Record a migration
static void migrate_record(int version, const char* table_name, const char* action,
                           const char* sql_applied, const char* sql_rollback) {
    // Escape single quotes in SQL strings
    char applied_esc[8192];
    char rollback_esc[8192];
    int ap = 0, rp = 0;
    for (int i = 0; sql_applied[i] && ap < 8190; i++) {
        if (sql_applied[i] == '\'') applied_esc[ap++] = '\'';
        applied_esc[ap++] = sql_applied[i];
    }
    applied_esc[ap] = '\0';
    for (int i = 0; sql_rollback[i] && rp < 8190; i++) {
        if (sql_rollback[i] == '\'') rollback_esc[rp++] = '\'';
        rollback_esc[rp++] = sql_rollback[i];
    }
    rollback_esc[rp] = '\0';

    char insert[16384];
    snprintf(insert, sizeof(insert),
        "INSERT INTO _desi_migrations (version, table_name, action, sql_applied, sql_rollback) "
        "VALUES (%d, '%s', '%s', '%s', '%s')",
        version, table_name, action, applied_esc, rollback_esc);
    __db_execute_stmt(insert);
}

// ============================================================
// Schema introspection
// ============================================================

// Check if a table exists
int32_t __db_table_exists(const char* table_name) {
    if (!__db_is_connected() || !table_name) return 0;

    char* drv = __db_driver();
    int is_pg = (strcmp(drv, "postgres") == 0);
    free(drv);

    char sql[512];
    if (is_pg) {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '%s'",
            table_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'",
            table_name);
    }

    int rows = __db_query_exec(sql);
    return rows > 0 ? 1 : 0;
}

// Check if a column exists in a table
static int column_exists(const char* table_name, const char* col_name) {
    char* drv = __db_driver();
    int is_pg = (strcmp(drv, "postgres") == 0);
    free(drv);

    char sql[512];
    if (is_pg) {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    }

    int rows = __db_query_exec(sql);
    return rows > 0 ? 1 : 0;
}

// ============================================================
// Migrate
// ============================================================

int32_t __db_migrate(void) {
    if (!__db_is_connected()) return -1;

    migrate_ensure_tracking();

    int version = g_migrate_version + 1;
    int applied = 0;
    int model_count = __orm_model_count();

    for (int m = 0; m < model_count; m++) {
        const char* tname = __orm_model_name(m);
        if (!tname || tname[0] == '\0') continue;

        if (!__db_table_exists(tname)) {
            // Table doesn't exist — CREATE TABLE
            char* create_sql = __orm_create_table_sql(tname);
            if (create_sql && create_sql[0] != '\0') {
                int result = __db_execute_stmt(create_sql);
                if (result >= 0 || result == 0) {
                    // Record with rollback = DROP TABLE
                    char rollback[256];
                    snprintf(rollback, sizeof(rollback), "DROP TABLE IF EXISTS %s", tname);
                    migrate_record(version, tname, "create", create_sql, rollback);
                    applied++;
                    fprintf(stderr, "[migrate] CREATE TABLE %s ✓\n", tname);
                } else {
                    fprintf(stderr, "[migrate] CREATE TABLE %s FAILED\n", tname);
                }
            }
            if (create_sql) free(create_sql);
        } else {
            // Table exists — check for missing columns
            int field_count = __orm_field_count(tname);
            for (int f = 0; f < field_count; f++) {
                const char* fname = __orm_field_name(tname, f);
                if (!fname || fname[0] == '\0') continue;

                if (!column_exists(tname, fname)) {
                    // Column missing — ALTER TABLE ADD COLUMN
                    char* alter_sql = __orm_add_column_sql(tname, fname);
                    if (alter_sql && alter_sql[0] != '\0') {
                        int result = __db_execute_stmt(alter_sql);
                        if (result >= 0 || result == 0) {
                            char* drv = __db_driver();
                            int is_pg = (strcmp(drv, "postgres") == 0);
                            free(drv);

                            char rollback[512];
                            if (is_pg) {
                                snprintf(rollback, sizeof(rollback),
                                    "ALTER TABLE %s DROP COLUMN %s", tname, fname);
                            } else {
                                snprintf(rollback, sizeof(rollback),
                                    "ALTER TABLE %s DROP COLUMN %s", tname, fname);
                            }
                            migrate_record(version, tname, "add_column", alter_sql, rollback);
                            applied++;
                            fprintf(stderr, "[migrate] ALTER TABLE %s ADD COLUMN %s ✓\n", tname, fname);
                        } else {
                            fprintf(stderr, "[migrate] ALTER TABLE %s ADD COLUMN %s FAILED\n", tname, fname);
                        }
                    }
                    if (alter_sql) free(alter_sql);
                }
            }
        }
    }

    if (applied > 0) {
        g_migrate_version = version;
        fprintf(stderr, "[migrate] Applied %d operation(s) as version %d\n", applied, version);
    } else {
        fprintf(stderr, "[migrate] No changes needed\n");
    }

    return applied;
}

// ============================================================
// Rollback
// ============================================================

int32_t __db_rollback(void) {
    if (!__db_is_connected()) return -1;

    migrate_ensure_tracking();

    if (g_migrate_version <= 0) {
        fprintf(stderr, "[rollback] No migrations to roll back\n");
        return 0;
    }

    // Get all rollback SQL for the current version (in reverse order)
    char query[256];
    snprintf(query, sizeof(query),
        "SELECT id, table_name, action, sql_rollback FROM _desi_migrations "
        "WHERE version = %d ORDER BY id DESC", g_migrate_version);

    int rows = __db_query_exec(query);
    if (rows <= 0) {
        fprintf(stderr, "[rollback] No entries for version %d\n", g_migrate_version);
        return 0;
    }

    // IMPORTANT: Collect all data from result set BEFORE executing anything,
    // because __db_execute_stmt overwrites the result set.
    #define MAX_ROLLBACK 64
    char* rb_sql[MAX_ROLLBACK];
    char* rb_table[MAX_ROLLBACK];
    char* rb_action[MAX_ROLLBACK];
    int count = rows < MAX_ROLLBACK ? rows : MAX_ROLLBACK;

    for (int i = 0; i < count; i++) {
        rb_sql[i] = __db_get_field_by(i, "sql_rollback");
        rb_table[i] = __db_get_field_by(i, "table_name");
        rb_action[i] = __db_get_field_by(i, "action");
    }

    // Now execute all rollback SQL
    int rolled_back = 0;
    for (int i = 0; i < count; i++) {
        if (rb_sql[i] && rb_sql[i][0] != '\0') {
            int result = __db_execute_stmt(rb_sql[i]);
            if (result >= 0 || result == 0) {
                fprintf(stderr, "[rollback] %s %s ✓\n",
                    rb_action[i] ? rb_action[i] : "?",
                    rb_table[i] ? rb_table[i] : "?");
                rolled_back++;
            } else {
                fprintf(stderr, "[rollback] %s %s FAILED\n",
                    rb_action[i] ? rb_action[i] : "?",
                    rb_table[i] ? rb_table[i] : "?");
            }
        }
        if (rb_sql[i]) free(rb_sql[i]);
        if (rb_table[i]) free(rb_table[i]);
        if (rb_action[i]) free(rb_action[i]);
    }

    // Delete rolled-back entries
    char del[256];
    snprintf(del, sizeof(del),
        "DELETE FROM _desi_migrations WHERE version = %d", g_migrate_version);
    __db_execute_stmt(del);

    g_migrate_version--;
    fprintf(stderr, "[rollback] Rolled back %d operation(s), now at version %d\n",
        rolled_back, g_migrate_version);

    return rolled_back;
}

// ============================================================
// Status
// ============================================================

char* __db_migration_status(void) {
    if (!__db_is_connected()) return strdup("Not connected");

    migrate_ensure_tracking();

    char* buf = malloc(4096);
    int pos = 0;

    pos += sprintf(buf + pos, "Migration Status (version %d)\n", g_migrate_version);
    pos += sprintf(buf + pos, "---\n");

    int rows = __db_query_exec(
        "SELECT version, table_name, action, applied_at FROM _desi_migrations ORDER BY id");

    if (rows <= 0) {
        pos += sprintf(buf + pos, "No migrations applied.\n");
    } else {
        for (int i = 0; i < rows; i++) {
            char* ver = __db_get_field_by(i, "version");
            char* tbl = __db_get_field_by(i, "table_name");
            char* act = __db_get_field_by(i, "action");
            char* at = __db_get_field_by(i, "applied_at");
            pos += sprintf(buf + pos, "  v%s  %-15s %-12s %s\n",
                ver ? ver : "?", tbl ? tbl : "?", act ? act : "?", at ? at : "?");
            if (ver) free(ver);
            if (tbl) free(tbl);
            if (act) free(act);
            if (at) free(at);
        }
    }

    buf[pos] = '\0';
    return buf;
}
