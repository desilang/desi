/*
 * db_migrate.c — Django-style file-based migration system for Desi
 *
 * Provides:
 *   - makemigrations(dir): Generate .desi migration files from ORM model diffs
 *   - migrate(dir):        Apply pending .desi migration files
 *   - rollback(dir):       Rollback the last applied migration file
 *   - migration_status(dir): Show applied/pending migrations
 *
 * Migration files are real Desi code with forward()/rollback() that call
 * db.execute("SQL"). The C runtime parses SQL from db.execute() calls.
 *
 * Tracking is stored in the _desi_migrations table in the connected database.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <dirent.h>
#include <sys/stat.h>
#include <time.h>

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
extern const char* __orm_model_name(int32_t index);
extern const char* __orm_field_name(const char* table_name, int32_t field_index);
extern char*   __orm_column_type_sql(const char* table_name, const char* col_name);
extern char*   __orm_drop_table_sql(const char* table_name);

// ---- External: active driver ----
extern char* __db_driver(void);

// ============================================================
// Migration tracking table
// ============================================================

static int g_tracking_ready = 0;

static int is_pg(void) {
    char* drv = __db_driver();
    int pg = (strcmp(drv, "postgres") == 0);
    free(drv);
    return pg;
}

// Create/ensure _desi_migrations table with filename column
static void ensure_tracking(void) {
    if (g_tracking_ready) return;

    const char* sql;
    if (is_pg()) {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id SERIAL PRIMARY KEY, "
              "filename VARCHAR(256) NOT NULL, "
              "applied_sql TEXT NOT NULL, "
              "rollback_sql TEXT NOT NULL, "
              "applied_at TIMESTAMPTZ DEFAULT NOW())";
    } else {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id INT AUTO_INCREMENT PRIMARY KEY, "
              "filename VARCHAR(256) NOT NULL, "
              "applied_sql TEXT NOT NULL, "
              "rollback_sql TEXT NOT NULL, "
              "applied_at DATETIME DEFAULT CURRENT_TIMESTAMP) "
              "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4";
    }
    __db_execute_stmt(sql);
    g_tracking_ready = 1;
}

// Check if a migration file has already been applied
static int migration_applied(const char* filename) {
    char sql[512];
    snprintf(sql, sizeof(sql),
        "SELECT 1 FROM _desi_migrations WHERE filename = '%s'", filename);
    int rows = __db_query_exec(sql);
    return rows > 0 ? 1 : 0;
}

// Escape single quotes for SQL strings
static void sql_escape(const char* src, char* dst, int dst_size) {
    int j = 0;
    for (int i = 0; src[i] && j < dst_size - 2; i++) {
        if (src[i] == '\'') dst[j++] = '\'';
        dst[j++] = src[i];
    }
    dst[j] = '\0';
}

// Record a migration as applied
static void record_migration(const char* filename, const char* applied_sql, const char* rollback_sql) {
    char applied_esc[16384];
    char rollback_esc[16384];
    sql_escape(applied_sql, applied_esc, sizeof(applied_esc));
    sql_escape(rollback_sql, rollback_esc, sizeof(rollback_esc));

    char insert[32768];
    snprintf(insert, sizeof(insert),
        "INSERT INTO _desi_migrations (filename, applied_sql, rollback_sql) "
        "VALUES ('%s', '%s', '%s')",
        filename, applied_esc, rollback_esc);
    __db_execute_stmt(insert);
}

// ============================================================
// Schema introspection (kept from original)
// ============================================================

// Check if a table exists
int32_t __db_table_exists(const char* table_name) {
    if (!__db_is_connected() || !table_name) return 0;

    char sql[512];
    if (is_pg()) {
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
    char sql[512];
    if (is_pg()) {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    }
    return __db_query_exec(sql) > 0 ? 1 : 0;
}

// Get actual SQL type of a column from information_schema
static char* get_column_type(const char* table_name, const char* col_name) {
    char sql[512];
    if (is_pg()) {
        snprintf(sql, sizeof(sql),
            "SELECT data_type FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT column_type FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'",
            table_name, col_name);
    }
    int rows = __db_query_exec(sql);
    if (rows > 0) return __db_get_value_at(0, 0);
    return strdup("");
}

// Get all column names in a table
static int get_table_columns(const char* table_name) {
    char sql[512];
    if (is_pg()) {
        snprintf(sql, sizeof(sql),
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' ORDER BY ordinal_position",
            table_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' ORDER BY ordinal_position",
            table_name);
    }
    return __db_query_exec(sql);
}

// Case-insensitive string comparison
static int strcasecmp_local(const char* a, const char* b) {
    while (*a && *b) {
        char ca = *a >= 'A' && *a <= 'Z' ? *a + 32 : *a;
        char cb = *b >= 'A' && *b <= 'Z' ? *b + 32 : *b;
        if (ca != cb) return ca - cb;
        a++; b++;
    }
    return *a - *b;
}

// Normalize SQL type names for comparison
static int types_match(const char* db_type, const char* model_type) {
    if (!db_type || !model_type) return 1;
    if (strcasecmp_local(db_type, model_type) == 0) return 1;

    // PG long-form names
    if (strcasecmp_local(db_type, "integer") == 0 &&
        strcasecmp_local(model_type, "INTEGER") == 0) return 1;
    if (strcasecmp_local(db_type, "bigint") == 0 &&
        strcasecmp_local(model_type, "BIGINT") == 0) return 1;
    if (strcasecmp_local(db_type, "character varying") == 0 &&
        strncasecmp(model_type, "VARCHAR", 7) == 0) return 1;
    if (strcasecmp_local(db_type, "text") == 0 &&
        strcasecmp_local(model_type, "TEXT") == 0) return 1;
    if (strcasecmp_local(db_type, "boolean") == 0 &&
        strcasecmp_local(model_type, "BOOLEAN") == 0) return 1;
    if (strcasecmp_local(db_type, "double precision") == 0 &&
        strcasecmp_local(model_type, "DOUBLE PRECISION") == 0) return 1;
    if (strcasecmp_local(db_type, "timestamp with time zone") == 0 &&
        strcasecmp_local(model_type, "TIMESTAMPTZ") == 0) return 1;
    if (strcasecmp_local(db_type, "jsonb") == 0 &&
        strcasecmp_local(model_type, "JSONB") == 0) return 1;
    if (strcasecmp_local(db_type, "json") == 0 &&
        strcasecmp_local(model_type, "JSON") == 0) return 1;
    if (strcasecmp_local(db_type, "uuid") == 0 &&
        strcasecmp_local(model_type, "UUID") == 0) return 1;
    if (strcasecmp_local(db_type, "real") == 0 &&
        strcasecmp_local(model_type, "REAL") == 0) return 1;
    if (strcasecmp_local(db_type, "numeric") == 0 &&
        strncasecmp(model_type, "DECIMAL", 7) == 0) return 1;
    if (strcasecmp_local(db_type, "date") == 0 &&
        strcasecmp_local(model_type, "DATE") == 0) return 1;

    // MySQL short-form
    if (strcasecmp_local(db_type, "int") == 0 &&
        strcasecmp_local(model_type, "INTEGER") == 0) return 1;
    if (strcasecmp_local(db_type, "tinyint(1)") == 0 &&
        strcasecmp_local(model_type, "TINYINT(1)") == 0) return 1;

    // Auto-increment PK types
    if (strcasecmp_local(db_type, "integer") == 0 &&
        strncasecmp(model_type, "SERIAL", 6) == 0) return 1;
    if (strcasecmp_local(db_type, "int") == 0 &&
        strncasecmp(model_type, "INT AUTO_INCREMENT", 18) == 0) return 1;
    if (strcasecmp_local(db_type, "bigint") == 0 &&
        strncasecmp(model_type, "BIGSERIAL", 9) == 0) return 1;

    return 0;
}

// ============================================================
// .desi Migration File Parser
//
// Extracts SQL strings from db.execute("...") / db.execute("""...""")
// calls within forward() and rollback() functions.
// ============================================================

#define MAX_STMTS 64
#define MAX_SQL_LEN 8192

typedef struct {
    char stmts[MAX_STMTS][MAX_SQL_LEN];
    int count;
} SqlList;

// Read entire file into malloc'd buffer
static char* read_file(const char* path) {
    FILE* f = fopen(path, "r");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* buf = malloc(size + 1);
    if (!buf) { fclose(f); return NULL; }
    long read = fread(buf, 1, size, f);
    buf[read] = '\0';
    fclose(f);
    return buf;
}

// Find the start of a section ("def forward():" or "def rollback():")
static const char* find_section(const char* content, const char* section_name) {
    char pattern[64];
    snprintf(pattern, sizeof(pattern), "def %s()", section_name);
    const char* p = strstr(content, pattern);
    if (!p) return NULL;
    // Skip to the next line after the colon
    p = strchr(p, ':');
    if (!p) return NULL;
    p++; // skip ':'
    while (*p == ' ' || *p == '\t' || *p == '\r') p++;
    if (*p == '\n') p++;
    return p;
}

// Find the end of a section (next "def " at column 0, or EOF)
static const char* find_section_end(const char* section_start) {
    const char* p = section_start;
    while (*p) {
        // A line starting with "def " (not indented) marks the next function
        if (*p == '\n') {
            if (strncmp(p + 1, "def ", 4) == 0) {
                return p;
            }
        }
        p++;
    }
    return p; // EOF
}

// Extract SQL from a single db.execute() call
// Handles both db.execute("...") and db.execute("""...""")
// Returns pointer past the closing paren, or NULL on error
static const char* extract_execute_sql(const char* p, char* sql_out, int sql_max) {
    // We're positioned at the start of the string content
    int pos = 0;
    int triple = 0;

    // Check for triple-quote
    if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
        triple = 1;
        p += 3;
        // Find closing """
        while (*p) {
            if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
                sql_out[pos] = '\0';
                p += 3; // skip """
                // Skip to closing paren
                while (*p && *p != ')') p++;
                if (*p == ')') p++;
                return p;
            }
            if (pos < sql_max - 1) sql_out[pos++] = *p;
            p++;
        }
    } else if (*p == '"') {
        // Single-quoted string
        p++; // skip opening "
        while (*p && *p != '"') {
            if (*p == '\\' && p[1]) {
                // Handle escapes
                p++;
                if (*p == 'n') { if (pos < sql_max - 1) sql_out[pos++] = '\n'; }
                else if (*p == 't') { if (pos < sql_max - 1) sql_out[pos++] = '\t'; }
                else if (*p == '"') { if (pos < sql_max - 1) sql_out[pos++] = '"'; }
                else if (*p == '\\') { if (pos < sql_max - 1) sql_out[pos++] = '\\'; }
                else { if (pos < sql_max - 1) sql_out[pos++] = *p; }
            } else {
                if (pos < sql_max - 1) sql_out[pos++] = *p;
            }
            p++;
        }
        sql_out[pos] = '\0';
        if (*p == '"') p++;
        while (*p && *p != ')') p++;
        if (*p == ')') p++;
        return p;
    }

    sql_out[pos] = '\0';
    return NULL;
}

// Parse a section (forward or rollback) and extract all db.execute() SQL strings
static int parse_section(const char* content, const char* section_name, SqlList* out) {
    out->count = 0;

    const char* section = find_section(content, section_name);
    if (!section) return 0;

    const char* section_end = find_section_end(section);
    const char* p = section;

    while (p < section_end && out->count < MAX_STMTS) {
        // Find next db.execute(
        const char* exec = strstr(p, "db.execute(");
        if (!exec || exec >= section_end) break;

        // Skip to the opening quote
        exec += 11; // len("db.execute(")
        while (*exec == ' ' || *exec == '\t') exec++;

        const char* next = extract_execute_sql(exec, out->stmts[out->count], MAX_SQL_LEN);
        if (next && out->stmts[out->count][0]) {
            out->count++;
            p = next;
        } else {
            p = exec + 1; // skip and continue
        }
    }

    return out->count;
}

// ============================================================
// File scanning — find migration files in directory
// ============================================================

#define MAX_FILES 256

typedef struct {
    char names[MAX_FILES][256];
    int count;
} FileList;

// Compare for qsort
static int cmp_strings(const void* a, const void* b) {
    return strcmp((const char*)a, (const char*)b);
}

// Scan directory for NNNN_*.desi files, sorted by name
static int scan_migrations(const char* dir, FileList* out) {
    out->count = 0;
    DIR* d = opendir(dir);
    if (!d) return 0;

    struct dirent* entry;
    while ((entry = readdir(d)) != NULL && out->count < MAX_FILES) {
        const char* name = entry->d_name;
        // Must match pattern: 4+ digits followed by _...desi
        int digits = 0;
        while (name[digits] >= '0' && name[digits] <= '9') digits++;
        if (digits < 4) continue;
        if (name[digits] != '_') continue;
        // Check .desi extension
        int len = strlen(name);
        if (len < 5 || strcmp(name + len - 5, ".desi") != 0) continue;

        strncpy(out->names[out->count], name, 255);
        out->names[out->count][255] = '\0';
        out->count++;
    }
    closedir(d);

    // Sort by filename (numeric prefix ensures correct order)
    if (out->count > 1) {
        qsort(out->names, out->count, 256, cmp_strings);
    }

    return out->count;
}

// Find the next migration number
static int next_migration_number(const char* dir) {
    FileList files;
    int count = scan_migrations(dir, &files);
    if (count == 0) return 1;

    // Parse number from last file
    int max_num = 0;
    for (int i = 0; i < count; i++) {
        int num = atoi(files.names[i]);
        if (num > max_num) max_num = num;
    }
    return max_num + 1;
}

// ============================================================
// makemigrations — Generate .desi migration files
// ============================================================

// Operation types for building migration content
typedef struct {
    char action[32];       // "create_table", "add_column", "alter_type", "drop_column"
    char table[128];
    char column[64];
    char forward_sql[MAX_SQL_LEN];
    char rollback_sql[MAX_SQL_LEN];
} MigOp;

#define MAX_OPS 64

int32_t __db_makemigrations(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    // Ensure directory exists
    mkdir(dir, 0755);

    MigOp ops[MAX_OPS];
    int op_count = 0;
    int model_count = __orm_model_count();

    // ---- Diff models against live DB schema ----
    for (int m = 0; m < model_count && op_count < MAX_OPS; m++) {
        const char* tname = __orm_model_name(m);
        if (!tname || tname[0] == '\0') continue;

        if (!__db_table_exists(tname)) {
            // Table doesn't exist — need CREATE TABLE
            char* create_sql = __orm_create_table_sql(tname);
            if (create_sql && create_sql[0]) {
                MigOp* op = &ops[op_count++];
                strncpy(op->action, "create_table", sizeof(op->action));
                strncpy(op->table, tname, sizeof(op->table));
                op->column[0] = '\0';
                strncpy(op->forward_sql, create_sql, MAX_SQL_LEN - 1);
                snprintf(op->rollback_sql, MAX_SQL_LEN, "DROP TABLE IF EXISTS %s", tname);
            }
            if (create_sql) free(create_sql);
        } else {
            // Table exists — diff columns
            int field_count = __orm_field_count(tname);
            for (int f = 0; f < field_count && op_count < MAX_OPS; f++) {
                const char* fname = __orm_field_name(tname, f);
                if (!fname || fname[0] == '\0') continue;

                if (!column_exists(tname, fname)) {
                    // Column missing — need ADD COLUMN
                    char* alter_sql = __orm_add_column_sql(tname, fname);
                    if (alter_sql && alter_sql[0]) {
                        MigOp* op = &ops[op_count++];
                        strncpy(op->action, "add_column", sizeof(op->action));
                        strncpy(op->table, tname, sizeof(op->table));
                        strncpy(op->column, fname, sizeof(op->column));
                        strncpy(op->forward_sql, alter_sql, MAX_SQL_LEN - 1);
                        snprintf(op->rollback_sql, MAX_SQL_LEN,
                            "ALTER TABLE %s DROP COLUMN %s", tname, fname);
                    }
                    if (alter_sql) free(alter_sql);
                } else {
                    // Column exists — check type
                    char* old_type = get_column_type(tname, fname);
                    char* new_type = __orm_column_type_sql(tname, fname);
                    if (old_type && new_type && old_type[0] && new_type[0]) {
                        if (!types_match(old_type, new_type)) {
                            MigOp* op = &ops[op_count++];
                            strncpy(op->action, "alter_type", sizeof(op->action));
                            strncpy(op->table, tname, sizeof(op->table));
                            strncpy(op->column, fname, sizeof(op->column));
                            if (is_pg()) {
                                snprintf(op->forward_sql, MAX_SQL_LEN,
                                    "ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s",
                                    tname, fname, new_type, fname, new_type);
                                snprintf(op->rollback_sql, MAX_SQL_LEN,
                                    "ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s",
                                    tname, fname, old_type, fname, old_type);
                            } else {
                                snprintf(op->forward_sql, MAX_SQL_LEN,
                                    "ALTER TABLE %s MODIFY COLUMN %s %s",
                                    tname, fname, new_type);
                                snprintf(op->rollback_sql, MAX_SQL_LEN,
                                    "ALTER TABLE %s MODIFY COLUMN %s %s",
                                    tname, fname, old_type);
                            }
                        }
                    }
                    if (old_type) free(old_type);
                    if (new_type) free(new_type);
                }
            }

            // Check for orphaned columns
            int db_cols = get_table_columns(tname);
            if (db_cols > 0) {
                int col_count = __db_row_count();
                char* col_names[64];
                int nc = col_count < 64 ? col_count : 64;
                for (int c = 0; c < nc; c++) {
                    col_names[c] = __db_get_value_at(c, 0);
                }
                int field_count2 = __orm_field_count(tname);
                for (int c = 0; c < nc && op_count < MAX_OPS; c++) {
                    if (!col_names[c] || col_names[c][0] == '\0') { free(col_names[c]); continue; }
                    int found = 0;
                    for (int f = 0; f < field_count2; f++) {
                        const char* mf = __orm_field_name(tname, f);
                        if (mf && strcmp(mf, col_names[c]) == 0) { found = 1; break; }
                    }
                    if (!found) {
                        char* old_type = get_column_type(tname, col_names[c]);
                        MigOp* op = &ops[op_count++];
                        strncpy(op->action, "drop_column", sizeof(op->action));
                        strncpy(op->table, tname, sizeof(op->table));
                        strncpy(op->column, col_names[c], sizeof(op->column));
                        snprintf(op->forward_sql, MAX_SQL_LEN,
                            "ALTER TABLE %s DROP COLUMN %s", tname, col_names[c]);
                        snprintf(op->rollback_sql, MAX_SQL_LEN,
                            "ALTER TABLE %s ADD COLUMN %s %s",
                            tname, col_names[c], old_type ? old_type : "TEXT");
                        if (old_type) free(old_type);
                    }
                    free(col_names[c]);
                }
            }
        }
    }

    if (op_count == 0) {
        fprintf(stderr, "[makemigrations] No changes detected\n");
        return 0;
    }

    // ---- Generate description from operations ----
    char description[256] = "";
    if (next_migration_number(dir) == 1) {
        strcpy(description, "initial");
    } else if (op_count == 1) {
        if (strcmp(ops[0].action, "create_table") == 0) {
            snprintf(description, sizeof(description), "create_%s", ops[0].table);
        } else if (strcmp(ops[0].action, "add_column") == 0) {
            snprintf(description, sizeof(description), "add_%s_to_%s", ops[0].column, ops[0].table);
        } else if (strcmp(ops[0].action, "drop_column") == 0) {
            snprintf(description, sizeof(description), "drop_%s_from_%s", ops[0].column, ops[0].table);
        } else if (strcmp(ops[0].action, "alter_type") == 0) {
            snprintf(description, sizeof(description), "alter_%s_in_%s", ops[0].column, ops[0].table);
        }
    } else {
        snprintf(description, sizeof(description), "auto_%d_operations", op_count);
    }

    // ---- Build file path ----
    int num = next_migration_number(dir);
    char filename[256];
    snprintf(filename, sizeof(filename), "%04d_%s.desi", num, description);

    char filepath[512];
    snprintf(filepath, sizeof(filepath), "%s/%s", dir, filename);

    // ---- Write the .desi migration file ----
    FILE* f = fopen(filepath, "w");
    if (!f) {
        fprintf(stderr, "[makemigrations] Failed to create %s\n", filepath);
        return -1;
    }

    // Get current timestamp
    time_t now = time(NULL);
    struct tm* tm = localtime(&now);
    char timestamp[64];
    strftime(timestamp, sizeof(timestamp), "%Y-%m-%dT%H:%M:%S", tm);

    // Find previous migration for depends
    char depends[256] = "none";
    if (num > 1) {
        FileList files;
        scan_migrations(dir, &files);
        if (files.count > 0) {
            // Remove .desi extension for depends
            strncpy(depends, files.names[files.count - 1], sizeof(depends));
            char* dot = strstr(depends, ".desi");
            if (dot) *dot = '\0';
        }
    }

    // Write header
    fprintf(f, "# Migration: %04d_%s\n", num, description);
    fprintf(f, "# Generated: %s\n", timestamp);
    fprintf(f, "# Depends: %s\n", depends);
    fprintf(f, "#\n");
    fprintf(f, "# Operations:\n");
    for (int i = 0; i < op_count; i++) {
        if (ops[i].column[0]) {
            fprintf(f, "#   %s(%s.%s)\n", ops[i].action, ops[i].table, ops[i].column);
        } else {
            fprintf(f, "#   %s(%s)\n", ops[i].action, ops[i].table);
        }
    }
    fprintf(f, "\n");
    fprintf(f, "import db\n");
    fprintf(f, "\n");

    // Write forward()
    fprintf(f, "def forward():\n");
    if (op_count == 0) {
        fprintf(f, "\tpass\n");
    } else {
        for (int i = 0; i < op_count; i++) {
            fprintf(f, "\t# %s", ops[i].action);
            if (ops[i].column[0]) fprintf(f, ": %s.%s", ops[i].table, ops[i].column);
            else fprintf(f, ": %s", ops[i].table);
            fprintf(f, "\n");

            // Use triple-quotes for multi-line SQL (CREATE TABLE), regular for single-line
            if (strchr(ops[i].forward_sql, '\n')) {
                fprintf(f, "\tdb.execute(\"\"\"%s\"\"\")\n", ops[i].forward_sql);
            } else {
                fprintf(f, "\tdb.execute(\"%s\")\n", ops[i].forward_sql);
            }
            if (i < op_count - 1) fprintf(f, "\n");
        }
    }

    fprintf(f, "\n");

    // Write rollback() — operations in reverse order
    fprintf(f, "def rollback():\n");
    if (op_count == 0) {
        fprintf(f, "\tpass\n");
    } else {
        for (int i = op_count - 1; i >= 0; i--) {
            fprintf(f, "\t# undo %s", ops[i].action);
            if (ops[i].column[0]) fprintf(f, ": %s.%s", ops[i].table, ops[i].column);
            else fprintf(f, ": %s", ops[i].table);
            fprintf(f, "\n");

            if (strchr(ops[i].rollback_sql, '\n')) {
                fprintf(f, "\tdb.execute(\"\"\"%s\"\"\")\n", ops[i].rollback_sql);
            } else {
                fprintf(f, "\tdb.execute(\"%s\")\n", ops[i].rollback_sql);
            }
            if (i > 0) fprintf(f, "\n");
        }
    }

    fprintf(f, "\n");
    fclose(f);

    fprintf(stderr, "[makemigrations] Created %s (%d operation(s))\n", filepath, op_count);
    return op_count;
}

// ============================================================
// migrate — Apply pending .desi migration files
// ============================================================

int32_t __db_migrate_files(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    FileList files;
    int total = scan_migrations(dir, &files);
    if (total == 0) {
        fprintf(stderr, "[migrate] No migration files found in %s/\n", dir);
        return 0;
    }

    int applied = 0;

    for (int i = 0; i < total; i++) {
        // Check if already applied
        if (migration_applied(files.names[i])) continue;

        // Build full path
        char path[512];
        snprintf(path, sizeof(path), "%s/%s", dir, files.names[i]);

        // Read and parse the file
        char* content = read_file(path);
        if (!content) {
            fprintf(stderr, "[migrate] Failed to read %s\n", path);
            continue;
        }

        SqlList forward_stmts;
        SqlList rollback_stmts;
        parse_section(content, "forward", &forward_stmts);
        parse_section(content, "rollback", &rollback_stmts);

        if (forward_stmts.count == 0) {
            fprintf(stderr, "[migrate] No forward SQL in %s — skipping\n", files.names[i]);
            free(content);
            continue;
        }

        // Execute all forward statements
        int all_ok = 1;
        // Collect all forward + rollback SQL for recording
        char all_forward[32768] = "";
        char all_rollback[32768] = "";

        for (int s = 0; s < forward_stmts.count; s++) {
            int result = __db_execute_stmt(forward_stmts.stmts[s]);
            if (result < 0) {
                fprintf(stderr, "[migrate] %s: statement %d FAILED\n", files.names[i], s + 1);
                all_ok = 0;
                break;
            }
            // Accumulate SQL
            if (s > 0) strcat(all_forward, "\n---\n");
            strncat(all_forward, forward_stmts.stmts[s],
                sizeof(all_forward) - strlen(all_forward) - 10);
        }

        if (all_ok) {
            // Build rollback SQL string
            for (int s = 0; s < rollback_stmts.count; s++) {
                if (s > 0) strcat(all_rollback, "\n---\n");
                strncat(all_rollback, rollback_stmts.stmts[s],
                    sizeof(all_rollback) - strlen(all_rollback) - 10);
            }

            record_migration(files.names[i], all_forward, all_rollback);
            applied++;
            fprintf(stderr, "[migrate] Applied %s ✓\n", files.names[i]);
        }

        free(content);
    }

    if (applied == 0) {
        fprintf(stderr, "[migrate] No pending migrations\n");
    } else {
        fprintf(stderr, "[migrate] Applied %d migration(s)\n", applied);
    }

    return applied;
}

// ============================================================
// rollback — Undo the last applied migration file
// ============================================================

int32_t __db_rollback_file(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    // Find the last applied migration
    int rows = __db_query_exec(
        "SELECT filename FROM _desi_migrations ORDER BY id DESC LIMIT 1");
    if (rows <= 0) {
        fprintf(stderr, "[rollback] No migrations to roll back\n");
        return 0;
    }

    char* filename = __db_get_value_at(0, 0);
    if (!filename || filename[0] == '\0') {
        fprintf(stderr, "[rollback] No migration filename found\n");
        if (filename) free(filename);
        return 0;
    }

    // Read the migration file
    char path[512];
    snprintf(path, sizeof(path), "%s/%s", dir, filename);

    char* content = read_file(path);
    if (!content) {
        fprintf(stderr, "[rollback] Failed to read %s\n", path);
        free(filename);
        return -1;
    }

    // Parse rollback section
    SqlList rollback_stmts;
    parse_section(content, "rollback", &rollback_stmts);

    if (rollback_stmts.count == 0) {
        fprintf(stderr, "[rollback] No rollback SQL in %s\n", filename);
        free(content);
        free(filename);
        return 0;
    }

    // Execute rollback statements
    int rolled_back = 0;
    for (int s = 0; s < rollback_stmts.count; s++) {
        int result = __db_execute_stmt(rollback_stmts.stmts[s]);
        if (result >= 0) {
            rolled_back++;
        } else {
            fprintf(stderr, "[rollback] Statement %d FAILED in %s\n", s + 1, filename);
        }
    }

    // Remove the migration record
    char del[512];
    char fn_esc[512];
    sql_escape(filename, fn_esc, sizeof(fn_esc));
    snprintf(del, sizeof(del),
        "DELETE FROM _desi_migrations WHERE filename = '%s'", fn_esc);
    __db_execute_stmt(del);

    fprintf(stderr, "[rollback] Rolled back %s (%d operation(s)) ✓\n", filename, rolled_back);

    free(content);
    free(filename);
    return rolled_back;
}

// ============================================================
// migration_status — Show applied vs pending migrations
// ============================================================

char* __db_migration_status_files(const char* dir) {
    if (!__db_is_connected() || !dir) return strdup("Not connected");

    ensure_tracking();

    char* buf = malloc(8192);
    int pos = 0;
    pos += sprintf(buf + pos, "Migration Status\n");
    pos += sprintf(buf + pos, "─────────────────────────────────────────\n");

    FileList files;
    int total = scan_migrations(dir, &files);

    if (total == 0) {
        pos += sprintf(buf + pos, "  No migration files found in %s/\n", dir);
        buf[pos] = '\0';
        return buf;
    }

    for (int i = 0; i < total; i++) {
        int applied = migration_applied(files.names[i]);
        pos += sprintf(buf + pos, "  %s  %s\n",
            applied ? "✓" : "○",
            files.names[i]);
    }

    buf[pos] = '\0';
    return buf;
}
