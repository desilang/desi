/*
 * db_migrate.c — Django-style file-based migration system for Desi
 *
 * Provides:
 *   - makemigrations(dir): Generate .desi migration files from ORM model diffs
 *   - migrate(dir):        Apply pending .desi migration files
 *   - rollback(dir):       Rollback the last applied migration file
 *   - migration_status(dir): Show applied/pending migrations
 *
 * Migration files use the operation-based DSL (db.op_create_table, etc.)
 * which generates database-specific SQL at apply-time. Legacy db.execute()
 * format is also supported for backward compatibility.
 *
 * Tracking is stored in the _desi_migrations table in the connected database.
 *
 * Phase 1: Dynamic memory, SQL injection prevention, transactional DDL
 * Phase 2: Operation-based DSL (db.op_*) for portable migrations
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdarg.h>
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

// ---- External: transactions ----
extern int32_t __db_begin(void);
extern int32_t __db_commit(void);
extern int32_t __db_rollback_tx(void);

// ---- External: ORM model registry ----
extern int32_t __orm_model_count(void);
extern char*   __orm_create_table_sql(const char* table_name);
extern char*   __orm_add_column_sql(const char* table_name, const char* col_name);
extern int32_t __orm_field_count(const char* table_name);
extern const char* __orm_model_name(int32_t index);
extern const char* __orm_field_name(const char* table_name, int32_t field_index);
extern char*   __orm_column_type_sql(const char* table_name, const char* col_name);
extern char*   __orm_drop_table_sql(const char* table_name);
extern char*   __orm_field_spec(const char* table_name, int32_t field_index);

// ---- External: active driver ----
extern char* __db_driver(void);

// ============================================================
// DynBuf — Growable byte buffer
//
// Replaces all fixed char[] arrays. Starts at a small initial
// capacity and grows via realloc. Always null-terminated.
// Stack cost: 24 bytes (3 ints/pointers) per buffer.
// ============================================================

typedef struct {
    char* data;
    int   len;
    int   cap;
} DynBuf;

static DynBuf dynbuf_new(int initial_cap) {
    DynBuf b;
    b.cap = initial_cap > 16 ? initial_cap : 16;
    b.data = (char*)malloc(b.cap);
    b.data[0] = '\0';
    b.len = 0;
    return b;
}

static void dynbuf_ensure(DynBuf* b, int needed) {
    if (b->len + needed + 1 > b->cap) {
        while (b->cap < b->len + needed + 1) b->cap *= 2;
        b->data = (char*)realloc(b->data, b->cap);
    }
}

static void dynbuf_append(DynBuf* b, const char* s) {
    int slen = (int)strlen(s);
    dynbuf_ensure(b, slen);
    memcpy(b->data + b->len, s, slen);
    b->len += slen;
    b->data[b->len] = '\0';
}

static void dynbuf_append_char(DynBuf* b, char c) {
    dynbuf_ensure(b, 1);
    b->data[b->len++] = c;
    b->data[b->len] = '\0';
}

static void dynbuf_appendf(DynBuf* b, const char* fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    // Measure required size
    va_list ap2;
    va_copy(ap2, ap);
    int needed = vsnprintf(NULL, 0, fmt, ap2);
    va_end(ap2);
    if (needed < 0) { va_end(ap); return; }
    dynbuf_ensure(b, needed);
    vsnprintf(b->data + b->len, needed + 1, fmt, ap);
    b->len += needed;
    va_end(ap);
}

static void dynbuf_reset(DynBuf* b) {
    b->len = 0;
    b->data[0] = '\0';
}

static void dynbuf_free(DynBuf* b) {
    if (b->data) { free(b->data); b->data = NULL; }
    b->len = b->cap = 0;
}

// ============================================================
// StrList — Growable list of heap-allocated strings
//
// Replaces FileList (char names[256][256]) and SqlList
// (char stmts[64][8192]). No fixed limits.
// Stack cost: 12 bytes per list.
// ============================================================

typedef struct {
    char** items;
    int    count;
    int    cap;
} StrList;

static StrList strlist_new(int initial_cap) {
    StrList sl;
    sl.cap = initial_cap > 4 ? initial_cap : 4;
    sl.items = (char**)malloc(sl.cap * sizeof(char*));
    sl.count = 0;
    return sl;
}

static void strlist_push(StrList* sl, const char* s) {
    if (sl->count >= sl->cap) {
        sl->cap *= 2;
        sl->items = (char**)realloc(sl->items, sl->cap * sizeof(char*));
    }
    sl->items[sl->count++] = strdup(s);
}

static void strlist_free(StrList* sl) {
    for (int i = 0; i < sl->count; i++) free(sl->items[i]);
    free(sl->items);
    sl->items = NULL;
    sl->count = sl->cap = 0;
}

// ============================================================
// MigOp — Migration operation (heap-allocated)
//
// Replaces MigOp[MAX_OPS] with no fixed SQL length limit.
// forward_sql/rollback_sql are used for legacy db.execute() format.
// field_specs/old_type are used for the new db.op_* DSL format.
// ============================================================

typedef struct {
    char  action[32];
    char  table[128];
    char  column[64];
    char* forward_sql;     // heap-allocated, any length (legacy fallback)
    char* rollback_sql;    // heap-allocated, any length (legacy fallback)
    char* field_specs;     // for create_table: newline-separated field specs
    char* old_type;        // for alter_type: old type name (for rollback)
    char* new_type;        // for alter_type: new type name (for forward)
} MigOp;

typedef struct {
    MigOp** items;
    int     count;
    int     cap;
} OpList;

static OpList oplist_new(int initial_cap) {
    OpList ol;
    ol.cap = initial_cap > 8 ? initial_cap : 8;
    ol.items = (MigOp**)malloc(ol.cap * sizeof(MigOp*));
    ol.count = 0;
    return ol;
}

static MigOp* oplist_add(OpList* ol) {
    if (ol->count >= ol->cap) {
        ol->cap *= 2;
        ol->items = (MigOp**)realloc(ol->items, ol->cap * sizeof(MigOp*));
    }
    MigOp* op = (MigOp*)calloc(1, sizeof(MigOp));
    ol->items[ol->count++] = op;
    return op;
}

static void oplist_free(OpList* ol) {
    for (int i = 0; i < ol->count; i++) {
        if (ol->items[i]->forward_sql) free(ol->items[i]->forward_sql);
        if (ol->items[i]->rollback_sql) free(ol->items[i]->rollback_sql);
        if (ol->items[i]->field_specs) free(ol->items[i]->field_specs);
        if (ol->items[i]->old_type) free(ol->items[i]->old_type);
        if (ol->items[i]->new_type) free(ol->items[i]->new_type);
        free(ol->items[i]);
    }
    free(ol->items);
    ol->items = NULL;
    ol->count = ol->cap = 0;
}

// ============================================================
// Migration tracking table
// ============================================================

static int g_tracking_ready = 0;
static int g_cached_driver = -1;  // -1 = not cached, 0 = mysql, 1 = pg

static int is_pg(void) {
    if (g_cached_driver < 0) {
        char* drv = __db_driver();
        g_cached_driver = (strcmp(drv, "postgres") == 0) ? 1 : 0;
        free(drv);
    }
    return g_cached_driver;
}

// Reset migration state — called from dispatch.c on close/reconnect
void __migrate_reset_state(void) {
    g_tracking_ready = 0;
    g_cached_driver = -1;
}

// Escape single quotes for SQL string interpolation.
// Returns a heap-allocated string — caller must free.
static char* sql_escape_alloc(const char* src) {
    if (!src) return strdup("");
    int src_len = (int)strlen(src);
    // Worst case: every char is a quote (doubles in size)
    char* dst = (char*)malloc(src_len * 2 + 1);
    int j = 0;
    for (int i = 0; i < src_len; i++) {
        if (src[i] == '\'') dst[j++] = '\'';
        dst[j++] = src[i];
    }
    dst[j] = '\0';
    return dst;
}

// Forward declarations for functions used before their definitions
static int column_exists(const char* table_name, const char* col_name);
static void record_migration_with_app(const char* app_label, const char* filename,
                                       const char* applied_sql, const char* rollback_sql);

// Create/ensure _desi_migrations table with app_label column.
// Auto-migrates existing tables that lack the app_label column.
static void ensure_tracking(void) {
    if (g_tracking_ready) return;

    const char* sql;
    if (is_pg()) {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id SERIAL PRIMARY KEY, "
              "app_label VARCHAR(128) NOT NULL DEFAULT '', "
              "filename VARCHAR(256) NOT NULL, "
              "applied_sql TEXT NOT NULL, "
              "rollback_sql TEXT NOT NULL, "
              "applied_at TIMESTAMPTZ DEFAULT NOW())";
    } else {
        sql = "CREATE TABLE IF NOT EXISTS _desi_migrations ("
              "id INT AUTO_INCREMENT PRIMARY KEY, "
              "app_label VARCHAR(128) NOT NULL DEFAULT '', "
              "filename VARCHAR(256) NOT NULL, "
              "applied_sql TEXT NOT NULL, "
              "rollback_sql TEXT NOT NULL, "
              "applied_at DATETIME DEFAULT CURRENT_TIMESTAMP) "
              "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4";
    }
    __db_execute_stmt(sql);

    // Auto-migrate: add app_label column if it doesn't exist on an older table.
    // We check information_schema — if app_label is missing, ALTER TABLE.
    if (!column_exists("_desi_migrations", "app_label")) {
        if (is_pg()) {
            __db_execute_stmt(
                "ALTER TABLE _desi_migrations ADD COLUMN app_label VARCHAR(128) NOT NULL DEFAULT ''");
        } else {
            __db_execute_stmt(
                "ALTER TABLE _desi_migrations ADD COLUMN app_label VARCHAR(128) NOT NULL DEFAULT '' AFTER id");
        }
        printf("[migrate] Auto-migrated _desi_migrations: added app_label column\n");
        fflush(stdout);
    }

    g_tracking_ready = 1;
}

// Extract app label from a directory path.
// Uses the parent directory name: "accounts/migrations" → "accounts".
// Falls back to the directory name itself: "migrations" → "migrations".
// Returns a heap-allocated string — caller must free.
static char* extract_app_label(const char* dir) {
    if (!dir || !dir[0]) return strdup("");

    // Normalize: strip trailing slashes
    int len = (int)strlen(dir);
    char normalized[512];
    snprintf(normalized, sizeof(normalized), "%s", dir);
    while (len > 0 && (normalized[len-1] == '/' || normalized[len-1] == '\\')) {
        normalized[--len] = '\0';
    }

    // Find the last path component
    const char* last = strrchr(normalized, '/');
    if (!last) last = strrchr(normalized, '\\');
    const char* last_component = last ? last + 1 : normalized;

    // If last component is "migrations", use the parent directory name
    if (strcmp(last_component, "migrations") == 0 && last) {
        // Back up to find the parent
        char parent[512];
        int plen = (int)(last - normalized);
        if (plen > 0 && plen < (int)sizeof(parent)) {
            snprintf(parent, plen + 1, "%s", normalized);
            const char* p2 = strrchr(parent, '/');
            if (!p2) p2 = strrchr(parent, '\\');
            return strdup(p2 ? p2 + 1 : parent);
        }
    }

    return strdup(last_component);
}

// Check if a migration file has already been applied (escaped).
// When app_label is non-empty, filters by app_label as well.
static int migration_applied(const char* filename) {
    char* fn_esc = sql_escape_alloc(filename);
    DynBuf sql = dynbuf_new(256);
    dynbuf_appendf(&sql,
        "SELECT 1 FROM _desi_migrations WHERE filename = '%s'", fn_esc);
    free(fn_esc);
    int rows = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    return rows > 0 ? 1 : 0;
}

// Check if a migration is applied for a specific app
static int migration_applied_for_app(const char* app_label, const char* filename) {
    char* fn_esc = sql_escape_alloc(filename);
    char* app_esc = sql_escape_alloc(app_label);
    DynBuf sql = dynbuf_new(256);
    dynbuf_appendf(&sql,
        "SELECT 1 FROM _desi_migrations WHERE app_label = '%s' AND filename = '%s'",
        app_esc, fn_esc);
    free(fn_esc);
    free(app_esc);
    int rows = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    return rows > 0 ? 1 : 0;
}

// Record a migration as applied (all inputs escaped, dynamic buffers)
static void record_migration(const char* filename, const char* applied_sql, const char* rollback_sql) {
    record_migration_with_app("", filename, applied_sql, rollback_sql);
}

// Record a migration with app_label
static void record_migration_with_app(const char* app_label, const char* filename,
                                       const char* applied_sql, const char* rollback_sql) {
    char* app_esc = sql_escape_alloc(app_label);
    char* fn_esc = sql_escape_alloc(filename);
    char* app_sql_esc = sql_escape_alloc(applied_sql);
    char* rb_esc = sql_escape_alloc(rollback_sql);

    DynBuf insert = dynbuf_new(512);
    dynbuf_appendf(&insert,
        "INSERT INTO _desi_migrations (app_label, filename, applied_sql, rollback_sql) "
        "VALUES ('%s', '%s', '%s', '%s')",
        app_esc, fn_esc, app_sql_esc, rb_esc);

    __db_execute_stmt(insert.data);

    dynbuf_free(&insert);
    free(app_esc);
    free(fn_esc);
    free(app_sql_esc);
    free(rb_esc);
}

// ============================================================
// Schema introspection
// ============================================================

// Check if a table exists
int32_t __db_table_exists(const char* table_name) {
    if (!__db_is_connected() || !table_name) return 0;

    char* tn_esc = sql_escape_alloc(table_name);
    DynBuf sql = dynbuf_new(256);
    if (is_pg()) {
        dynbuf_appendf(&sql,
            "SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '%s'",
            tn_esc);
    } else {
        dynbuf_appendf(&sql,
            "SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'",
            tn_esc);
    }
    free(tn_esc);
    int rows = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    return rows > 0 ? 1 : 0;
}

// Check if a column exists in a table
static int column_exists(const char* table_name, const char* col_name) {
    char* tn_esc = sql_escape_alloc(table_name);
    char* cn_esc = sql_escape_alloc(col_name);
    DynBuf sql = dynbuf_new(256);
    if (is_pg()) {
        dynbuf_appendf(&sql,
            "SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s'",
            tn_esc, cn_esc);
    } else {
        dynbuf_appendf(&sql,
            "SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'",
            tn_esc, cn_esc);
    }
    free(tn_esc);
    free(cn_esc);
    int result = __db_query_exec(sql.data) > 0 ? 1 : 0;
    dynbuf_free(&sql);
    return result;
}

// Get actual SQL type of a column from information_schema
static char* get_column_type(const char* table_name, const char* col_name) {
    char* tn_esc = sql_escape_alloc(table_name);
    char* cn_esc = sql_escape_alloc(col_name);
    DynBuf sql = dynbuf_new(256);
    if (is_pg()) {
        dynbuf_appendf(&sql,
            "SELECT data_type FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' AND column_name = '%s'",
            tn_esc, cn_esc);
    } else {
        dynbuf_appendf(&sql,
            "SELECT column_type FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' AND column_name = '%s'",
            tn_esc, cn_esc);
    }
    free(tn_esc);
    free(cn_esc);
    int rows = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    if (rows > 0) return __db_get_value_at(0, 0);
    return strdup("");
}

// Get all column names in a table
static int get_table_columns(const char* table_name) {
    char* tn_esc = sql_escape_alloc(table_name);
    DynBuf sql = dynbuf_new(256);
    if (is_pg()) {
        dynbuf_appendf(&sql,
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' ORDER BY ordinal_position",
            tn_esc);
    } else {
        dynbuf_appendf(&sql,
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' ORDER BY ordinal_position",
            tn_esc);
    }
    free(tn_esc);
    int rows = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    return rows;
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
// Phase 2: Operation-Based Migration DSL
//
// Provides db.op_* functions that generate database-specific SQL
// at apply-time, making migration files portable across PG/MySQL.
//
// Field Spec Format (colon-delimited):
//   "name:type"                     → NOT NULL
//   "name:type:nullable"            → NULL
//   "name:type:unique"              → NOT NULL UNIQUE
//   "name:type:default(val)"        → NOT NULL DEFAULT val
//   "name:fk(table.field):cascade"  → FK with ON DELETE CASCADE
//   "name:auto"                     → auto-increment PK
//
// Supported types: auto, bigauto, int, bigint, varchar(N), text,
//   bool, float, double, datetime, date, json, uuid, inet,
//   decimal(P,S), fk(table.field)
// ============================================================

// Parse a single field spec string into a SQL column definition.
// dialect: 0 = PG, nonzero = MySQL.
// Returns heap-allocated SQL fragment — caller must free.
static char* field_spec_to_sql(const char* spec, int dialect) {
    if (!spec || !spec[0]) return strdup("");

    // Work on a copy — we'll mutate it for tokenizing
    char copy[512];
    snprintf(copy, sizeof(copy), "%s", spec);

    // Split by ':' — but be careful not to split inside parens.
    // We tokenize manually to handle nested parens in types like
    // fk(users.id) and decimal(10,2).
    char* parts[16];
    int nparts = 0;
    char* p = copy;
    while (*p && nparts < 16) {
        parts[nparts++] = p;
        int depth = 0;
        while (*p) {
            if (*p == '(') depth++;
            else if (*p == ')') depth--;
            else if (*p == ':' && depth == 0) {
                *p = '\0';
                p++;
                break;
            }
            p++;
        }
    }

    if (nparts < 2) return strdup(""); // need at least name:type

    const char* name = parts[0];
    const char* type = parts[1];

    // Scan modifiers
    int nullable = 0, unique = 0, auto_now = 0;
    const char* default_val = NULL;
    const char* on_delete = NULL;

    for (int i = 2; i < nparts; i++) {
        if (strcmp(parts[i], "nullable") == 0) nullable = 1;
        else if (strcmp(parts[i], "unique") == 0) unique = 1;
        else if (strcmp(parts[i], "auto_now") == 0 ||
                 strcmp(parts[i], "auto_now_add") == 0) auto_now = 1;
        else if (strncmp(parts[i], "default(", 8) == 0) {
            default_val = parts[i] + 8;
            // Remove trailing ')'
            char* paren = strrchr(parts[i] + 8, ')');
            if (paren) *paren = '\0';
        }
        else if (strcmp(parts[i], "cascade") == 0) on_delete = "CASCADE";
        else if (strcmp(parts[i], "protect") == 0 ||
                 strcmp(parts[i], "restrict") == 0)  on_delete = "RESTRICT";
        else if (strcmp(parts[i], "setnull") == 0 ||
                 strcmp(parts[i], "set_null") == 0)  { on_delete = "SET NULL"; nullable = 1; }
        else if (strcmp(parts[i], "setdefault") == 0 ||
                 strcmp(parts[i], "set_default") == 0) on_delete = "SET DEFAULT";
        else if (strcmp(parts[i], "noaction") == 0 ||
                 strcmp(parts[i], "no_action") == 0)   on_delete = "NO ACTION";
    }

    DynBuf col = dynbuf_new(128);

    // ---- Handle auto-increment PK types (no modifiers) ----
    if (strcmp(type, "auto") == 0) {
        if (dialect == 0) dynbuf_appendf(&col, "%s SERIAL PRIMARY KEY", name);
        else dynbuf_appendf(&col, "%s INT AUTO_INCREMENT PRIMARY KEY", name);
        char* result = col.data; col.data = NULL; return result;
    }
    if (strcmp(type, "bigauto") == 0) {
        if (dialect == 0) dynbuf_appendf(&col, "%s BIGSERIAL PRIMARY KEY", name);
        else dynbuf_appendf(&col, "%s BIGINT AUTO_INCREMENT PRIMARY KEY", name);
        char* result = col.data; col.data = NULL; return result;
    }

    // ---- Map type keyword to SQL type ----
    dynbuf_appendf(&col, "%s ", name);

    if (strcmp(type, "int") == 0)            dynbuf_append(&col, "INTEGER");
    else if (strcmp(type, "bigint") == 0)    dynbuf_append(&col, "BIGINT");
    else if (strncmp(type, "varchar(", 8) == 0) {
        // varchar(100) → VARCHAR(100)
        dynbuf_appendf(&col, "VARCHAR(%s", type + 8);
    }
    else if (strcmp(type, "text") == 0)      dynbuf_append(&col, "TEXT");
    else if (strcmp(type, "bool") == 0) {
        if (dialect == 0) dynbuf_append(&col, "BOOLEAN");
        else              dynbuf_append(&col, "TINYINT(1)");
    }
    else if (strcmp(type, "float") == 0)     dynbuf_append(&col, "REAL");
    else if (strcmp(type, "double") == 0) {
        if (dialect == 0) dynbuf_append(&col, "DOUBLE PRECISION");
        else              dynbuf_append(&col, "DOUBLE");
    }
    else if (strcmp(type, "datetime") == 0) {
        if (dialect == 0) dynbuf_append(&col, "TIMESTAMPTZ");
        else              dynbuf_append(&col, "DATETIME");
    }
    else if (strcmp(type, "date") == 0)      dynbuf_append(&col, "DATE");
    else if (strcmp(type, "json") == 0) {
        if (dialect == 0) dynbuf_append(&col, "JSONB");
        else              dynbuf_append(&col, "JSON");
    }
    else if (strcmp(type, "uuid") == 0) {
        if (dialect == 0) dynbuf_append(&col, "UUID");
        else              dynbuf_append(&col, "VARCHAR(36)");
    }
    else if (strcmp(type, "inet") == 0) {
        if (dialect == 0) dynbuf_append(&col, "INET");
        else              dynbuf_append(&col, "VARCHAR(45)");
    }
    else if (strncmp(type, "decimal(", 8) == 0) {
        dynbuf_appendf(&col, "DECIMAL(%s", type + 8);
    }
    else if (strncmp(type, "fk(", 3) == 0) {
        // Foreign key — column type is INTEGER
        dynbuf_append(&col, "INTEGER");
    }
    else if (strncmp(type, "generated(", 10) == 0) {
        // Generated/computed column — pass through raw SQL type
        // Format: generated(expression,sql_type)
        char gen_buf[256];
        snprintf(gen_buf, sizeof(gen_buf), "%s", type + 10);
        char* paren = strrchr(gen_buf, ')');
        if (paren) *paren = '\0';
        char* comma = strrchr(gen_buf, ',');
        if (comma) {
            *comma = '\0';
            const char* expr = gen_buf;
            const char* sql_type = comma + 1;
            dynbuf_reset(&col);
            dynbuf_appendf(&col, "%s %s GENERATED ALWAYS AS (%s) STORED", name, sql_type, expr);
            char* result = col.data; col.data = NULL; return result;
        }
        dynbuf_append(&col, "TEXT"); // fallback
    }
    else {
        // Custom/unknown type — pass through verbatim
        dynbuf_append(&col, type);
    }

    // ---- Modifiers ----
    if (!nullable) dynbuf_append(&col, " NOT NULL");
    if (unique)    dynbuf_append(&col, " UNIQUE");
    if (default_val) dynbuf_appendf(&col, " DEFAULT %s", default_val);
    if (auto_now) {
        if (dialect == 0) dynbuf_append(&col, " DEFAULT NOW()");
        else              dynbuf_append(&col, " DEFAULT CURRENT_TIMESTAMP");
    }

    // ---- FK REFERENCES clause ----
    if (strncmp(type, "fk(", 3) == 0) {
        char fk_buf[256];
        snprintf(fk_buf, sizeof(fk_buf), "%s", type + 3);
        char* paren = strchr(fk_buf, ')');
        if (paren) *paren = '\0';
        char* dot = strchr(fk_buf, '.');
        if (dot) {
            *dot = '\0';
            dynbuf_appendf(&col, " REFERENCES %s(%s)", fk_buf, dot + 1);
        }
        if (on_delete) dynbuf_appendf(&col, " ON DELETE %s", on_delete);
    }

    char* result = col.data;
    col.data = NULL;
    return result;
}

// ---- SQL generators for each operation ----
// These return heap-allocated SQL strings. Caller must free.

// Generate CREATE TABLE SQL from newline-separated field specs
static char* op_create_table_sql(const char* table_name, const char* fields_str, int dialect) {
    if (!table_name || !fields_str) return strdup("");

    DynBuf sql = dynbuf_new(512);
    dynbuf_appendf(&sql, "CREATE TABLE IF NOT EXISTS %s (\n", table_name);

    // Split fields_str by newlines
    char* fields_copy = strdup(fields_str);
    int first = 1;
    char* line = fields_copy;

    while (*line) {
        // Find end of this line
        char* eol = strchr(line, '\n');
        if (eol) *eol = '\0';

        // Trim leading whitespace
        while (*line == ' ' || *line == '\t') line++;

        // Skip empty lines
        if (*line != '\0') {
            if (!first) dynbuf_append(&sql, ",\n");
            first = 0;

            char* col_sql = field_spec_to_sql(line, dialect);
            dynbuf_appendf(&sql, "    %s", col_sql);
            free(col_sql);
        }

        if (eol) line = eol + 1;
        else break;
    }

    free(fields_copy);
    dynbuf_append(&sql, "\n)");

    if (dialect != 0) { // MySQL
        dynbuf_append(&sql, " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4");
    }

    char* result = sql.data;
    sql.data = NULL;
    return result;
}

// Generate DROP TABLE SQL
static char* op_drop_table_sql(const char* table_name) {
    DynBuf sql = dynbuf_new(64);
    dynbuf_appendf(&sql, "DROP TABLE IF EXISTS %s", table_name);
    char* result = sql.data; sql.data = NULL; return result;
}

// Generate ALTER TABLE ADD COLUMN from a field spec
static char* op_add_column_sql(const char* table_name, const char* field_spec, int dialect) {
    char* col_sql = field_spec_to_sql(field_spec, dialect);
    DynBuf sql = dynbuf_new(256);
    dynbuf_appendf(&sql, "ALTER TABLE %s ADD COLUMN %s", table_name, col_sql);
    free(col_sql);
    char* result = sql.data; sql.data = NULL; return result;
}

// Generate ALTER TABLE DROP COLUMN
static char* op_drop_column_sql(const char* table_name, const char* column) {
    DynBuf sql = dynbuf_new(128);
    dynbuf_appendf(&sql, "ALTER TABLE %s DROP COLUMN %s", table_name, column);
    char* result = sql.data; sql.data = NULL; return result;
}

// Generate ALTER TABLE ALTER/MODIFY COLUMN
static char* op_alter_column_sql(const char* table_name, const char* column,
                                  const char* new_type, int dialect) {
    DynBuf sql = dynbuf_new(256);
    if (dialect == 0) {
        // PG: ALTER TABLE t ALTER COLUMN c TYPE new USING c::new
        dynbuf_appendf(&sql,
            "ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s",
            table_name, column, new_type, column, new_type);
    } else {
        // MySQL: ALTER TABLE t MODIFY COLUMN c new_type
        dynbuf_appendf(&sql,
            "ALTER TABLE %s MODIFY COLUMN %s %s",
            table_name, column, new_type);
    }
    char* result = sql.data; sql.data = NULL; return result;
}

// Generate ALTER TABLE RENAME COLUMN
static char* op_rename_column_sql(const char* table_name, const char* old_name,
                                   const char* new_name, int dialect) {
    DynBuf sql = dynbuf_new(128);
    (void)dialect; // syntax is the same for PG and newer MySQL
    dynbuf_appendf(&sql, "ALTER TABLE %s RENAME COLUMN %s TO %s",
        table_name, old_name, new_name);
    char* result = sql.data; sql.data = NULL; return result;
}

// Generate CREATE INDEX
static char* op_add_index_sql(const char* table_name, const char* columns, int dialect) {
    (void)dialect;
    // Build index name from table + columns
    DynBuf idx_name = dynbuf_new(128);
    dynbuf_appendf(&idx_name, "idx_%s_", table_name);

    // Sanitize column names for index name (replace commas/spaces with _)
    for (const char* c = columns; *c; c++) {
        if (*c == ',' || *c == ' ') {
            if (idx_name.data[idx_name.len - 1] != '_')
                dynbuf_append_char(&idx_name, '_');
        } else {
            dynbuf_append_char(&idx_name, *c);
        }
    }
    // Remove trailing _ if present
    if (idx_name.len > 0 && idx_name.data[idx_name.len - 1] == '_') {
        idx_name.data[--idx_name.len] = '\0';
    }

    DynBuf sql = dynbuf_new(256);
    dynbuf_appendf(&sql, "CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
        idx_name.data, table_name, columns);

    dynbuf_free(&idx_name);
    char* result = sql.data; sql.data = NULL; return result;
}

// ---- Exported __db_op_* functions ----
// These can be called from compiled Desi code or parsed from migration files.
// Each generates dialect-specific SQL and executes it.

int32_t __db_op_create_table(const char* table_name, const char* fields) {
    if (!__db_is_connected() || !table_name || !fields) return -1;
    char* sql = op_create_table_sql(table_name, fields, is_pg() ? 0 : 1);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Created table %s\n", table_name);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_drop_table(const char* table_name) {
    if (!__db_is_connected() || !table_name) return -1;
    char* sql = op_drop_table_sql(table_name);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Dropped table %s\n", table_name);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_add_column(const char* table_name, const char* field_spec) {
    if (!__db_is_connected() || !table_name || !field_spec) return -1;
    char* sql = op_add_column_sql(table_name, field_spec, is_pg() ? 0 : 1);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Added column to %s\n", table_name);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_drop_column(const char* table_name, const char* column) {
    if (!__db_is_connected() || !table_name || !column) return -1;
    char* sql = op_drop_column_sql(table_name, column);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Dropped column %s from %s\n", column, table_name);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_alter_column(const char* table_name, const char* column, const char* new_type) {
    if (!__db_is_connected() || !table_name || !column || !new_type) return -1;
    char* sql = op_alter_column_sql(table_name, column, new_type, is_pg() ? 0 : 1);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Altered column %s.%s → %s\n", table_name, column, new_type);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_rename_column(const char* table_name, const char* old_name, const char* new_name) {
    if (!__db_is_connected() || !table_name || !old_name || !new_name) return -1;
    char* sql = op_rename_column_sql(table_name, old_name, new_name, is_pg() ? 0 : 1);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Renamed column %s.%s → %s\n", table_name, old_name, new_name);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_add_index(const char* table_name, const char* columns) {
    if (!__db_is_connected() || !table_name || !columns) return -1;
    char* sql = op_add_index_sql(table_name, columns, is_pg() ? 0 : 1);
    int result = __db_execute_stmt(sql);
    if (result >= 0) {
        printf("[op] Created index on %s(%s)\n", table_name, columns);
        fflush(stdout);
    }
    free(sql);
    return result;
}

int32_t __db_op_run_sql(const char* sql_str) {
    if (!__db_is_connected() || !sql_str) return -1;
    return __db_execute_stmt(sql_str);
}

// ============================================================
// .desi Migration File Parser
//
// Extracts SQL strings from db.execute("...") / db.execute("""...""")
// calls within forward() and rollback() functions.
//
// Comment-aware: lines starting with # (after whitespace) are
// skipped, so commented-out db.execute() calls are not matched.
// ============================================================

// Read entire file into malloc'd buffer
static char* read_file(const char* path) {
    FILE* f = fopen(path, "r");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    long size = ftell(f);
    fseek(f, 0, SEEK_SET);
    if (size <= 0) {
        // Empty or unreadable file
        fclose(f);
        return strdup("");
    }
    char* buf = (char*)malloc(size + 1);
    if (!buf) { fclose(f); return NULL; }
    long nread = fread(buf, 1, size, f);
    buf[nread] = '\0';
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
// Returns pointer past the closing paren, or NULL on error.
// sql_out is a DynBuf — grows to fit any SQL length.
static const char* extract_execute_sql(const char* p, DynBuf* sql_out) {
    // Check for triple-quote
    if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
        p += 3;
        // Find closing """
        while (*p) {
            if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
                p += 3; // skip """
                // Skip to closing paren
                while (*p && *p != ')') p++;
                if (*p == ')') p++;
                return p;
            }
            dynbuf_append_char(sql_out, *p);
            p++;
        }
    } else if (*p == '"') {
        // Single-quoted string
        p++; // skip opening "
        while (*p && *p != '"') {
            if (*p == '\\' && p[1]) {
                // Handle escapes
                p++;
                if (*p == 'n') dynbuf_append_char(sql_out, '\n');
                else if (*p == 't') dynbuf_append_char(sql_out, '\t');
                else if (*p == '"') dynbuf_append_char(sql_out, '"');
                else if (*p == '\\') dynbuf_append_char(sql_out, '\\');
                else dynbuf_append_char(sql_out, *p);
            } else {
                dynbuf_append_char(sql_out, *p);
            }
            p++;
        }
        if (*p == '"') p++;
        while (*p && *p != ')') p++;
        if (*p == ')') p++;
        return p;
    }

    return NULL;
}

// Find the start of the next line from position p
static const char* next_line(const char* p) {
    while (*p && *p != '\n') p++;
    if (*p == '\n') p++;
    return p;
}

// Check if a line is a comment (first non-whitespace char is #)
static int is_comment_line(const char* line_start, const char* section_end) {
    const char* p = line_start;
    while (p < section_end && (*p == ' ' || *p == '\t')) p++;
    return (p < section_end && *p == '#');
}

// ---- Helpers for parsing db.op_* arguments from text ----

// Extract a single string argument starting at p (past the opening quote).
// Handles both "..." and """...""" forms.
// Writes the extracted content into out, returns pointer past the closing quote.
static const char* extract_string_arg(const char* p, DynBuf* out) {
    if (!p) return NULL;
    while (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r') p++;

    if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
        // Triple-quoted string
        p += 3;
        while (*p) {
            if (p[0] == '"' && p[1] == '"' && p[2] == '"') {
                p += 3;
                return p;
            }
            dynbuf_append_char(out, *p);
            p++;
        }
    } else if (*p == '"') {
        // Single-quoted string
        p++; // skip opening "
        while (*p && *p != '"') {
            if (*p == '\\' && p[1]) {
                p++;
                if (*p == 'n') dynbuf_append_char(out, '\n');
                else if (*p == 't') dynbuf_append_char(out, '\t');
                else if (*p == '"') dynbuf_append_char(out, '"');
                else if (*p == '\\') dynbuf_append_char(out, '\\');
                else dynbuf_append_char(out, *p);
            } else {
                dynbuf_append_char(out, *p);
            }
            p++;
        }
        if (*p == '"') p++;
        return p;
    }
    return NULL;
}

// Skip whitespace and optional comma between arguments
static const char* skip_arg_sep(const char* p) {
    while (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r' || *p == ',') p++;
    return p;
}

// Try to parse a db.op_* call starting at `found` (pointing to "db.op_").
// If successful, generates SQL and pushes it to `out`, returns pointer past ')'.
// Returns NULL if this is not a recognized op call.
static const char* try_parse_op_call(const char* found, const char* section_end, StrList* out) {
    const char* p = found + 6; // skip "db.op_"
    int dialect = is_pg() ? 0 : 1;

    // Extract operation name
    char op_name[64];
    int oi = 0;
    while (*p && *p != '(' && oi < 63) {
        op_name[oi++] = *p++;
    }
    op_name[oi] = '\0';

    if (*p != '(') return NULL;
    p++; // skip '('

    // Now extract arguments based on the operation type
    if (strcmp(op_name, "create_table") == 0) {
        // db.op_create_table("table", "field_specs") — 2 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(256);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p && arg1.len > 0 && arg2.len > 0) {
            char* sql = op_create_table_sql(arg1.data, arg2.data, dialect);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
    }
    else if (strcmp(op_name, "drop_table") == 0) {
        // db.op_drop_table("table") — 1 arg
        DynBuf arg1 = dynbuf_new(64);
        p = extract_string_arg(p, &arg1);
        if (p && arg1.len > 0) {
            char* sql = op_drop_table_sql(arg1.data);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
    }
    else if (strcmp(op_name, "add_column") == 0) {
        // db.op_add_column("table", "field_spec") — 2 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(128);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p && arg1.len > 0 && arg2.len > 0) {
            char* sql = op_add_column_sql(arg1.data, arg2.data, dialect);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
    }
    else if (strcmp(op_name, "drop_column") == 0) {
        // db.op_drop_column("table", "column") — 2 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(64);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p && arg1.len > 0 && arg2.len > 0) {
            char* sql = op_drop_column_sql(arg1.data, arg2.data);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
    }
    else if (strcmp(op_name, "alter_column") == 0) {
        // db.op_alter_column("table", "column", "new_type") — 3 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(64);
        DynBuf arg3 = dynbuf_new(64);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg3); }
        if (p && arg1.len > 0 && arg2.len > 0 && arg3.len > 0) {
            char* sql = op_alter_column_sql(arg1.data, arg2.data, arg3.data, dialect);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
        dynbuf_free(&arg3);
    }
    else if (strcmp(op_name, "rename_column") == 0) {
        // db.op_rename_column("table", "old", "new") — 3 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(64);
        DynBuf arg3 = dynbuf_new(64);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg3); }
        if (p && arg1.len > 0 && arg2.len > 0 && arg3.len > 0) {
            char* sql = op_rename_column_sql(arg1.data, arg2.data, arg3.data, dialect);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
        dynbuf_free(&arg3);
    }
    else if (strcmp(op_name, "add_index") == 0) {
        // db.op_add_index("table", "col1,col2") — 2 args
        DynBuf arg1 = dynbuf_new(64);
        DynBuf arg2 = dynbuf_new(128);
        p = extract_string_arg(p, &arg1);
        if (p) { p = skip_arg_sep(p); p = extract_string_arg(p, &arg2); }
        if (p && arg1.len > 0 && arg2.len > 0) {
            char* sql = op_add_index_sql(arg1.data, arg2.data, dialect);
            strlist_push(out, sql);
            free(sql);
        }
        dynbuf_free(&arg1);
        dynbuf_free(&arg2);
    }
    else if (strcmp(op_name, "run_sql") == 0) {
        // db.op_run_sql("raw sql") — 1 arg (pass-through)
        DynBuf arg1 = dynbuf_new(256);
        p = extract_string_arg(p, &arg1);
        if (p && arg1.len > 0) {
            strlist_push(out, arg1.data);
        }
        dynbuf_free(&arg1);
    }
    else {
        return NULL; // unknown op
    }

    // Skip to closing ')'
    if (p) {
        while (*p && *p != ')') p++;
        if (*p == ')') p++;
    }
    return p;
}

// Parse a section (forward or rollback) and extract SQL statements.
// Handles both db.op_* (translated to SQL) and db.execute() (legacy).
// Comment-aware: skips lines that start with # (after whitespace).
static int parse_section(const char* content, const char* section_name, StrList* out) {
    out->count = 0;

    const char* section = find_section(content, section_name);
    if (!section) return 0;

    const char* section_end = find_section_end(section);
    const char* p = section;

    while (p < section_end) {
        // Find the start of the current line
        const char* line_start = p;

        // Skip comment lines entirely
        if (is_comment_line(line_start, section_end)) {
            p = next_line(p);
            continue;
        }

        // Search for db.op_* or db.execute( on this line
        const char* line_end = p;
        while (line_end < section_end && *line_end != '\n') line_end++;

        const char* scan = line_start;
        while (scan < line_end) {
            // Try db.op_* first (new format)
            const char* op_found = strstr(scan, "db.op_");
            if (op_found && op_found < line_end) {
                const char* after = try_parse_op_call(op_found, section_end, out);
                if (after) {
                    scan = after;
                    // op calls can span multiple lines (triple-quoted args)
                    if (scan > line_end) {
                        // Update line_end to re-scan from current position
                        line_end = scan;
                        while (line_end < section_end && *line_end != '\n') line_end++;
                    }
                    continue;
                }
            }

            // Try db.execute( (legacy format)
            const char* exec_found = strstr(scan, "db.execute(");
            if (exec_found && exec_found < line_end) {
                const char* arg = exec_found + 11; // len("db.execute(")
                while (*arg == ' ' || *arg == '\t') arg++;

                DynBuf sql_buf = dynbuf_new(256);
                const char* after = extract_execute_sql(arg, &sql_buf);
                if (after && sql_buf.len > 0) {
                    strlist_push(out, sql_buf.data);
                    dynbuf_free(&sql_buf);
                    scan = after;
                    if (scan >= line_end) break;
                    continue;
                } else {
                    dynbuf_free(&sql_buf);
                    scan = exec_found + 1;
                    continue;
                }
            }

            // Nothing found on this line
            break;
        }

        // Move to next line (or past the scan position if multi-line op)
        if (scan > line_start) {
            p = next_line(scan > line_end ? scan : line_start);
        } else {
            p = next_line(line_start);
        }
    }

    return out->count;
}

// ============================================================
// File scanning — find migration files in directory
// ============================================================

// Compare for qsort
static int cmp_strings_256(const void* a, const void* b) {
    return strcmp(*(const char**)a, *(const char**)b);
}

// Scan directory for NNNN_*.desi files, sorted by name.
// Returns a StrList — caller must free with strlist_free().
static StrList scan_migrations(const char* dir) {
    StrList files = strlist_new(16);
    DIR* d = opendir(dir);
    if (!d) return files;

    struct dirent* entry;
    while ((entry = readdir(d)) != NULL) {
        const char* name = entry->d_name;
        // Must match pattern: 4+ digits followed by _...desi
        int digits = 0;
        while (name[digits] >= '0' && name[digits] <= '9') digits++;
        if (digits < 4) continue;
        if (name[digits] != '_') continue;
        // Check .desi extension
        int len = (int)strlen(name);
        if (len < 5 || strcmp(name + len - 5, ".desi") != 0) continue;

        strlist_push(&files, name);
    }
    closedir(d);

    // Sort by filename (numeric prefix ensures correct order)
    if (files.count > 1) {
        qsort(files.items, files.count, sizeof(char*), cmp_strings_256);
    }

    return files;
}

// Find the next migration number from an already-scanned file list
static int next_migration_number_from(const StrList* files) {
    if (files->count == 0) return 1;

    int max_num = 0;
    for (int i = 0; i < files->count; i++) {
        int num = atoi(files->items[i]);
        if (num > max_num) max_num = num;
    }
    return max_num + 1;
}

// ============================================================
// makemigrations — Generate .desi migration files
// ============================================================

int32_t __db_makemigrations(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    // Ensure directory exists
    mkdir(dir, 0755);

    // Scan existing migrations once (eliminates double-scan)
    StrList existing_files = scan_migrations(dir);
    int next_num = next_migration_number_from(&existing_files);

    OpList ops = oplist_new(16);
    int model_count = __orm_model_count();

    // ---- Diff models against live DB schema ----
    for (int m = 0; m < model_count; m++) {
        const char* tname = __orm_model_name(m);
        if (!tname || tname[0] == '\0') continue;

        if (!__db_table_exists(tname)) {
            // Table doesn't exist — need CREATE TABLE
            // Build field specs for the op-based format
            int fc = __orm_field_count(tname);
            DynBuf specs = dynbuf_new(256);
            for (int fi = 0; fi < fc; fi++) {
                char* spec = __orm_field_spec(tname, fi);
                if (spec && spec[0]) {
                    if (specs.len > 0) dynbuf_append_char(&specs, '\n');
                    dynbuf_append(&specs, spec);
                }
                if (spec) free(spec);
            }
            if (specs.len > 0) {
                MigOp* op = oplist_add(&ops);
                snprintf(op->action, sizeof(op->action), "%s", "create_table");
                snprintf(op->table, sizeof(op->table), "%s", tname);
                op->column[0] = '\0';
                op->field_specs = specs.data; specs.data = NULL;
                // forward_sql kept as fallback (legacy compat)
                char* create_sql = __orm_create_table_sql(tname);
                op->forward_sql = create_sql ? create_sql : strdup("");
                DynBuf rb = dynbuf_new(64);
                dynbuf_appendf(&rb, "DROP TABLE IF EXISTS %s", tname);
                op->rollback_sql = rb.data; rb.data = NULL;
            } else {
                dynbuf_free(&specs);
            }
        } else {
            // Table exists — diff columns
            int field_count = __orm_field_count(tname);
            for (int f = 0; f < field_count; f++) {
                const char* fname = __orm_field_name(tname, f);
                if (!fname || fname[0] == '\0') continue;

                if (!column_exists(tname, fname)) {
                    // Column missing — need ADD COLUMN
                    // Get field spec for this field
                    char* spec = __orm_field_spec(tname, f);
                    if (spec && spec[0]) {
                        MigOp* op = oplist_add(&ops);
                        snprintf(op->action, sizeof(op->action), "%s", "add_column");
                        snprintf(op->table, sizeof(op->table), "%s", tname);
                        snprintf(op->column, sizeof(op->column), "%s", fname);
                        op->field_specs = spec; // transfer ownership
                        // Keep forward_sql as legacy fallback
                        char* alter_sql = __orm_add_column_sql(tname, fname);
                        op->forward_sql = alter_sql ? alter_sql : strdup("");
                        DynBuf rb = dynbuf_new(128);
                        dynbuf_appendf(&rb, "ALTER TABLE %s DROP COLUMN %s", tname, fname);
                        op->rollback_sql = rb.data;
                        rb.data = NULL;
                    } else {
                        if (spec) free(spec);
                    }
                } else {
                    // Column exists — check type
                    char* old_type = get_column_type(tname, fname);
                    char* new_type = __orm_column_type_sql(tname, fname);
                    if (old_type && new_type && old_type[0] && new_type[0]) {
                        if (!types_match(old_type, new_type)) {
                            MigOp* op = oplist_add(&ops);
                            snprintf(op->action, sizeof(op->action), "%s", "alter_type");
                            snprintf(op->table, sizeof(op->table), "%s", tname);
                            snprintf(op->column, sizeof(op->column), "%s", fname);
                            op->new_type = strdup(new_type);
                            op->old_type = strdup(old_type);
                            // Keep forward/rollback SQL as legacy fallback
                            DynBuf fwd = dynbuf_new(256);
                            DynBuf rb = dynbuf_new(256);
                            if (is_pg()) {
                                dynbuf_appendf(&fwd,
                                    "ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s",
                                    tname, fname, new_type, fname, new_type);
                                dynbuf_appendf(&rb,
                                    "ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s",
                                    tname, fname, old_type, fname, old_type);
                            } else {
                                dynbuf_appendf(&fwd,
                                    "ALTER TABLE %s MODIFY COLUMN %s %s",
                                    tname, fname, new_type);
                                dynbuf_appendf(&rb,
                                    "ALTER TABLE %s MODIFY COLUMN %s %s",
                                    tname, fname, old_type);
                            }
                            op->forward_sql = fwd.data; fwd.data = NULL;
                            op->rollback_sql = rb.data; rb.data = NULL;
                        }
                    }
                    if (old_type) free(old_type);
                    if (new_type) free(new_type);
                }
            }

            // Check for orphaned columns (columns in DB but not in model)
            int db_cols = get_table_columns(tname);
            if (db_cols > 0) {
                int col_count = __db_row_count();
                // Collect column names into a temporary list
                StrList db_col_names = strlist_new(col_count > 0 ? col_count : 4);
                for (int c = 0; c < col_count; c++) {
                    char* cn = __db_get_value_at(c, 0);
                    if (cn && cn[0]) {
                        strlist_push(&db_col_names, cn);
                    }
                    if (cn) free(cn);
                }
                int field_count2 = __orm_field_count(tname);
                for (int c = 0; c < db_col_names.count; c++) {
                    int found = 0;
                    for (int f2 = 0; f2 < field_count2; f2++) {
                        const char* mf = __orm_field_name(tname, f2);
                        if (mf && strcmp(mf, db_col_names.items[c]) == 0) { found = 1; break; }
                    }
                    if (!found) {
                        char* old_type = get_column_type(tname, db_col_names.items[c]);
                        MigOp* op = oplist_add(&ops);
                        snprintf(op->action, sizeof(op->action), "%s", "drop_column");
                        snprintf(op->table, sizeof(op->table), "%s", tname);
                        snprintf(op->column, sizeof(op->column), "%s", db_col_names.items[c]);
                        DynBuf fwd = dynbuf_new(128);
                        dynbuf_appendf(&fwd, "ALTER TABLE %s DROP COLUMN %s",
                            tname, db_col_names.items[c]);
                        op->forward_sql = fwd.data; fwd.data = NULL;
                        DynBuf rb = dynbuf_new(128);
                        dynbuf_appendf(&rb, "ALTER TABLE %s ADD COLUMN %s %s",
                            tname, db_col_names.items[c],
                            old_type ? old_type : "TEXT");
                        op->rollback_sql = rb.data; rb.data = NULL;
                        if (old_type) free(old_type);
                    }
                }
                strlist_free(&db_col_names);
            }
        }
    }

    if (ops.count == 0) {
        printf("[makemigrations] No changes detected\n");
        fflush(stdout);
        oplist_free(&ops);
        strlist_free(&existing_files);
        return 0;
    }

    // ---- Generate description from operations ----
    char description[256] = "";
    if (next_num == 1) {
        snprintf(description, sizeof(description), "%s", "initial");
    } else if (ops.count == 1) {
        MigOp* op0 = ops.items[0];
        if (strcmp(op0->action, "create_table") == 0) {
            snprintf(description, sizeof(description), "create_%s", op0->table);
        } else if (strcmp(op0->action, "add_column") == 0) {
            snprintf(description, sizeof(description), "add_%s_to_%s", op0->column, op0->table);
        } else if (strcmp(op0->action, "drop_column") == 0) {
            snprintf(description, sizeof(description), "drop_%s_from_%s", op0->column, op0->table);
        } else if (strcmp(op0->action, "alter_type") == 0) {
            snprintf(description, sizeof(description), "alter_%s_in_%s", op0->column, op0->table);
        }
    } else {
        snprintf(description, sizeof(description), "auto_%d_operations", ops.count);
    }

    // ---- Build file path ----
    char filename[256];
    snprintf(filename, sizeof(filename), "%04d_%s.desi", next_num, description);

    char filepath[512];
    snprintf(filepath, sizeof(filepath), "%s/%s", dir, filename);

    // ---- Write the .desi migration file ----
    FILE* f = fopen(filepath, "w");
    if (!f) {
        printf("[makemigrations] Failed to create %s\n", filepath);
        fflush(stdout);
        oplist_free(&ops);
        strlist_free(&existing_files);
        return -1;
    }

    // Get current timestamp
    time_t now = time(NULL);
    struct tm* tm_info = localtime(&now);
    char timestamp[64];
    strftime(timestamp, sizeof(timestamp), "%Y-%m-%dT%H:%M:%S", tm_info);

    // Find previous migration for depends
    char depends[256] = "none";
    if (next_num > 1 && existing_files.count > 0) {
        // Remove .desi extension for depends
        snprintf(depends, sizeof(depends), "%s", existing_files.items[existing_files.count - 1]);
        char* dot = strstr(depends, ".desi");
        if (dot) *dot = '\0';
    }

    // Extract app label from directory
    char* app_label = extract_app_label(dir);

    // Write header
    fprintf(f, "# Migration: %04d_%s\n", next_num, description);
    fprintf(f, "# Generated: %s\n", timestamp);
    fprintf(f, "# App: %s\n", app_label);
    fprintf(f, "# Depends: %s\n", depends);
    fprintf(f, "#\n");
    fprintf(f, "# Operations:\n");
    for (int i = 0; i < ops.count; i++) {
        MigOp* op = ops.items[i];
        if (op->column[0]) {
            fprintf(f, "#   %s(%s.%s)\n", op->action, op->table, op->column);
        } else {
            fprintf(f, "#   %s(%s)\n", op->action, op->table);
        }
    }
    fprintf(f, "\n");
    fprintf(f, "import db\n");
    fprintf(f, "\n");

    // Write forward() — using db.op_* DSL for portability
    fprintf(f, "def forward():\n");
    for (int i = 0; i < ops.count; i++) {
        MigOp* op = ops.items[i];
        fprintf(f, "\t# %s", op->action);
        if (op->column[0]) fprintf(f, ": %s.%s", op->table, op->column);
        else fprintf(f, ": %s", op->table);
        fprintf(f, "\n");

        if (strcmp(op->action, "create_table") == 0 && op->field_specs) {
            fprintf(f, "\tdb.op_create_table(\"%s\", \"\"\"%s\"\"\")\n",
                op->table, op->field_specs);
        } else if (strcmp(op->action, "add_column") == 0 && op->field_specs) {
            fprintf(f, "\tdb.op_add_column(\"%s\", \"%s\")\n",
                op->table, op->field_specs);
        } else if (strcmp(op->action, "drop_column") == 0) {
            fprintf(f, "\tdb.op_drop_column(\"%s\", \"%s\")\n",
                op->table, op->column);
        } else if (strcmp(op->action, "alter_type") == 0 && op->new_type) {
            fprintf(f, "\tdb.op_alter_column(\"%s\", \"%s\", \"%s\")\n",
                op->table, op->column, op->new_type);
        } else {
            // Fallback to legacy db.execute()
            if (op->forward_sql && strchr(op->forward_sql, '\n')) {
                fprintf(f, "\tdb.execute(\"\"\"%s\"\"\")\n", op->forward_sql);
            } else {
                fprintf(f, "\tdb.execute(\"%s\")\n", op->forward_sql ? op->forward_sql : "");
            }
        }
        if (i < ops.count - 1) fprintf(f, "\n");
    }

    fprintf(f, "\n");

    // Write rollback() — operations in reverse order, using db.op_* DSL
    fprintf(f, "def rollback():\n");
    for (int i = ops.count - 1; i >= 0; i--) {
        MigOp* op = ops.items[i];
        fprintf(f, "\t# undo %s", op->action);
        if (op->column[0]) fprintf(f, ": %s.%s", op->table, op->column);
        else fprintf(f, ": %s", op->table);
        fprintf(f, "\n");

        if (strcmp(op->action, "create_table") == 0) {
            fprintf(f, "\tdb.op_drop_table(\"%s\")\n", op->table);
        } else if (strcmp(op->action, "add_column") == 0) {
            fprintf(f, "\tdb.op_drop_column(\"%s\", \"%s\")\n",
                op->table, op->column);
        } else if (strcmp(op->action, "drop_column") == 0) {
            // Rollback: re-add the column (use legacy SQL — we don't have spec)
            if (op->rollback_sql) {
                fprintf(f, "\tdb.execute(\"%s\")\n", op->rollback_sql);
            }
        } else if (strcmp(op->action, "alter_type") == 0 && op->old_type) {
            fprintf(f, "\tdb.op_alter_column(\"%s\", \"%s\", \"%s\")\n",
                op->table, op->column, op->old_type);
        } else {
            // Fallback to legacy db.execute()
            if (op->rollback_sql && strchr(op->rollback_sql, '\n')) {
                fprintf(f, "\tdb.execute(\"\"\"%s\"\"\")\n", op->rollback_sql);
            } else {
                fprintf(f, "\tdb.execute(\"%s\")\n", op->rollback_sql ? op->rollback_sql : "");
            }
        }
        if (i > 0) fprintf(f, "\n");
    }

    fprintf(f, "\n");
    fclose(f);

    int result_count = ops.count;
    printf("[makemigrations] Created %s (%d operation(s)) [app: %s]\n", filepath, result_count, app_label);
    fflush(stdout);

    free(app_label);
    oplist_free(&ops);
    strlist_free(&existing_files);
    return result_count;
}

// ============================================================
// migrate — Apply pending .desi migration files
//
// PostgreSQL: wraps each migration in BEGIN/COMMIT for
// transactional DDL — if any statement fails, the entire
// migration is rolled back atomically.
//
// MySQL: DDL causes implicit commits, so transactions don't
// help for schema changes. We use best-effort execution with
// clear error messages. The migration is only recorded if all
// statements succeed.
// ============================================================

int32_t __db_migrate_files(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    // Extract app label from the directory path
    char* app_label = extract_app_label(dir);

    StrList files = scan_migrations(dir);
    if (files.count == 0) {
        printf("[migrate] No migration files found in %s/\n", dir);
        fflush(stdout);
        strlist_free(&files);
        free(app_label);
        return 0;
    }

    int applied = 0;
    int use_tx = is_pg();  // Only PG supports transactional DDL

    for (int i = 0; i < files.count; i++) {
        // Check if already applied (scoped by app_label)
        if (migration_applied_for_app(app_label, files.items[i])) continue;

        // Build full path
        char path[512];
        snprintf(path, sizeof(path), "%s/%s", dir, files.items[i]);

        // Read and parse the file
        char* content = read_file(path);
        if (!content) {
            printf("[migrate] Failed to read %s\n", path);
            fflush(stdout);
            continue;
        }

        StrList forward_stmts = strlist_new(8);
        StrList rollback_stmts = strlist_new(8);
        parse_section(content, "forward", &forward_stmts);
        parse_section(content, "rollback", &rollback_stmts);

        if (forward_stmts.count == 0) {
            printf("[migrate] No forward SQL in %s — skipping\n", files.items[i]);
            fflush(stdout);
            strlist_free(&forward_stmts);
            strlist_free(&rollback_stmts);
            free(content);
            continue;
        }

        // Begin transaction for PG (supports transactional DDL)
        if (use_tx) __db_begin();

        // Execute all forward statements
        int all_ok = 1;
        for (int s = 0; s < forward_stmts.count; s++) {
            int result = __db_execute_stmt(forward_stmts.items[s]);
            if (result < 0) {
                printf("[migrate] %s: statement %d/%d FAILED\n",
                    files.items[i], s + 1, forward_stmts.count);
                fflush(stdout);
                all_ok = 0;
                if (use_tx) {
                    __db_rollback_tx();
                    printf("[migrate] %s: transaction rolled back (no changes applied)\n",
                        files.items[i]);
                    fflush(stdout);
                } else {
                    printf("[migrate] %s: MySQL — statements 1-%d may have been applied (DDL auto-commits)\n",
                        files.items[i], s);
                    fflush(stdout);
                }
                break;
            }
        }

        if (all_ok) {
            // Commit transaction for PG
            if (use_tx) __db_commit();

            // Build SQL strings for recording
            DynBuf all_forward = dynbuf_new(256);
            DynBuf all_rollback = dynbuf_new(256);

            for (int s = 0; s < forward_stmts.count; s++) {
                if (s > 0) dynbuf_append(&all_forward, "\n---\n");
                dynbuf_append(&all_forward, forward_stmts.items[s]);
            }
            for (int s = 0; s < rollback_stmts.count; s++) {
                if (s > 0) dynbuf_append(&all_rollback, "\n---\n");
                dynbuf_append(&all_rollback, rollback_stmts.items[s]);
            }

            record_migration_with_app(app_label, files.items[i], all_forward.data, all_rollback.data);
            applied++;
            printf("[migrate] Applied %s [%s] ✓\n", files.items[i], app_label);
            fflush(stdout);

            dynbuf_free(&all_forward);
            dynbuf_free(&all_rollback);
        }

        strlist_free(&forward_stmts);
        strlist_free(&rollback_stmts);
        free(content);
    }

    if (applied == 0) {
        printf("[migrate] No pending migrations\n");
    } else {
        printf("[migrate] Applied %d migration(s) for app '%s'\n", applied, app_label);
    }
    fflush(stdout);

    free(app_label);
    strlist_free(&files);
    return applied;
}

// ============================================================
// rollback — Undo the last applied migration file
//
// Same transactional semantics as migrate: PG uses BEGIN/COMMIT,
// MySQL uses best-effort.
// ============================================================

int32_t __db_rollback_file(const char* dir) {
    if (!__db_is_connected() || !dir) return -1;

    ensure_tracking();

    // Extract app label from the directory path
    char* app_label = extract_app_label(dir);
    char* app_esc = sql_escape_alloc(app_label);

    // Find the last applied migration for this app
    DynBuf query = dynbuf_new(256);
    dynbuf_appendf(&query,
        "SELECT filename FROM _desi_migrations WHERE app_label = '%s' ORDER BY id DESC LIMIT 1",
        app_esc);
    int rows = __db_query_exec(query.data);
    dynbuf_free(&query);
    free(app_esc);

    if (rows <= 0) {
        printf("[rollback] No migrations to roll back for app '%s'\n", app_label);
        fflush(stdout);
        free(app_label);
        return 0;
    }

    char* filename = __db_get_value_at(0, 0);
    if (!filename || filename[0] == '\0') {
        printf("[rollback] No migration filename found\n");
        fflush(stdout);
        if (filename) free(filename);
        free(app_label);
        return 0;
    }

    // Read the migration file
    char path[512];
    snprintf(path, sizeof(path), "%s/%s", dir, filename);

    char* content = read_file(path);
    if (!content) {
        printf("[rollback] Failed to read %s\n", path);
        fflush(stdout);
        free(filename);
        free(app_label);
        return -1;
    }

    // Parse rollback section
    StrList rollback_stmts = strlist_new(8);
    parse_section(content, "rollback", &rollback_stmts);

    if (rollback_stmts.count == 0) {
        printf("[rollback] No rollback SQL in %s\n", filename);
        fflush(stdout);
        strlist_free(&rollback_stmts);
        free(content);
        free(filename);
        free(app_label);
        return 0;
    }

    int use_tx = is_pg();
    if (use_tx) __db_begin();

    // Execute rollback statements
    int rolled_back = 0;
    int all_ok = 1;
    for (int s = 0; s < rollback_stmts.count; s++) {
        int result = __db_execute_stmt(rollback_stmts.items[s]);
        if (result >= 0) {
            rolled_back++;
        } else {
            printf("[rollback] Statement %d/%d FAILED in %s\n",
                s + 1, rollback_stmts.count, filename);
            fflush(stdout);
            all_ok = 0;
            if (use_tx) {
                __db_rollback_tx();
                printf("[rollback] %s: transaction rolled back\n", filename);
                fflush(stdout);
                rolled_back = 0;
            }
            break;
        }
    }

    if (all_ok) {
        if (use_tx) __db_commit();

        // Remove the migration record (escaped, scoped by app_label)
        char* fn_esc = sql_escape_alloc(filename);
        char* app_esc2 = sql_escape_alloc(app_label);
        DynBuf del = dynbuf_new(256);
        dynbuf_appendf(&del,
            "DELETE FROM _desi_migrations WHERE app_label = '%s' AND filename = '%s'",
            app_esc2, fn_esc);
        __db_execute_stmt(del.data);
        dynbuf_free(&del);
        free(fn_esc);
        free(app_esc2);

        printf("[rollback] Rolled back %s [%s] (%d operation(s)) ✓\n", filename, app_label, rolled_back);
        fflush(stdout);
    }

    strlist_free(&rollback_stmts);
    free(content);
    free(filename);
    free(app_label);
    return rolled_back;
}

// ============================================================
// migration_status — Show applied vs pending migrations
// ============================================================

char* __db_migration_status_files(const char* dir) {
    if (!__db_is_connected() || !dir) return strdup("Not connected");

    ensure_tracking();

    // Extract app label from directory path
    char* app_label = extract_app_label(dir);

    DynBuf buf = dynbuf_new(512);
    dynbuf_appendf(&buf, "Migration Status [%s]\n", app_label);
    dynbuf_append(&buf, "─────────────────────────────────────────\n");

    StrList files = scan_migrations(dir);

    if (files.count == 0) {
        dynbuf_appendf(&buf, "  No migration files found in %s/\n", dir);
        strlist_free(&files);
        free(app_label);
        char* result = buf.data;
        buf.data = NULL;
        return result;
    }

    for (int i = 0; i < files.count; i++) {
        int applied = migration_applied_for_app(app_label, files.items[i]);
        dynbuf_appendf(&buf, "  %s  %s\n",
            applied ? "✓" : "○",
            files.items[i]);
    }

    strlist_free(&files);
    free(app_label);

    // Transfer ownership — caller frees
    char* result = buf.data;
    buf.data = NULL;
    return result;
}

// ============================================================
// Header Parser — Extract metadata from migration file headers
//
// Migration files have comment headers like:
//   # App: accounts
//   # Depends: accounts.0001_initial
//
// parse_header_field() extracts the value for a given key.
// ============================================================

// Parse a header field from migration file content.
// key: the field name (e.g. "App", "Depends")
// Returns heap-allocated value string — caller must free.
// Returns NULL if field not found.
static char* parse_header_field(const char* content, const char* key) {
    if (!content || !key) return NULL;

    // Build search pattern: "# Key: "
    char pattern[128];
    snprintf(pattern, sizeof(pattern), "# %s: ", key);
    int plen = (int)strlen(pattern);

    const char* p = content;
    while (*p) {
        // Only search in comment lines (lines starting with #)
        // Skip leading whitespace
        while (*p == ' ' || *p == '\t') p++;
        if (*p != '#') {
            // Not a comment line — headers are at the top, stop searching
            // once we hit a non-comment, non-blank line
            if (*p != '\n' && *p != '\r' && *p != '\0') break;
            if (*p) p++;
            continue;
        }

        // Match \"# Key: \" from current position
        if (strncmp(p, pattern, plen) == 0) {
            // Extract value until end of line
            const char* val_start = p + plen;
            const char* val_end = val_start;
            while (*val_end && *val_end != '\n' && *val_end != '\r') val_end++;

            // Trim trailing whitespace
            while (val_end > val_start && (*(val_end-1) == ' ' || *(val_end-1) == '\t'))
                val_end--;

            int vlen = (int)(val_end - val_start);
            char* value = (char*)malloc(vlen + 1);
            memcpy(value, val_start, vlen);
            value[vlen] = '\0';
            return value;
        }

        // Skip to next line
        while (*p && *p != '\n') p++;
        if (*p == '\n') p++;
    }

    return NULL;
}

// ============================================================
// migrate_all — Apply migrations across multiple app directories
//
// Accepts a newline-separated string of migration directories.
// Reads dependency headers from migration files and applies
// them in topological order (like Django's migration executor).
//
// Project structure:
//   accounts/migrations/   → app_label "accounts"
//   orders/migrations/     → app_label "orders" (may depend on accounts)
// ============================================================

// Internal: represents a migration node in the dependency graph
typedef struct {
    char* app_label;     // e.g. "accounts"
    char* filename;      // e.g. "0001_initial.desi"
    char* dir;           // e.g. "accounts/migrations"
    char* depends;       // e.g. "accounts.0001_initial" or "none"
    int   applied;       // already applied?
    int   visited;       // for topological sort
    int   in_progress;   // for cycle detection
} MigNode;

typedef struct {
    MigNode** items;
    int       count;
    int       cap;
} MigNodeList;

static MigNodeList mignodelist_new(int cap) {
    MigNodeList ml;
    ml.cap = cap > 4 ? cap : 4;
    ml.items = (MigNode**)malloc(ml.cap * sizeof(MigNode*));
    ml.count = 0;
    return ml;
}

static MigNode* mignodelist_add(MigNodeList* ml) {
    if (ml->count >= ml->cap) {
        ml->cap *= 2;
        ml->items = (MigNode**)realloc(ml->items, ml->cap * sizeof(MigNode*));
    }
    MigNode* n = (MigNode*)calloc(1, sizeof(MigNode));
    ml->items[ml->count++] = n;
    return n;
}

static void mignodelist_free(MigNodeList* ml) {
    for (int i = 0; i < ml->count; i++) {
        if (ml->items[i]->app_label) free(ml->items[i]->app_label);
        if (ml->items[i]->filename) free(ml->items[i]->filename);
        if (ml->items[i]->dir) free(ml->items[i]->dir);
        if (ml->items[i]->depends) free(ml->items[i]->depends);
        free(ml->items[i]);
    }
    free(ml->items);
    ml->items = NULL;
    ml->count = ml->cap = 0;
}

// Find a node by app_label.migration_name
static MigNode* find_node(MigNodeList* ml, const char* app_label, const char* migration) {
    for (int i = 0; i < ml->count; i++) {
        if (strcmp(ml->items[i]->app_label, app_label) == 0) {
            // Compare migration name (without .desi extension)
            char basename[256];
            snprintf(basename, sizeof(basename), "%s", ml->items[i]->filename);
            char* dot = strstr(basename, ".desi");
            if (dot) *dot = '\0';
            if (strcmp(basename, migration) == 0) return ml->items[i];
        }
    }
    return NULL;
}

// Topological sort helper — DFS with cycle detection
// Appends to 'order' list in dependency-first order
static int topo_visit(MigNode* node, MigNodeList* all_nodes,
                       MigNode** order, int* order_count) {
    if (node->visited) return 0;  // already processed
    if (node->in_progress) {
        printf("[migrate_all] ERROR: circular dependency involving %s.%s\n",
            node->app_label, node->filename);
        fflush(stdout);
        return -1;  // cycle detected
    }

    node->in_progress = 1;

    // Process dependency first
    if (node->depends && strcmp(node->depends, "none") != 0) {
        // Parse depends: "app.migration_name"
        char dep_copy[256];
        snprintf(dep_copy, sizeof(dep_copy), "%s", node->depends);
        char* dot = strchr(dep_copy, '.');
        if (dot) {
            *dot = '\0';
            const char* dep_app = dep_copy;
            const char* dep_mig = dot + 1;
            MigNode* dep_node = find_node(all_nodes, dep_app, dep_mig);
            if (dep_node) {
                int r = topo_visit(dep_node, all_nodes, order, order_count);
                if (r < 0) return r;
            }
            // If dep_node not found, it might already be applied or from
            // outside our set — that's OK, we'll just proceed
        } else {
            // Depends is just a filename within the same app
            MigNode* dep_node = find_node(all_nodes, node->app_label, node->depends);
            if (dep_node) {
                int r = topo_visit(dep_node, all_nodes, order, order_count);
                if (r < 0) return r;
            }
        }
    }

    node->in_progress = 0;
    node->visited = 1;
    order[(*order_count)++] = node;
    return 0;
}

int32_t __db_migrate_all(const char* dirs_newline_separated) {
    if (!__db_is_connected() || !dirs_newline_separated) return -1;

    ensure_tracking();

    // Split input by newlines into list of directories
    StrList dirs = strlist_new(8);
    const char* p = dirs_newline_separated;
    while (*p) {
        // Skip whitespace/newlines
        while (*p == '\n' || *p == '\r' || *p == ' ' || *p == '\t') p++;
        if (!*p) break;

        const char* start = p;
        while (*p && *p != '\n' && *p != '\r') p++;

        // Trim trailing whitespace
        const char* end = p;
        while (end > start && (*(end-1) == ' ' || *(end-1) == '\t')) end--;

        if (end > start) {
            char dir_buf[512];
            int dlen = (int)(end - start);
            if (dlen >= (int)sizeof(dir_buf)) dlen = (int)sizeof(dir_buf) - 1;
            memcpy(dir_buf, start, dlen);
            dir_buf[dlen] = '\0';
            strlist_push(&dirs, dir_buf);
        }
    }

    if (dirs.count == 0) {
        printf("[migrate_all] No migration directories provided\n");
        fflush(stdout);
        strlist_free(&dirs);
        return 0;
    }

    printf("[migrate_all] Scanning %d app(s) for migrations...\n", dirs.count);
    fflush(stdout);

    // Build the global node list from all directories
    MigNodeList nodes = mignodelist_new(32);

    for (int d = 0; d < dirs.count; d++) {
        char* app_label = extract_app_label(dirs.items[d]);
        StrList files = scan_migrations(dirs.items[d]);

        for (int f = 0; f < files.count; f++) {
            // Check if already applied
            if (migration_applied_for_app(app_label, files.items[f])) {
                continue;  // skip applied migrations
            }

            // Read file to get dependency info
            char path[512];
            snprintf(path, sizeof(path), "%s/%s", dirs.items[d], files.items[f]);
            char* content = read_file(path);

            MigNode* node = mignodelist_add(&nodes);
            node->app_label = strdup(app_label);
            node->filename = strdup(files.items[f]);
            node->dir = strdup(dirs.items[d]);
            node->applied = 0;
            node->visited = 0;
            node->in_progress = 0;

            if (content) {
                // Try to read App header (might override directory-derived app_label)
                char* file_app = parse_header_field(content, "App");
                if (file_app && file_app[0]) {
                    free(node->app_label);
                    node->app_label = file_app;
                } else {
                    if (file_app) free(file_app);
                }

                // Read depends header
                char* dep = parse_header_field(content, "Depends");
                node->depends = dep ? dep : strdup("none");

                free(content);
            } else {
                node->depends = strdup("none");
            }
        }

        strlist_free(&files);
        free(app_label);
    }

    if (nodes.count == 0) {
        printf("[migrate_all] No pending migrations across all apps\n");
        fflush(stdout);
        mignodelist_free(&nodes);
        strlist_free(&dirs);
        return 0;
    }

    // Topological sort
    MigNode** order = (MigNode**)malloc(nodes.count * sizeof(MigNode*));
    int order_count = 0;

    for (int i = 0; i < nodes.count; i++) {
        if (!nodes.items[i]->visited) {
            int r = topo_visit(nodes.items[i], &nodes, order, &order_count);
            if (r < 0) {
                printf("[migrate_all] Aborting due to circular dependency\n");
                fflush(stdout);
                free(order);
                mignodelist_free(&nodes);
                strlist_free(&dirs);
                return -1;
            }
        }
    }

    // Apply migrations in topological order
    int total_applied = 0;
    int use_tx = is_pg();

    for (int i = 0; i < order_count; i++) {
        MigNode* node = order[i];

        // Double-check not applied (could have been applied by a previous
        // iteration if the same migration appears in multiple paths)
        if (migration_applied_for_app(node->app_label, node->filename)) continue;

        // Build full path
        char path[512];
        snprintf(path, sizeof(path), "%s/%s", node->dir, node->filename);

        char* content = read_file(path);
        if (!content) {
            printf("[migrate_all] Failed to read %s\n", path);
            fflush(stdout);
            continue;
        }

        StrList forward_stmts = strlist_new(8);
        StrList rollback_stmts = strlist_new(8);
        parse_section(content, "forward", &forward_stmts);
        parse_section(content, "rollback", &rollback_stmts);

        if (forward_stmts.count == 0) {
            printf("[migrate_all] No forward SQL in %s — skipping\n", node->filename);
            fflush(stdout);
            strlist_free(&forward_stmts);
            strlist_free(&rollback_stmts);
            free(content);
            continue;
        }

        // Begin transaction for PG
        if (use_tx) __db_begin();

        int all_ok = 1;
        for (int s = 0; s < forward_stmts.count; s++) {
            int result = __db_execute_stmt(forward_stmts.items[s]);
            if (result < 0) {
                printf("[migrate_all] %s.%s: statement %d/%d FAILED\n",
                    node->app_label, node->filename, s + 1, forward_stmts.count);
                fflush(stdout);
                all_ok = 0;
                if (use_tx) {
                    __db_rollback_tx();
                    printf("[migrate_all] %s.%s: transaction rolled back\n",
                        node->app_label, node->filename);
                    fflush(stdout);
                }
                break;
            }
        }

        if (all_ok) {
            if (use_tx) __db_commit();

            DynBuf all_forward = dynbuf_new(256);
            DynBuf all_rollback = dynbuf_new(256);
            for (int s = 0; s < forward_stmts.count; s++) {
                if (s > 0) dynbuf_append(&all_forward, "\n---\n");
                dynbuf_append(&all_forward, forward_stmts.items[s]);
            }
            for (int s = 0; s < rollback_stmts.count; s++) {
                if (s > 0) dynbuf_append(&all_rollback, "\n---\n");
                dynbuf_append(&all_rollback, rollback_stmts.items[s]);
            }

            record_migration_with_app(node->app_label, node->filename,
                all_forward.data, all_rollback.data);
            total_applied++;
            printf("[migrate_all] Applied %s.%s ✓\n", node->app_label, node->filename);
            fflush(stdout);

            dynbuf_free(&all_forward);
            dynbuf_free(&all_rollback);
        }

        strlist_free(&forward_stmts);
        strlist_free(&rollback_stmts);
        free(content);
    }

    if (total_applied == 0) {
        printf("[migrate_all] No pending migrations across all apps\n");
    } else {
        printf("[migrate_all] Applied %d migration(s) across %d app(s)\n",
            total_applied, dirs.count);
    }
    fflush(stdout);

    free(order);
    mignodelist_free(&nodes);
    strlist_free(&dirs);
    return total_applied;
}

// ============================================================
// Migration Squashing — collapse N migration files into 1
//
// Django equivalent: python manage.py squashmigrations app 0001 0010
//
// Reads all .desi migration files in `dir`, combines their forward
// sections into a single consolidated file, and writes it to
// `dir/output_name.desi`. Original files are NOT deleted — they
// can be manually removed after squash is verified.
// ============================================================

int32_t __db_squash_migrations(const char* dir, const char* output_name) {
    if (!dir || !output_name) return -1;

    StrList files = scan_migrations(dir);
    if (files.count == 0) {
        printf("[squash] No migration files found in %s\n", dir);
        fflush(stdout);
        strlist_free(&files);
        return 0;
    }

    if (files.count <= 1) {
        printf("[squash] Only %d file(s) — nothing to squash\n", files.count);
        fflush(stdout);
        strlist_free(&files);
        return 0;
    }

    DynBuf forward_all = dynbuf_new(2048);
    DynBuf rollback_all = dynbuf_new(2048);
    int squashed_count = 0;

    for (int i = 0; i < files.count; i++) {
        char path[512];
        snprintf(path, sizeof(path), "%s/%s", dir, files.items[i]);
        char* content = read_file(path);
        if (!content) continue;

        StrList fwd = strlist_new(4);
        StrList rb = strlist_new(4);
        parse_section(content, "forward", &fwd);
        parse_section(content, "rollback", &rb);

        // Append forward SQL
        for (int s = 0; s < fwd.count; s++) {
            if (forward_all.len > 0) dynbuf_append(&forward_all, "\n");
            dynbuf_appendf(&forward_all, "    db.execute(\"\"\"%s\"\"\")", fwd.items[s]);
        }

        // Prepend rollback SQL (reverse order for proper undo)
        for (int s = rb.count - 1; s >= 0; s--) {
            if (rollback_all.len > 0) {
                DynBuf tmp = dynbuf_new(rollback_all.len + 256);
                dynbuf_appendf(&tmp, "    db.execute(\"\"\"%s\"\"\")\n%s",
                    rb.items[s], rollback_all.data);
                dynbuf_free(&rollback_all);
                rollback_all = tmp;
            } else {
                dynbuf_appendf(&rollback_all, "    db.execute(\"\"\"%s\"\"\")", rb.items[s]);
            }
        }

        squashed_count++;
        strlist_free(&fwd);
        strlist_free(&rb);
        free(content);
    }

    // Write squashed file
    char out_path[512];
    snprintf(out_path, sizeof(out_path), "%s/%s.desi", dir, output_name);

    FILE* fp = fopen(out_path, "w");
    if (!fp) {
        printf("[squash] Failed to create %s\n", out_path);
        fflush(stdout);
        dynbuf_free(&forward_all);
        dynbuf_free(&rollback_all);
        strlist_free(&files);
        return -1;
    }

    // Write header
    fprintf(fp, "# Squashed migration: %d files combined\n", squashed_count);
    fprintf(fp, "# Replaces: ");
    for (int i = 0; i < files.count; i++) {
        if (i > 0) fprintf(fp, ", ");
        fprintf(fp, "%s", files.items[i]);
    }
    fprintf(fp, "\n#\n# Depends: none\nimport db\n\n");

    // Write forward
    fprintf(fp, "def forward():\n");
    if (forward_all.len > 0) {
        fprintf(fp, "%s\n", forward_all.data);
    } else {
        fprintf(fp, "    pass\n");
    }

    // Write rollback
    fprintf(fp, "\ndef rollback():\n");
    if (rollback_all.len > 0) {
        fprintf(fp, "%s\n", rollback_all.data);
    } else {
        fprintf(fp, "    pass\n");
    }

    fclose(fp);
    printf("[squash] Squashed %d migrations → %s\n", squashed_count, out_path);
    printf("[squash] Original files preserved. Delete them manually after verifying.\n");
    fflush(stdout);

    dynbuf_free(&forward_all);
    dynbuf_free(&rollback_all);
    strlist_free(&files);
    return squashed_count;
}

// ============================================================
// Data Migrations — register and invoke named code callbacks
//
// Django equivalent:
//   def forwards(apps, schema_editor):
//       User = apps.get_model('auth', 'User')
//       for user in User.objects.all():
//           user.name = user.name.upper()
//           user.save()
//
// Desi:
//   db.register_data_migration("uppercase_names", uppercase_fn)
//   # In migration file: db.op_run_code("uppercase_names")
// ============================================================

#define MAX_DATA_MIGRATIONS 64
typedef struct {
    char label[128];
    void (*fn)(void);
} DataMigrationDef;

static DataMigrationDef g_data_migrations[MAX_DATA_MIGRATIONS];
static int g_data_migration_count = 0;

int32_t __db_register_data_migration(const char* label, void (*fn)(void)) {
    if (!label || !fn || g_data_migration_count >= MAX_DATA_MIGRATIONS) return -1;
    DataMigrationDef* dm = &g_data_migrations[g_data_migration_count++];
    strncpy(dm->label, label, sizeof(dm->label) - 1);
    dm->fn = fn;
    return 0;
}

int32_t __db_op_run_code(const char* label) {
    if (!label) return -1;
    for (int i = 0; i < g_data_migration_count; i++) {
        if (strcmp(g_data_migrations[i].label, label) == 0) {
            printf("[migrate] Running data migration: %s\n", label);
            fflush(stdout);
            g_data_migrations[i].fn();
            printf("[migrate] Data migration '%s' complete\n", label);
            fflush(stdout);
            return 0;
        }
    }
    printf("[migrate] WARNING: data migration '%s' not registered\n", label);
    fflush(stdout);
    return -1;
}

// ============================================================
// Schema Diff / Dry Run — show what migrations WOULD generate
//
// Django equivalent: python manage.py makemigrations --dry-run
//
// Compares current ORM model definitions against the database
// schema and returns a formatted string describing the changes.
// Does NOT write any files or modify the database.
// ============================================================

char* __db_makemigrations_dryrun(const char* dir) {
    if (!__db_is_connected()) return strdup("[dry-run] Not connected to database");

    ensure_tracking();

    DynBuf output = dynbuf_new(2048);
    dynbuf_append(&output, "=== Migration Dry Run ===\n\n");

    int changes = 0;
    int model_count = __orm_model_count();

    for (int m = 0; m < model_count; m++) {
        const char* table = __orm_model_name(m);
        if (!table || table[0] == '\0') continue;

        // Skip abstract models
        extern int32_t __orm_is_abstract(const char* table_name);
        if (__orm_is_abstract(table)) continue;

        if (!__db_table_exists(table)) {
            // New table
            char* create_sql = __orm_create_table_sql(table);
            dynbuf_appendf(&output, "CREATE TABLE %s\n", table);
            int fc = __orm_field_count(table);
            for (int f = 0; f < fc; f++) {
                const char* fname = __orm_field_name(table, f);
                char* fspec = __orm_field_spec(table, f);
                dynbuf_appendf(&output, "  + %s: %s\n", fname, fspec ? fspec : "?");
                if (fspec) free(fspec);
            }
            dynbuf_append(&output, "\n");
            free(create_sql);
            changes++;
        } else {
            // Existing table — check for new/altered columns
            int fc = __orm_field_count(table);
            for (int f = 0; f < fc; f++) {
                const char* fname = __orm_field_name(table, f);
                if (!column_exists(table, fname)) {
                    char* fspec = __orm_field_spec(table, f);
                    dynbuf_appendf(&output, "ALTER TABLE %s\n  + ADD COLUMN %s: %s\n\n",
                        table, fname, fspec ? fspec : "?");
                    if (fspec) free(fspec);
                    changes++;
                } else {
                    // Check for type changes
                    char* db_type = get_column_type(table, fname);
                    char* col_sql = __orm_column_type_sql(table, fname);
                    if (db_type && col_sql && !types_match(db_type, col_sql)) {
                        dynbuf_appendf(&output, "ALTER TABLE %s\n  ~ CHANGE %s: %s → %s\n\n",
                            table, fname, db_type, col_sql);
                        changes++;
                    }
                    if (db_type) free(db_type);
                    if (col_sql) free(col_sql);
                }
            }
        }
    }

    if (changes == 0) {
        dynbuf_append(&output, "No changes detected — models match database schema.\n");
    } else {
        dynbuf_appendf(&output, "--- %d change(s) detected ---\n", changes);
    }

    char* result = output.data;
    output.data = NULL;
    return result;
}

// ============================================================
// inspectdb — Reverse-engineer models from existing database
//
// Django equivalent: python manage.py inspectdb
//
// Queries information_schema for all user tables and generates
// Desi model definition code (db.model / db.*_field calls).
// ============================================================

char* __db_inspectdb(void) {
    if (!__db_is_connected()) return strdup("# Not connected to database\n");

    DynBuf output = dynbuf_new(4096);
    dynbuf_append(&output, "# Auto-generated by db.inspectdb()\n");
    dynbuf_append(&output, "# Review and adjust types before using in production\n\n");
    dynbuf_append(&output, "import db\n\n");

    // Query all user tables
    DynBuf sql = dynbuf_new(256);
    if (is_pg()) {
        dynbuf_append(&sql,
            "SELECT table_name FROM information_schema.tables "
            "WHERE table_schema = 'public' AND table_type = 'BASE TABLE' "
            "AND table_name NOT LIKE '_desi_%' "
            "ORDER BY table_name");
    } else {
        dynbuf_append(&sql,
            "SELECT table_name FROM information_schema.tables "
            "WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' "
            "AND table_name NOT LIKE '_desi_%' "
            "ORDER BY table_name");
    }

    int table_count = __db_query_exec(sql.data);
    dynbuf_free(&sql);

    if (table_count <= 0) {
        dynbuf_append(&output, "# No tables found in database\n");
        char* result = output.data; output.data = NULL;
        return result;
    }

    // Collect table names first (query results will be overwritten)
    StrList tables = strlist_new(table_count);
    for (int t = 0; t < table_count; t++) {
        char* tname = __db_get_value_at(t, 0);
        if (tname) strlist_push(&tables, tname);
    }

    // For each table, generate model code
    for (int t = 0; t < tables.count; t++) {
        const char* tname = tables.items[t];

        dynbuf_appendf(&output, "# --- %s ---\n", tname);
        dynbuf_appendf(&output, "db.model(\"%s\")\n", tname);

        // Query columns
        DynBuf col_sql = dynbuf_new(512);
        if (is_pg()) {
            dynbuf_appendf(&col_sql,
                "SELECT column_name, data_type, is_nullable, column_default, "
                "character_maximum_length "
                "FROM information_schema.columns "
                "WHERE table_schema = 'public' AND table_name = '%s' "
                "ORDER BY ordinal_position", tname);
        } else {
            dynbuf_appendf(&col_sql,
                "SELECT column_name, column_type, is_nullable, column_default, "
                "character_maximum_length "
                "FROM information_schema.columns "
                "WHERE table_schema = DATABASE() AND table_name = '%s' "
                "ORDER BY ordinal_position", tname);
        }

        int col_count = __db_query_exec(col_sql.data);
        dynbuf_free(&col_sql);

        // Collect column data before it gets overwritten
        typedef struct { char name[128]; char type[128]; int nullable; int max_len; } ColInfo;
        ColInfo* cols = NULL;
        if (col_count > 0) {
            cols = (ColInfo*)calloc(col_count, sizeof(ColInfo));
            for (int c = 0; c < col_count; c++) {
                char* cn = __db_get_value_at(c, 0);
                char* ct = __db_get_value_at(c, 1);
                char* nullable = __db_get_value_at(c, 2);
                char* max_len_s = __db_get_value_at(c, 4);
                if (cn) strncpy(cols[c].name, cn, 127);
                if (ct) strncpy(cols[c].type, ct, 127);
                cols[c].nullable = (nullable && strcasecmp_local(nullable, "YES") == 0) ? 1 : 0;
                cols[c].max_len = (max_len_s && max_len_s[0]) ? atoi(max_len_s) : 0;
            }
        }

        for (int c = 0; c < col_count; c++) {
            const char* cn = cols[c].name;
            const char* ct = cols[c].type;
            int nullable = cols[c].nullable;
            int max_len = cols[c].max_len;

            // Map SQL types to Desi field calls
            if (strcmp(cn, "id") == 0 &&
                (strcasecmp_local(ct, "integer") == 0 || strcasecmp_local(ct, "int") == 0 ||
                 strncasecmp(ct, "serial", 6) == 0 || strncasecmp(ct, "bigserial", 9) == 0 ||
                 strstr(ct, "AUTO_INCREMENT") != NULL || strstr(ct, "auto_increment") != NULL)) {
                dynbuf_appendf(&output, "db.auto_field(\"id\")\n");
            } else if (strcasecmp_local(ct, "text") == 0) {
                dynbuf_appendf(&output, "db.text_field(\"%s\", %d, %d)\n",
                    cn, nullable, 0);
            } else if (strncasecmp(ct, "varchar", 7) == 0 ||
                       strcasecmp_local(ct, "character varying") == 0) {
                int ml = max_len > 0 ? max_len : 255;
                dynbuf_appendf(&output, "db.char_field(\"%s\", %d, %d, %d)\n",
                    cn, ml, nullable, 0);
            } else if (strcasecmp_local(ct, "integer") == 0 ||
                       strcasecmp_local(ct, "int") == 0 ||
                       strcasecmp_local(ct, "bigint") == 0) {
                dynbuf_appendf(&output, "db.integer_field(\"%s\", %d, %d)\n",
                    cn, nullable, 0);
            } else if (strcasecmp_local(ct, "boolean") == 0 ||
                       strcasecmp_local(ct, "tinyint(1)") == 0) {
                dynbuf_appendf(&output, "db.boolean_field(\"%s\", %d, 0)\n",
                    cn, nullable);
            } else if (strcasecmp_local(ct, "real") == 0 ||
                       strcasecmp_local(ct, "float") == 0 ||
                       strcasecmp_local(ct, "double precision") == 0 ||
                       strcasecmp_local(ct, "double") == 0) {
                dynbuf_appendf(&output, "db.float_field(\"%s\", %d, %d)\n",
                    cn, nullable, 0);
            } else if (strcasecmp_local(ct, "timestamp with time zone") == 0 ||
                       strcasecmp_local(ct, "timestamptz") == 0 ||
                       strcasecmp_local(ct, "datetime") == 0 ||
                       strcasecmp_local(ct, "timestamp") == 0) {
                dynbuf_appendf(&output, "db.datetime_field(\"%s\", %d, 0, 0)\n",
                    cn, nullable);
            } else if (strcasecmp_local(ct, "date") == 0) {
                dynbuf_appendf(&output, "db.date_field(\"%s\", %d)\n",
                    cn, nullable);
            } else if (strcasecmp_local(ct, "jsonb") == 0 ||
                       strcasecmp_local(ct, "json") == 0) {
                dynbuf_appendf(&output, "db.json_field(\"%s\", %d)\n",
                    cn, nullable);
            } else if (strcasecmp_local(ct, "uuid") == 0) {
                dynbuf_appendf(&output, "db.uuid_field(\"%s\", %d, 0)\n",
                    cn, nullable);
            } else if (strncasecmp(ct, "numeric", 7) == 0 ||
                       strncasecmp(ct, "decimal", 7) == 0) {
                dynbuf_appendf(&output, "db.decimal_field(\"%s\", 10, 2, %d, 0)\n",
                    cn, nullable);
            } else {
                // Unknown type — emit comment
                dynbuf_appendf(&output, "# db.char_field(\"%s\", 255, %d, 0)  # original: %s\n",
                    cn, nullable, ct);
            }
        }

        if (cols) free(cols);
        dynbuf_append(&output, "\n");
    }

    strlist_free(&tables);

    char* result = output.data;
    output.data = NULL;
    return result;
}
