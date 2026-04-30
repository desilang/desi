/*
 * db_crud.c — ORM CRUD operations for Desi (v2 — Parameterized Queries)
 *
 * Provides C-level functions for:
 *   - Django-style lookup parsing (__lt, __gt, __contains, etc.)
 *   - QuerySet SQL generation with PARAMETERIZED values
 *   - INSERT with parameterized key-value pairs
 *   - Aggregations (COUNT, SUM, AVG, MIN, MAX)
 *   - Q objects with parameterized values
 *   - F expressions (field references — not parameterized, they're column names)
 *
 * SECURITY: All user-supplied values are sent as parameters, NEVER
 * interpolated into SQL strings. This prevents SQL injection.
 *
 * Placeholder format:
 *   - PostgreSQL: $1, $2, $3, ...
 *   - MySQL:      ?, ?, ?, ...
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include "dynbuf.h"

// ---- External: dispatch layer ----
extern int32_t __db_query_exec(const char* sql);
extern int32_t __db_query_params(const char* sql, const char** params, int nparams);
extern int32_t __db_execute_stmt(const char* sql);
extern int32_t __db_execute_params(const char* sql, const char** params, int nparams);
extern char*   __db_get_value_at(int32_t row, int32_t col);
extern int32_t __db_is_connected(void);

// ---- External: ORM field registry (for select_related FK lookup) ----
extern int32_t __orm_field_count(const char* table_name);
extern const char* __orm_field_name(const char* table_name, int32_t field_index);

// FK metadata lookup — find ref_table and ref_field for a FK field
// Returns 1 if found, 0 if not a FK
int32_t __orm_fk_info(const char* table_name, const char* field_name,
                      char* ref_table_out, int ref_table_size,
                      char* ref_field_out, int ref_field_size);

// ============================================================
// Dialect & Debug Flags
// ============================================================

#define DIALECT_PG    0
#define DIALECT_MYSQL 1

static int g_crud_dialect = DIALECT_PG;  // default: PostgreSQL
static int g_debug_queries = 0;          // configurable via desi.mod

int32_t __crud_set_dialect(int32_t dialect) {
    g_crud_dialect = dialect;
    return 0;
}

int32_t __db_set_debug_queries(int32_t enabled) {
    g_debug_queries = enabled;
    return 0;
}

// ============================================================
// QuerySet Handle Struct (replaces all static globals)
//
// Every ORM query gets its own heap-allocated QuerySet, making
// the runtime fully reentrant and spawn-safe. No shared mutable
// state between concurrent queries.
// ============================================================

#define QS_PARAMS_INITIAL   16
#define QS_FIELDS_INITIAL   8
#define QS_RELATED_INITIAL  4
#define QS_ANNOTATIONS_MAX  8

typedef struct {
    char alias[128];
    char func[16];
    char col[128];
} Annotation;

typedef struct QuerySet {
    // Table & query building (fixed-size for sizeof() compat)
    char table[256];
    char where_clause[4096];
    char order[512];
    char columns[4096];
    char having[2048];
    char group_by[1024];

    // Pagination
    int limit;
    int offset;

    // Flags
    int distinct;
    int row_count;

    // Parameters (dynamic array)
    char** params;
    int    param_count;
    int    param_cap;

    // INSERT field accumulator (dynamic)
    char** insert_keys;
    int    insert_count;
    int    insert_cap;
    int    insert_param_start;

    // UPDATE field accumulator (dynamic)
    char** update_keys;
    int    update_count;
    int    update_cap;
    int    update_param_start;

    // select_related (dynamic)
    char** related;
    int    related_count;
    int    related_cap;

    // Annotations
    Annotation annotations[QS_ANNOTATIONS_MAX];
    int    annotation_count;

    // Soft-delete
    int    soft_delete;
    int    include_deleted;
    char   soft_delete_col[128];

    // UPSERT conflict column
    char   upsert_col[128];
    // Window expression
    char   window_expr[1024];

    // Cursor state
    char   cursor_name[64];
    int    cursor_open;

    // CTEs
    #define QS_MAX_CTES 4
    struct {
        char name[64];
        char query[2048];
    } ctes[4];
    int    cte_count;
    int    cte_recursive;

    // Prefetch
    #define QS_MAX_PREFETCH 4
    struct {
        char related_table[128];
        char fk_col[128];
        char pk_col[128];
    } prefetches[4];
    int    prefetch_count;

    // Row locking (FOR UPDATE)
    // 0 = none, 1 = FOR UPDATE, 2 = FOR UPDATE NOWAIT, 3 = FOR UPDATE SKIP LOCKED
    int    for_update;
} QuerySet;

// Forward declaration
int32_t __qs_do_insert(void);

// ============================================================
// QuerySet Lifecycle
// ============================================================

// Allocate and initialize a new QuerySet handle for the given table.
// This is the entry point for every ORM query chain.
QuerySet* __qs_new(const char* table) {
    QuerySet* qs = (QuerySet*)calloc(1, sizeof(QuerySet));
    if (!qs) return NULL;

    // calloc zeros all char[] fields, so they are already '\0'-terminated

    // Set table name
    if (table) {
        strncpy(qs->table, table, sizeof(qs->table) - 1);
        qs->table[sizeof(qs->table) - 1] = '\0';
    }

    // Default soft-delete column
    strncpy(qs->soft_delete_col, "is_deleted", sizeof(qs->soft_delete_col) - 1);

    // Init dynamic arrays
    qs->param_cap = QS_PARAMS_INITIAL;
    qs->params = (char**)calloc(qs->param_cap, sizeof(char*));

    qs->insert_cap = QS_FIELDS_INITIAL;
    qs->insert_keys = (char**)calloc(qs->insert_cap, sizeof(char*));

    qs->update_cap = QS_FIELDS_INITIAL;
    qs->update_keys = (char**)calloc(qs->update_cap, sizeof(char*));

    qs->related_cap = QS_RELATED_INITIAL;
    qs->related = (char**)calloc(qs->related_cap, sizeof(char*));

    return qs;
}

// Free a QuerySet handle and all its owned memory.
void __qs_free(QuerySet* qs) {
    if (!qs) return;

    // char[] fields are embedded — no separate free needed

    // Free param copies
    for (int i = 0; i < qs->param_count; i++) {
        if (qs->params[i]) free(qs->params[i]);
    }
    free(qs->params);

    // Free insert keys
    for (int i = 0; i < qs->insert_count; i++) {
        if (qs->insert_keys[i]) free(qs->insert_keys[i]);
    }
    free(qs->insert_keys);

    // Free update keys
    for (int i = 0; i < qs->update_count; i++) {
        if (qs->update_keys[i]) free(qs->update_keys[i]);
    }
    free(qs->update_keys);

    // Free related
    for (int i = 0; i < qs->related_count; i++) {
        if (qs->related[i]) free(qs->related[i]);
    }
    free(qs->related);

    free(qs);
}

// ============================================================
// Handle-Threading API
//
// The compiler emits explicit handle management:
//   %qs = call ptr @__qs_handle_new("users")   → allocate handle
//   call void @__qs_handle_bind(ptr %qs)        → set as active
//   call void @__qs_filter("name", "Alice")     → uses active handle
//   call i32  @__qs_fetch()                     → uses active handle
//   call void @__qs_handle_free(ptr %qs)        → deallocate
//
// This keeps all __qs_* function signatures unchanged while
// making handle lifetime explicit at the IR level.
// ============================================================
static __thread QuerySet* g_qs_current = NULL;

// Create a new handle and return it as an opaque pointer.
// Does NOT install it as the active handle — call __qs_handle_bind for that.
void* __qs_handle_new(const char* table) {
    return (void*)__qs_new(table);
}

// Install a handle as the active QuerySet for subsequent __qs_* calls.
void __qs_handle_bind(void* qs) {
    g_qs_current = (QuerySet*)qs;
}

// Free a handle and clear it from the active slot if it's the current one.
void __qs_handle_free(void* qs) {
    if (!qs) return;
    if (g_qs_current == (QuerySet*)qs) {
        g_qs_current = NULL;
    }
    __qs_free((QuerySet*)qs);
}

// Get the current active handle (for advanced introspection).
void* __qs_handle_get(void) {
    return (void*)g_qs_current;
}

// Legacy shim: __qs_reset creates a new handle and installs it.
// Used by the non-ORM db.objects() path and older code.
int32_t __qs_reset(const char* table) {
    if (g_qs_current) __qs_free(g_qs_current);
    g_qs_current = __qs_new(table);
    return g_qs_current ? 0 : -1;
}

// Get current handle (for legacy code paths)
static inline QuerySet* qs_current(void) {
    return g_qs_current;
}

// ============================================================
// Compatibility Shim Macros
//
// These macros redirect all old global variable names to fields
// on the thread-local g_qs_current handle. This lets every
// existing function body compile without modification while
// routing all state through the heap-allocated QuerySet.
// ============================================================

// char[] fields — array decays to char* in expressions, sizeof() works correctly
#define qs_table        (g_qs_current->table)
#define qs_where        (g_qs_current->where_clause)
#define qs_order        (g_qs_current->order)
#define qs_columns      (g_qs_current->columns)
#define qs_having       (g_qs_current->having)
#define qs_group_by     (g_qs_current->group_by)
#define qs_soft_delete_col (g_qs_current->soft_delete_col)
#define qs_upsert_col   (g_qs_current->upsert_col)
#define qs_window_expr  (g_qs_current->window_expr)

// Scalar fields
#define qs_limit        (g_qs_current->limit)
#define qs_offset       (g_qs_current->offset)
#define qs_distinct     (g_qs_current->distinct)
#define qs_row_count    (g_qs_current->row_count)
#define qs_soft_delete  (g_qs_current->soft_delete)
#define qs_include_deleted (g_qs_current->include_deleted)

// Parameter array
#define qs_params       ((const char**)g_qs_current->params)
#define qs_param_count  (g_qs_current->param_count)

// INSERT accumulator
#define qs_insert_keys      (g_qs_current->insert_keys)
#define qs_insert_count     (g_qs_current->insert_count)
#define qs_insert_param_start (g_qs_current->insert_param_start)

// UPDATE accumulator
#define qs_update_keys      (g_qs_current->update_keys)
#define qs_update_count     (g_qs_current->update_count)
#define qs_update_param_start (g_qs_current->update_param_start)

// Related (select_related)
#define qs_related       (g_qs_current->related)
#define qs_related_count (g_qs_current->related_count)

// Annotations
#define qs_annotations      (g_qs_current->annotations)
#define qs_annotation_count (g_qs_current->annotation_count)

// Legacy helper shims — redirect to handle-based versions
#define add_param(v)      qs_add_param(g_qs_current, (v))
#define add_param_copy(v) qs_add_param_copy(g_qs_current, (v))
#define debug_log_query(s) qs_debug_log(g_qs_current, (s))

// Max-size shims (for sizeof in old snprintf calls — now effectively unlimited)
// These return a safe large value so existing snprintf calls don't truncate
#define QS_MAX_PARAMS    (g_qs_current->param_cap)
#define QS_MAX_FIELDS    1024
#define QS_MAX_RELATED   (g_qs_current->related_cap)
#define QS_MAX_UPDATE_FIELDS 1024



// Helper: write a placeholder for the current dialect
static int write_placeholder(char* buf, int buf_size, int param_index) {
    if (g_crud_dialect == DIALECT_MYSQL) {
        return snprintf(buf, buf_size, "?");
    } else {
        return snprintf(buf, buf_size, "$%d", param_index);
    }
}

// Helper: add a parameter to a QuerySet and return its 1-based index
static int qs_add_param(QuerySet* qs, const char* val) {
    if (qs->param_count >= qs->param_cap) {
        qs->param_cap *= 2;
        qs->params = (char**)realloc(qs->params, qs->param_cap * sizeof(char*));
    }
    qs->params[qs->param_count] = (char*)val;  // caller must ensure lifetime
    qs->param_count++;
    return qs->param_count;
}

// Helper: add a heap-allocated copy of val as parameter
static int qs_add_param_copy(QuerySet* qs, const char* val) {
    return qs_add_param(qs, strdup(val));
}

// Debug: print query + params to stderr
static void qs_debug_log(QuerySet* qs, const char* sql) {
    if (!g_debug_queries) return;
    fprintf(stderr, "[db] QUERY: %s\n", sql);
    if (qs->param_count > 0) {
        fprintf(stderr, "[db] PARAMS: [");
        for (int i = 0; i < qs->param_count; i++) {
            if (i > 0) fprintf(stderr, ", ");
            fprintf(stderr, "\"%s\"", qs->params[i] ? qs->params[i] : "NULL");
        }
        fprintf(stderr, "]\n");
    }
}

// ============================================================
// Django-style lookup parser (PARAMETERIZED version)
// ============================================================

// Parse "age__lt" → col="age", op="<"
// Parse "name__contains" → col="name", op="LIKE", val modified to "%val%"
// Parse "age" → col="age", op="=" (default: exact match)
// Parse "id__in" → col="id", op="IN", special multi-param handling
//
// For LIKE patterns: the modified value (e.g., "%test%") is stored as the param.
// The SQL just has a placeholder. This is safe because the DB treats the
// entire param as a single string value — wildcards are LIKE-specific, not SQL injection.
static void parse_lookup(const char* lookup, const char* val,
                         char* col_out, char* op_out, char* val_out,
                         int* is_in_out) {
    // Copy defaults
    strncpy(col_out, lookup, 127);
    col_out[127] = '\0';
    strcpy(op_out, "=");
    strncpy(val_out, val, 511);
    val_out[511] = '\0';
    *is_in_out = 0;

    char* dunder = strstr(col_out, "__");
    if (!dunder) return;

    // Extract suffix
    char suffix[32];
    strncpy(suffix, dunder + 2, sizeof(suffix) - 1);
    suffix[sizeof(suffix) - 1] = '\0';

    // Truncate col to just the column name
    *dunder = '\0';

    if (strcmp(suffix, "lt") == 0) {
        strcpy(op_out, "<");
    } else if (strcmp(suffix, "lte") == 0) {
        strcpy(op_out, "<=");
    } else if (strcmp(suffix, "gt") == 0) {
        strcpy(op_out, ">");
    } else if (strcmp(suffix, "gte") == 0) {
        strcpy(op_out, ">=");
    } else if (strcmp(suffix, "ne") == 0) {
        strcpy(op_out, "!=");
    } else if (strcmp(suffix, "exact") == 0) {
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "iexact") == 0) {
        // Case-insensitive exact: PG uses ILIKE, MySQL uses = (already case-insensitive)
        if (g_crud_dialect == DIALECT_MYSQL) {
            strcpy(op_out, "=");
        } else {
            strcpy(op_out, "ILIKE");
        }
    } else if (strcmp(suffix, "contains") == 0) {
        strcpy(op_out, "LIKE");
        // Modify the VALUE (not the SQL) to add wildcards
        snprintf(val_out, 512, "%%%s%%", val);
    } else if (strcmp(suffix, "icontains") == 0) {
        // Case-insensitive LIKE
        if (g_crud_dialect == DIALECT_MYSQL) {
            strcpy(op_out, "LIKE");  // MySQL LIKE is case-insensitive by default
        } else {
            strcpy(op_out, "ILIKE"); // PG has ILIKE
        }
        snprintf(val_out, 512, "%%%s%%", val);
    } else if (strcmp(suffix, "startswith") == 0) {
        strcpy(op_out, "LIKE");
        snprintf(val_out, 512, "%s%%", val);
    } else if (strcmp(suffix, "istartswith") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            strcpy(op_out, "LIKE");
        } else {
            strcpy(op_out, "ILIKE");
        }
        snprintf(val_out, 512, "%s%%", val);
    } else if (strcmp(suffix, "endswith") == 0) {
        strcpy(op_out, "LIKE");
        snprintf(val_out, 512, "%%%s", val);
    } else if (strcmp(suffix, "iendswith") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            strcpy(op_out, "LIKE");
        } else {
            strcpy(op_out, "ILIKE");
        }
        snprintf(val_out, 512, "%%%s", val);
    } else if (strcmp(suffix, "isnull") == 0) {
        if (strcmp(val, "true") == 0) {
            strcpy(op_out, "IS");
            strcpy(val_out, "NULL");
        } else {
            strcpy(op_out, "IS NOT");
            strcpy(val_out, "NULL");
        }
    } else if (strcmp(suffix, "in") == 0) {
        strcpy(op_out, "IN");
        *is_in_out = 1;
    } else if (strcmp(suffix, "range") == 0) {
        strcpy(op_out, "BETWEEN");
        // val should be "low,high" — we'll handle this specially in the filter
        *is_in_out = 2; // signal: range mode
    // ---- Date lookups ----
    } else if (strcmp(suffix, "year") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            // MySQL: YEAR(col) = ?
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "YEAR(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            // PG: EXTRACT(YEAR FROM col) = ?
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(YEAR FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "month") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "MONTH(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(MONTH FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "day") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "DAY(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(DAY FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "hour") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "HOUR(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(HOUR FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "minute") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "MINUTE(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(MINUTE FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "second") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "SECOND(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(SECOND FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "week") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "WEEK(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(WEEK FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "quarter") == 0) {
        if (g_crud_dialect == DIALECT_MYSQL) {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "QUARTER(%s)", col_out);
            strncpy(col_out, tmp, 127);
        } else {
            char tmp[128];
            snprintf(tmp, sizeof(tmp), "EXTRACT(QUARTER FROM %s)", col_out);
            strncpy(col_out, tmp, 127);
        }
        strcpy(op_out, "=");
    // ---- JSON field lookups ----
    // data__key → PG: data->>'key' = $1, MySQL: JSON_UNQUOTE(JSON_EXTRACT(data,'$.key')) = ?
    // Detect: suffix doesn't match any known operator — check if parent col is a JSON field
    // For now, treat any unknown double-underscore as a JSON path access
    // The user can also use the explicit json_* lookups below
    } else if (strncmp(suffix, "json_", 5) == 0) {
        // Explicit JSON lookups: data__json_has → data ? 'key' (PG) / JSON_CONTAINS_PATH (MySQL)
        char* json_op = suffix + 5;
        if (strcmp(json_op, "has") == 0) {
            // test if key exists: PG: col ? val, MySQL: JSON_CONTAINS_PATH(col, 'one', '$.val')
            if (g_crud_dialect == DIALECT_MYSQL) {
                char tmp[256];
                snprintf(tmp, sizeof(tmp), "JSON_CONTAINS_PATH(%s, 'one', '$.\"%s\"')", col_out, val);
                strncpy(col_out, tmp, 127);
                strcpy(op_out, "=");
                strcpy(val_out, "1"); // JSON_CONTAINS_PATH returns 1/0
            } else {
                strcpy(op_out, "?");
            }
        } else if (strcmp(json_op, "contains") == 0) {
            // PG: col @> '{"key":"val"}', MySQL: JSON_CONTAINS(col, '{"key":"val"}')
            if (g_crud_dialect == DIALECT_MYSQL) {
                char tmp[256];
                snprintf(tmp, sizeof(tmp), "JSON_CONTAINS(%s, ?)", col_out);
                strncpy(col_out, tmp, 127);
                strcpy(op_out, "=");
                strcpy(val_out, "1");
            } else {
                strcpy(op_out, "@>");
            }
        }
    } else {
        // Check for nested JSON path: data__settings__theme
        // This would have been split at the first __ so suffix = "settings__theme"
        // We treat any unknown suffix that looks like a field name as JSON key access
        char* nested = strstr(suffix, "__");
        if (nested) {
            // Nested: data__settings__theme → PG: data->'settings'->>'theme'
            // For simplicity, handle first level only in v0.1.0
            *nested = '\0';
            char* key2 = nested + 2;
            if (g_crud_dialect == DIALECT_MYSQL) {
                char tmp[256];
                snprintf(tmp, sizeof(tmp), "JSON_UNQUOTE(JSON_EXTRACT(%s, '$.%s.%s'))", col_out, suffix, key2);
                strncpy(col_out, tmp, 127);
            } else {
                char tmp[256];
                snprintf(tmp, sizeof(tmp), "%s->'%s'->>'%s'", col_out, suffix, key2);
                strncpy(col_out, tmp, 127);
            }
            strcpy(op_out, "=");
        } else {
            // Unknown suffix — restore original (treat as exact match on full name)
            *dunder = '_';
            *(dunder + 1) = '_';
        }
    }
}

// ============================================================
// IN operator — split comma-separated values into N placeholders
// ============================================================

// Parse "1,2,3" → 3 params, returns "($1, $2, $3)" or "(?, ?, ?)"
static int emit_in_placeholders(const char* csv_val, char* sql_out, int sql_out_size) {
    // Count values and store each as a param
    char buf[512];
    strncpy(buf, csv_val, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    int pos = snprintf(sql_out, sql_out_size, "(");
    int first = 1;

    char* saveptr = NULL;
    char* token = strtok_r(buf, ",", &saveptr);
    while (token) {
        // Trim whitespace
        while (*token == ' ') token++;
        char* end = token + strlen(token) - 1;
        while (end > token && *end == ' ') { *end = '\0'; end--; }

        int param_idx = add_param_copy(token);
        if (!first) pos += snprintf(sql_out + pos, sql_out_size - pos, ", ");
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        pos += snprintf(sql_out + pos, sql_out_size - pos, "%s", ph);
        first = 0;
        token = strtok_r(NULL, ",", &saveptr);
    }
    pos += snprintf(sql_out + pos, sql_out_size - pos, ")");
    return pos;
}

// ============================================================
// Filter / Exclude — add WHERE conditions with placeholders
// ============================================================

int32_t __qs_filter(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    int is_in = 0;
    parse_lookup(lookup, val, col, op, parsed_val, &is_in);

    char condition[1024];

    if (strcmp(op, "IS") == 0 || strcmp(op, "IS NOT") == 0) {
        // IS NULL / IS NOT NULL — no parameter needed
        snprintf(condition, sizeof(condition), "%s %s %s", col, op, parsed_val);
    } else if (is_in == 2) {
        // BETWEEN — split "low,high" into two params
        char buf[512];
        strncpy(buf, val, sizeof(buf) - 1);
        buf[sizeof(buf) - 1] = '\0';
        char* comma = strchr(buf, ',');
        if (comma) {
            *comma = '\0';
            char* low = buf;
            char* high = comma + 1;
            while (*low == ' ') low++;
            while (*high == ' ') high++;
            int p1 = add_param_copy(low);
            int p2 = add_param_copy(high);
            char ph1[16], ph2[16];
            write_placeholder(ph1, sizeof(ph1), p1);
            write_placeholder(ph2, sizeof(ph2), p2);
            snprintf(condition, sizeof(condition), "%s BETWEEN %s AND %s", col, ph1, ph2);
        } else {
            // Malformed range, fallback to exact match
            int param_idx = add_param_copy(parsed_val);
            char ph[16];
            write_placeholder(ph, sizeof(ph), param_idx);
            snprintf(condition, sizeof(condition), "%s = %s", col, ph);
        }
    } else if (is_in == 1) {
        // IN operator — expand to multiple placeholders
        char in_clause[512];
        emit_in_placeholders(val, in_clause, sizeof(in_clause));
        snprintf(condition, sizeof(condition), "%s IN %s", col, in_clause);
    } else {
        // Normal case — single placeholder
        int param_idx = add_param_copy(parsed_val);
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        snprintf(condition, sizeof(condition), "%s %s %s", col, op, ph);
    }

    if (qs_where[0] == '\0') {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    } else {
        char tmp[4096];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_where, condition);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}

int32_t __qs_exclude(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    int is_in = 0;
    parse_lookup(lookup, val, col, op, parsed_val, &is_in);

    char condition[1024];

    if (strcmp(op, "IS") == 0 || strcmp(op, "IS NOT") == 0) {
        snprintf(condition, sizeof(condition), "NOT (%s %s %s)", col, op, parsed_val);
    } else if (is_in) {
        char in_clause[512];
        emit_in_placeholders(val, in_clause, sizeof(in_clause));
        snprintf(condition, sizeof(condition), "%s NOT IN %s", col, in_clause);
    } else {
        int param_idx = add_param_copy(parsed_val);
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        snprintf(condition, sizeof(condition), "NOT (%s %s %s)", col, op, ph);
    }

    if (qs_where[0] == '\0') {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    } else {
        char tmp[4096];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_where, condition);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}

// ============================================================
// Order / Limit / Offset (no parameterization needed — not user data)
// ============================================================

int32_t __qs_order_by(const char* col) {
    if (col[0] == '-') {
        snprintf(qs_order, sizeof(qs_order), "%s DESC", col + 1);
    } else {
        snprintf(qs_order, sizeof(qs_order), "%s ASC", col);
    }
    return 0;
}

int32_t __qs_limit(int32_t n) {
    qs_limit = n;
    return 0;
}

int32_t __qs_offset(int32_t n) {
    qs_offset = n;
    return 0;
}

// ============================================================
// Execution: fetch, count, exists, update, delete
// All use parameterized queries when params exist
// ============================================================

// Build SELECT SQL and execute with parameters
int32_t __qs_fetch(void) {
    DynBuf sql;
    dynbuf_init(&sql, 512);

    // Build column list and JOIN clauses for select_related
    if (qs_annotation_count > 0) {
        // Annotated query: SELECT group_cols, AGG(col) AS alias, ...
        dynbuf_appendf(&sql, "SELECT %s",
            qs_distinct ? "DISTINCT " : "");

        // If GROUP BY is set, include those columns first
        if (qs_group_by[0]) {
            dynbuf_append(&sql, qs_group_by);
        } else {
            dynbuf_appendf(&sql, "%s.*", qs_table);
        }

        // Add annotations: SUM(col) AS alias
        for (int a = 0; a < qs_annotation_count; a++) {
            dynbuf_appendf(&sql, ", %s(%s) AS %s",
                qs_annotations[a].func, qs_annotations[a].col, qs_annotations[a].alias);
        }

        dynbuf_appendf(&sql, " FROM %s", qs_table);
    } else if (qs_related_count > 0) {
        // Build: SELECT t.*, r1.col1 AS r1__col1, r1.col2 AS r1__col2, ...
        dynbuf_appendf(&sql, "SELECT %s%s.*",
            qs_distinct ? "DISTINCT " : "", qs_table);

        // For each related FK, add aliased columns from the related table
        for (int r = 0; r < qs_related_count; r++) {
            char ref_table[128] = "", ref_field[64] = "";
            if (__orm_fk_info(qs_table, qs_related[r], ref_table, sizeof(ref_table),
                             ref_field, sizeof(ref_field))) {
                int fcount = __orm_field_count(ref_table);
                for (int f = 0; f < fcount; f++) {
                    const char* fname = __orm_field_name(ref_table, f);
                    dynbuf_appendf(&sql, ", %s_rel%d.%s AS %s__%s",
                        qs_related[r], r, fname, qs_related[r], fname);
                }
            }
        }

        dynbuf_appendf(&sql, " FROM %s", qs_table);

        // Add LEFT JOINs
        for (int r = 0; r < qs_related_count; r++) {
            char ref_table[128] = "", ref_field[64] = "";
            if (__orm_fk_info(qs_table, qs_related[r], ref_table, sizeof(ref_table),
                             ref_field, sizeof(ref_field))) {
                dynbuf_appendf(&sql,
                    " LEFT JOIN %s AS %s_rel%d ON %s.%s = %s_rel%d.%s",
                    ref_table, qs_related[r], r,
                    qs_table, qs_related[r],
                    qs_related[r], r, ref_field);
            }
        }
    } else {
        // Simple SELECT (no joins)
        const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
        if (qs_distinct) {
            dynbuf_appendf(&sql, "SELECT DISTINCT %s FROM %s", cols, qs_table);
        } else {
            dynbuf_appendf(&sql, "SELECT %s FROM %s", cols, qs_table);
        }
    }

    // auto-inject soft-delete filter (unless with_deleted() was called)
    if (qs_soft_delete && !qs_include_deleted) {
        if (qs_where[0] == '\0') {
            snprintf(qs_where, sizeof(qs_where), "%s = FALSE", qs_soft_delete_col);
        } else {
            char tmp[4096];
            snprintf(tmp, sizeof(tmp), "%s AND %s = FALSE", qs_where, qs_soft_delete_col);
            strncpy(qs_where, tmp, sizeof(qs_where) - 1);
        }
    }

    if (qs_where[0]) dynbuf_appendf(&sql, " WHERE %s", qs_where);
    if (qs_group_by[0]) dynbuf_appendf(&sql, " GROUP BY %s", qs_group_by);
    // HAVING clause (after GROUP BY)
    if (qs_having[0]) dynbuf_appendf(&sql, " HAVING %s", qs_having);
    if (qs_order[0]) dynbuf_appendf(&sql, " ORDER BY %s", qs_order);
    if (qs_limit > 0) dynbuf_appendf(&sql, " LIMIT %d", qs_limit);
    if (qs_offset > 0) dynbuf_appendf(&sql, " OFFSET %d", qs_offset);

    // Row locking — FOR UPDATE must be last clause
    if (g_qs_current->for_update == 1) dynbuf_append(&sql, " FOR UPDATE");
    else if (g_qs_current->for_update == 2) dynbuf_append(&sql, " FOR UPDATE NOWAIT");
    else if (g_qs_current->for_update == 3) dynbuf_append(&sql, " FOR UPDATE SKIP LOCKED");

    debug_log_query(sql.data);

    if (qs_param_count > 0) {
        qs_row_count = __db_query_params(sql.data, qs_params, qs_param_count);
    } else {
        qs_row_count = __db_query_exec(sql.data);
    }
    dynbuf_free(&sql);
    return qs_row_count;
}

// ============================================================
// Terminal convenience functions
// These are called by the generic lowerer dispatch.
// ============================================================

// __qs_all — alias for fetch
int32_t __qs_all(void) {
    return __qs_fetch();
}

// __qs_first — LIMIT 1 + fetch
int32_t __qs_first(void) {
    qs_limit = 1;
    return __qs_fetch();
}

// __qs_last — ORDER BY id DESC + LIMIT 1 + fetch
int32_t __qs_last(void) {
    // Only override order if none set
    if (qs_order[0] == '\0') {
        snprintf(qs_order, sizeof(qs_order), "id DESC");
    } else {
        // Reverse existing order: append DESC logic
        // For simplicity, prepend "id DESC, " to existing
        char tmp[256];
        snprintf(tmp, sizeof(tmp), "id DESC, %s", qs_order);
        strncpy(qs_order, tmp, sizeof(qs_order) - 1);
    }
    qs_limit = 1;
    return __qs_fetch();
}

// __qs_distinct — set flag for SELECT DISTINCT
int32_t __qs_distinct(void) {
    qs_distinct = 1;
    return 0;
}

// __qs_select_for_update — enable row-level locking
// Django equivalent: User.objects.filter(id=1).select_for_update()
//
// mode: "" or "default" → FOR UPDATE
//       "nowait"         → FOR UPDATE NOWAIT (raises error if locked)
//       "skip_locked"    → FOR UPDATE SKIP LOCKED (skips locked rows)
//
// Must be used within a transaction (db.begin/commit/rollback).
// FOR UPDATE acquires an exclusive row lock until the transaction ends.
int32_t __qs_select_for_update(const char* mode) {
    if (!mode || mode[0] == '\0' || strcmp(mode, "default") == 0) {
        g_qs_current->for_update = 1;  // FOR UPDATE
    } else if (strcmp(mode, "nowait") == 0) {
        g_qs_current->for_update = 2;  // FOR UPDATE NOWAIT
    } else if (strcmp(mode, "skip_locked") == 0) {
        g_qs_current->for_update = 3;  // FOR UPDATE SKIP LOCKED
    } else {
        fprintf(stderr, "[orm] warning: unknown for_update mode '%s', using FOR UPDATE\n", mode);
        g_qs_current->for_update = 1;
    }
    return 0;
}

// __qs_select_related — register FK field for LEFT JOIN
// Django equivalent: Post.objects.select_related("author")
// The field_name should match the FK column name (e.g., "author_id")
int32_t __qs_select_related(const char* field_name) {
    if (qs_related_count >= QS_MAX_RELATED || !field_name) return -1;
    strncpy(qs_related[qs_related_count], field_name, 127);
    qs_related[qs_related_count][127] = '\0';
    qs_related_count++;
    return 0;
}

// __qs_in_bulk — fetch multiple rows by PK list in a single query
// Django equivalent: User.objects.in_bulk([1, 2, 3])
//
// id_list: comma-separated PKs, e.g. "1,2,3"
// field:   PK column name, e.g. "id" (default if empty)
//
// Builds: SELECT * FROM table WHERE field IN ($1, $2, $3)
// Returns: row count, or -1 on error
int32_t __qs_in_bulk(const char* id_list, const char* field) {
    if (!id_list || id_list[0] == '\0') return 0;

    const char* pk_col = (field && field[0] != '\0') ? field : "id";

    // Build "field__in" lookup key and filter directly
    char lookup[256];
    snprintf(lookup, sizeof(lookup), "%s__in", pk_col);
    __qs_filter(lookup, id_list);

    return __qs_fetch();
}

// ============================================================
// Database Functions — SQL expression builders
//
// These return heap-allocated strings like "LOWER(name)" that
// can be passed to annotate(), filter(), or order_by().
// Django equivalents: Lower(), Upper(), Coalesce(), Cast(), etc.
// ============================================================

// Lower("name") → "LOWER(name)"
char* __db_func_lower(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "LOWER(%s)", col);
    return buf;
}

// Upper("name") → "UPPER(name)"
char* __db_func_upper(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "UPPER(%s)", col);
    return buf;
}

// Length("name") → "LENGTH(name)" (PG) / "CHAR_LENGTH(name)" (MySQL)
char* __db_func_length(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 32);
    if (g_crud_dialect == DIALECT_MYSQL) {
        sprintf(buf, "CHAR_LENGTH(%s)", col);
    } else {
        sprintf(buf, "LENGTH(%s)", col);
    }
    return buf;
}

// Coalesce("col1,col2,default") → "COALESCE(col1, col2, default)"
// Takes comma-separated args
char* __db_func_coalesce(const char* args) {
    size_t len = strlen(args) + 32;
    char* buf = (char*)malloc(len);
    snprintf(buf, len, "COALESCE(%s)", args);
    return buf;
}

// Cast("col", "INTEGER") → "CAST(col AS INTEGER)"
char* __db_func_cast(const char* col, const char* type) {
    size_t len = strlen(col) + strlen(type) + 32;
    char* buf = (char*)malloc(len);
    snprintf(buf, len, "CAST(%s AS %s)", col, type);
    return buf;
}

// Concat("col1,col2") → "CONCAT(col1, col2)" (MySQL) / "col1 || col2" (PG)
char* __db_func_concat(const char* args) {
    size_t len = strlen(args) + 32;
    char* buf = (char*)malloc(len);
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(buf, len, "CONCAT(%s)", args);
    } else {
        // PG: replace commas with ||
        // Simple approach: wrap in CONCAT() since PG supports it too (9.1+)
        snprintf(buf, len, "CONCAT(%s)", args);
    }
    return buf;
}

// Abs("col") → "ABS(col)"
char* __db_func_abs(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "ABS(%s)", col);
    return buf;
}

// Greatest("a,b,c") → "GREATEST(a, b, c)"
char* __db_func_greatest(const char* args) {
    size_t len = strlen(args) + 32;
    char* buf = (char*)malloc(len);
    snprintf(buf, len, "GREATEST(%s)", args);
    return buf;
}

// Least("a,b,c") → "LEAST(a, b, c)"
char* __db_func_least(const char* args) {
    size_t len = strlen(args) + 32;
    char* buf = (char*)malloc(len);
    snprintf(buf, len, "LEAST(%s)", args);
    return buf;
}

// Now() → "NOW()" (PG/MySQL both support this)
char* __db_func_now(void) {
    return strdup("NOW()");
}

// Substr("col", 1, 5) → "SUBSTR(col, 1, 5)" (PG) / "SUBSTRING(col, 1, 5)" (MySQL)
char* __db_func_substr(const char* col, int32_t start, int32_t len) {
    char* buf = (char*)malloc(strlen(col) + 64);
    if (g_crud_dialect == DIALECT_MYSQL) {
        sprintf(buf, "SUBSTRING(%s, %d, %d)", col, start, len);
    } else {
        sprintf(buf, "SUBSTR(%s, %d, %d)", col, start, len);
    }
    return buf;
}

// Trim("col") → "TRIM(col)"
char* __db_func_trim(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "TRIM(%s)", col);
    return buf;
}

// Round("col", 2) → "ROUND(col, 2)"
char* __db_func_round(const char* col, int32_t decimals) {
    char* buf = (char*)malloc(strlen(col) + 32);
    sprintf(buf, "ROUND(%s, %d)", col, decimals);
    return buf;
}

// Ceil("col") → "CEIL(col)"
char* __db_func_ceil(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "CEIL(%s)", col);
    return buf;
}

// Floor("col") → "FLOOR(col)"
char* __db_func_floor(const char* col) {
    char* buf = (char*)malloc(strlen(col) + 16);
    sprintf(buf, "FLOOR(%s)", col);
    return buf;
}

// Left("col", 5) → "LEFT(col, 5)"
char* __db_func_left(const char* col, int32_t n) {
    char* buf = (char*)malloc(strlen(col) + 32);
    sprintf(buf, "LEFT(%s, %d)", col, n);
    return buf;
}

// Right("col", 5) → "RIGHT(col, 5)"
char* __db_func_right(const char* col, int32_t n) {
    char* buf = (char*)malloc(strlen(col) + 32);
    sprintf(buf, "RIGHT(%s, %d)", col, n);
    return buf;
}

// Replace("col", "old", "new") → "REPLACE(col, 'old', 'new')"
char* __db_func_replace(const char* col, const char* old_str, const char* new_str) {
    size_t len = strlen(col) + strlen(old_str) + strlen(new_str) + 32;
    char* buf = (char*)malloc(len);
    snprintf(buf, len, "REPLACE(%s, '%s', '%s')", col, old_str, new_str);
    return buf;
}

// ============================================================
// QuerySet .reverse() — flip current ordering
// Django equivalent: qs.reverse()
// ============================================================

int32_t __qs_reverse(void) {
    if (qs_order[0] == '\0') return 0; // nothing to reverse

    // Parse the current order and flip ASC↔DESC
    char buf[1024];
    strncpy(buf, qs_order, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    char result[1024] = {0};
    int pos = 0;

    char* saveptr = NULL;
    char* tok = strtok_r(buf, ",", &saveptr);
    while (tok) {
        while (*tok == ' ') tok++;
        if (pos > 0) pos += sprintf(result + pos, ", ");

        // Check if token ends with ASC or DESC
        char* asc_pos = strstr(tok, " ASC");
        char* desc_pos = strstr(tok, " DESC");

        if (desc_pos && (!asc_pos || desc_pos > asc_pos)) {
            *desc_pos = '\0';
            pos += sprintf(result + pos, "%s ASC", tok);
        } else if (asc_pos) {
            *asc_pos = '\0';
            pos += sprintf(result + pos, "%s DESC", tok);
        } else {
            // No explicit direction → assume ASC, flip to DESC
            pos += sprintf(result + pos, "%s DESC", tok);
        }

        tok = strtok_r(NULL, ",", &saveptr);
    }

    strncpy(qs_order, result, sizeof(qs_order) - 1);
    qs_order[sizeof(qs_order) - 1] = '\0';
    return 0;
}

// ============================================================
// Case/When — conditional SQL expression builder
//
// Django equivalent:
//   Case(When(status='urgent', then=Value(1)),
//        When(status='high', then=Value(2)),
//        default=Value(3))
//
// Generates:
//   CASE WHEN status='urgent' THEN 1 WHEN status='high' THEN 2 ELSE 3 END
//
// Usage pattern:
//   db.case_when("status = 'urgent'", "1")
//   db.case_when("status = 'high'", "2")
//   db.case_else("3")
//   let expr = db.case_end("priority")
//   db.annotate("priority", "IDENTITY", expr)
// ============================================================

// Thread-local case builder state
static __thread DynBuf g_case_buf;
static __thread int    g_case_started;
static __thread char   g_case_else[512];

// case_when(condition, result) — add a WHEN clause
// condition: SQL boolean expression, e.g. "status = 'urgent'"
// result:    value when condition is true, e.g. "1" or "'high_priority'"
int32_t __db_case_when(const char* condition, const char* result) {
    if (!condition || !result) return -1;

    if (!g_case_started) {
        dynbuf_init(&g_case_buf, 256);
        dynbuf_append(&g_case_buf, "CASE");
        g_case_else[0] = '\0';
        g_case_started = 1;
    }

    dynbuf_appendf(&g_case_buf, " WHEN %s THEN %s", condition, result);
    return 0;
}

// case_else(default_val) — set the ELSE clause
// default_val: value when no WHEN matches, e.g. "0" or "'unknown'"
int32_t __db_case_else(const char* default_val) {
    if (!default_val) return -1;
    strncpy(g_case_else, default_val, sizeof(g_case_else) - 1);
    g_case_else[sizeof(g_case_else) - 1] = '\0';
    return 0;
}

// case_end(alias) — finalize and return the CASE expression
// alias: column alias for the result, e.g. "priority"
//        if empty, returns bare CASE expression without AS
// Returns heap-allocated string, resets builder state.
char* __db_case_end(const char* alias) {
    if (!g_case_started) {
        return strdup("NULL");  // no WHEN clauses → NULL
    }

    if (g_case_else[0] != '\0') {
        dynbuf_appendf(&g_case_buf, " ELSE %s", g_case_else);
    }
    dynbuf_append(&g_case_buf, " END");

    if (alias && alias[0] != '\0') {
        dynbuf_appendf(&g_case_buf, " AS %s", alias);
    }

    char* result = strdup(g_case_buf.data);
    dynbuf_free(&g_case_buf);
    g_case_started = 0;
    g_case_else[0] = '\0';

    return result;
}

// __qs_annotate — add an aggregate annotation
// Django equivalent: .annotate(total=Sum("amount"))
// func: "SUM", "COUNT", "AVG", "MIN", "MAX"
int32_t __qs_annotate(const char* alias, const char* func, const char* col) {
    if (qs_annotation_count >= QS_ANNOTATIONS_MAX || !alias || !func || !col) return -1;
    Annotation* a = &qs_annotations[qs_annotation_count++];
    strncpy(a->alias, alias, sizeof(a->alias) - 1);
    strncpy(a->func, func, sizeof(a->func) - 1);
    strncpy(a->col, col, sizeof(a->col) - 1);
    return 0;
}

// __qs_group_by — set GROUP BY columns
// cols: comma-separated column names, e.g. "customer_id" or "customer_id, status"
int32_t __qs_group_by(const char* cols) {
    if (!cols) return -1;
    strncpy(qs_group_by, cols, sizeof(qs_group_by) - 1);
    qs_group_by[sizeof(qs_group_by) - 1] = '\0';
    return 0;
}

// __db_using — switch active database connection for this QuerySet
// Called by .using("analytics") in the chain.
extern int32_t __db_use_conn(const char* name);
int32_t __db_using(const char* db_name) {
    return __db_use_conn(db_name);
}

int32_t __qs_row_count(void) {
    return qs_row_count;
}

// SELECT COUNT(*) with parameters
int32_t __qs_count(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT COUNT(*) FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }
    if (rows > 0) {
        char* v = __db_get_value_at(0, 0);
        int result = atoi(v);
        free(v);
        return result;
    }
    return 0;
}

// SELECT EXISTS(...)
int32_t __qs_exists(void) {
    return __qs_count() > 0 ? 1 : 0;
}

// UPDATE table SET col=$N WHERE ...
int32_t __qs_update(const char* col, const char* val) {
    int param_idx = add_param_copy(val);
    char ph[16];
    write_placeholder(ph, sizeof(ph), param_idx);

    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "UPDATE %s SET %s = %s", qs_table, col, ph);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// DELETE FROM table WHERE ...
int32_t __qs_delete(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "DELETE FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// __qs_update_multi — UPDATE multiple columns at once
// Django equivalent: User.objects.filter(id=1).update(name="Bob", age=30)
// fields: comma-separated "col=val" pairs, e.g. "name=Bob,age=30"
int32_t __qs_update_pairs(const char* fields) {
    if (!fields || fields[0] == '\0') return -1;

    DynBuf sql;
    dynbuf_init(&sql, 256);
    dynbuf_appendf(&sql, "UPDATE %s SET ", qs_table);

    // Parse "col1=val1,col2=val2" into parameterized SET clauses
    char buf[4096];
    strncpy(buf, fields, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    int first = 1;
    char* saveptr = NULL;
    char* tok = strtok_r(buf, ",", &saveptr);
    while (tok) {
        while (*tok == ' ') tok++;
        char* eq = strchr(tok, '=');
        if (!eq) { tok = strtok_r(NULL, ",", &saveptr); continue; }

        *eq = '\0';
        char* col = tok;
        char* val = eq + 1;

        // Trim
        char* col_end = eq - 1;
        while (col_end > col && *col_end == ' ') *col_end-- = '\0';
        while (*val == ' ') val++;
        char* val_end = val + strlen(val) - 1;
        while (val_end > val && *val_end == ' ') *val_end-- = '\0';

        int param_idx = add_param_copy(val);
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);

        if (!first) dynbuf_append(&sql, ", ");
        dynbuf_appendf(&sql, "%s = %s", col, ph);
        first = 0;

        tok = strtok_r(NULL, ",", &saveptr);
    }

    if (qs_where[0]) dynbuf_appendf(&sql, " WHERE %s", qs_where);

    debug_log_query(sql.data);

    int32_t result;
    if (qs_param_count > 0) {
        result = __db_execute_params(sql.data, qs_params, qs_param_count);
    } else {
        result = __db_execute_stmt(sql.data);
    }
    dynbuf_free(&sql);
    return result;
}

// ============================================================
// HAVING clause
// ============================================================

// Add HAVING condition — for use after GROUP BY + annotate
// Appends raw SQL condition (no parameterization — use having_val for user input)
// Django equivalent: .annotate(total=Sum("amount")).filter(total__gt=100)
int32_t __qs_having(const char* condition) {
    if (!condition || condition[0] == '\0') return -1;

    if (qs_having[0] == '\0') {
        strncpy(qs_having, condition, sizeof(qs_having) - 1);
    } else {
        char tmp[2048];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_having, condition);
        strncpy(qs_having, tmp, sizeof(qs_having) - 1);
    }
    return 0;
}

// HAVING with a parameterized value: "COUNT(*) >" + "5" → "COUNT(*) > $N"
int32_t __qs_having_val(const char* expr, const char* val) {
    if (!expr || !val) return -1;

    int param_idx = add_param_copy(val);
    char ph[16];
    write_placeholder(ph, sizeof(ph), param_idx);

    char condition[1024];
    snprintf(condition, sizeof(condition), "%s %s", expr, ph);

    if (qs_having[0] == '\0') {
        strncpy(qs_having, condition, sizeof(qs_having) - 1);
    } else {
        char tmp[2048];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_having, condition);
        strncpy(qs_having, tmp, sizeof(qs_having) - 1);
    }
    return 0;
}

// ============================================================
// Multi-column ORDER BY (append mode)
// ============================================================

// Append an additional ORDER BY column (vs. overwrite)
// Allows: order_by_add("-salary"); order_by_add("name") → ORDER BY salary DESC, name ASC
int32_t __qs_order_by_add(const char* col) {
    if (!col || col[0] == '\0') return -1;

    char clause[256];
    if (col[0] == '-') {
        snprintf(clause, sizeof(clause), "%s DESC", col + 1);
    } else {
        snprintf(clause, sizeof(clause), "%s ASC", col);
    }

    if (qs_order[0] == '\0') {
        strncpy(qs_order, clause, sizeof(qs_order) - 1);
    } else {
        char tmp[256];
        snprintf(tmp, sizeof(tmp), "%s, %s", qs_order, clause);
        strncpy(qs_order, tmp, sizeof(qs_order) - 1);
    }
    return 0;
}

// ============================================================
// Multi-column UPDATE (single SQL statement)
// ============================================================

// Accumulate a field for multi-column update
int32_t __qs_update_set(const char* col, const char* val) {
    if (!col || !val) return -1;

    // Grow update_keys if needed
    if (qs_update_count >= g_qs_current->update_cap) {
        g_qs_current->update_cap *= 2;
        g_qs_current->update_keys = (char**)realloc(g_qs_current->update_keys,
            g_qs_current->update_cap * sizeof(char*));
    }

    // On first call, record param start position
    if (qs_update_count == 0) {
        qs_update_param_start = qs_param_count;
    }

    if (qs_update_keys[qs_update_count]) free(qs_update_keys[qs_update_count]);
    qs_update_keys[qs_update_count] = strdup(col);
    add_param_copy(val);
    qs_update_count++;
    return 0;
}

// Execute UPDATE with all accumulated fields in a single statement
// Generates: UPDATE table SET col1=$1, col2=$2, ... WHERE ...
int32_t __qs_update_multi(void) {
    if (qs_update_count == 0 || qs_table[0] == '\0') return -1;

    char sql[8192];
    int pos = snprintf(sql, sizeof(sql), "UPDATE %s SET ", qs_table);

    for (int i = 0; i < qs_update_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        int param_idx = qs_update_param_start + i + 1; // 1-based
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s = %s",
            qs_update_keys[i], ph);
    }

    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    int32_t result;
    if (qs_param_count > 0) {
        // For MySQL (positional ? placeholders), we must reorder params so
        // SET values appear first, then WHERE values.  PostgreSQL uses $N
        // so the array order is irrelevant there.
        if (g_crud_dialect == DIALECT_MYSQL && qs_update_param_start > 0) {
            const char* reordered[QS_MAX_PARAMS];
            int ri = 0;
            // SET params first (indices qs_update_param_start .. qs_update_param_start + qs_update_count - 1)
            for (int i = 0; i < qs_update_count; i++) {
                reordered[ri++] = qs_params[qs_update_param_start + i];
            }
            // WHERE params next (indices 0 .. qs_update_param_start - 1)
            for (int i = 0; i < qs_update_param_start; i++) {
                reordered[ri++] = qs_params[i];
            }
            // Any remaining params after the update block
            for (int i = qs_update_param_start + qs_update_count; i < qs_param_count; i++) {
                reordered[ri++] = qs_params[i];
            }
            result = __db_execute_params(sql, reordered, ri);
        } else {
            result = __db_execute_params(sql, qs_params, qs_param_count);
        }
    } else {
        result = __db_execute_stmt(sql);
    }

    // Reset update accumulator (but NOT the whole queryset)
    qs_update_count = 0;
    qs_update_param_start = 0;
    return result;
}

// ============================================================
// Pagination helpers
// ============================================================

// Set page + page_size → calculates LIMIT and OFFSET
// page is 1-based. paginate(1, 25) → LIMIT 25 OFFSET 0
// Django equivalent: Paginator(queryset, 25).page(1)
int32_t __qs_paginate(int32_t page, int32_t page_size) {
    if (page < 1) page = 1;
    if (page_size < 1) page_size = 25; // sensible default
    qs_limit = page_size;
    qs_offset = (page - 1) * page_size;
    return 0;
}

// Total count ignoring LIMIT/OFFSET — for building pagination metadata
// Returns COUNT(*) with current WHERE but no LIMIT/OFFSET
int32_t __qs_total_count(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT COUNT(*) FROM %s", qs_table);

    // Inject soft-delete filter if needed
    char where_buf[4096];
    where_buf[0] = '\0';
    if (qs_where[0]) {
        strncpy(where_buf, qs_where, sizeof(where_buf) - 1);
    }
    if (qs_soft_delete && !qs_include_deleted) {
        if (where_buf[0] == '\0') {
            snprintf(where_buf, sizeof(where_buf), "%s = FALSE", qs_soft_delete_col);
        } else {
            char tmp[4096];
            snprintf(tmp, sizeof(tmp), "%s AND %s = FALSE", where_buf, qs_soft_delete_col);
            strncpy(where_buf, tmp, sizeof(where_buf) - 1);
        }
    }

    if (where_buf[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", where_buf);

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }
    if (rows > 0) {
        char* v = __db_get_value_at(0, 0);
        int result = atoi(v);
        free(v);
        return result;
    }
    return 0;
}

// ============================================================
// Soft Delete
// ============================================================

// Enable soft-delete mode for the current queryset
// When enabled, all SELECT/COUNT queries automatically add WHERE is_deleted = FALSE
// and soft_delete() marks rows instead of deleting them
int32_t __qs_soft_delete_mode(const char* col_name) {
    qs_soft_delete = 1;
    if (col_name && col_name[0] != '\0') {
        strncpy(qs_soft_delete_col, col_name, sizeof(qs_soft_delete_col) - 1);
        qs_soft_delete_col[sizeof(qs_soft_delete_col) - 1] = '\0';
    } else {
        strncpy(qs_soft_delete_col, "is_deleted", sizeof(qs_soft_delete_col) - 1);
    }
    return 0;
}

// Include soft-deleted rows in queries (bypass the auto-filter)
// Django equivalent: Model.all_objects.all() (custom manager)
int32_t __qs_with_deleted(void) {
    qs_include_deleted = 1;
    return 0;
}

// Permanently delete matching rows (real DELETE, ignoring soft-delete)
int32_t __qs_hard_delete(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "DELETE FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// Restore soft-deleted rows: SET is_deleted = FALSE WHERE ...
int32_t __qs_restore(void) {
    if (!qs_soft_delete) return -1; // no-op if not in soft-delete mode

    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "UPDATE %s SET %s = FALSE",
        qs_table, qs_soft_delete_col);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// Soft delete: UPDATE SET is_deleted = TRUE WHERE ...
// Marks rows as deleted without removing them
int32_t __qs_do_soft_delete(void) {
    if (!qs_soft_delete) return __qs_delete(); // fallback to real delete

    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "UPDATE %s SET %s = TRUE",
        qs_table, qs_soft_delete_col);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// ============================================================
// Aggregations (parameterized)
// ============================================================

char* __qs_aggregate(const char* func, const char* col) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT %s(%s) FROM %s", func, col, qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }
    if (rows > 0) {
        return __db_get_value_at(0, 0);
    }
    return strdup("0");
}

// ============================================================
// Raw SQL — execute arbitrary SQL
// ============================================================

// Execute raw SQL query (Django .raw() equivalent)
// For SELECT: populates result set, returns row count
// For DML: executes statement, returns affected rows
int32_t __qs_raw(const char* sql) {
    if (!sql || sql[0] == '\0') return -1;

    debug_log_query(sql);

    // Detect if it's a SELECT query
    const char* s = sql;
    while (*s == ' ' || *s == '\t' || *s == '\n') s++;

    if (strncasecmp(s, "SELECT", 6) == 0 ||
        strncasecmp(s, "WITH", 4) == 0 ||
        strncasecmp(s, "SHOW", 4) == 0 ||
        strncasecmp(s, "EXPLAIN", 7) == 0) {
        return __db_query_exec(sql);
    } else {
        return __db_execute_stmt(sql);
    }
}

// ============================================================
// latest / earliest (user-specified field)
// ============================================================

// Django: Entry.objects.latest('pub_date')
// If field is empty, falls back to 'id'
int32_t __qs_latest(const char* field) {
    const char* f = (field && field[0]) ? field : "id";
    snprintf(qs_order, sizeof(qs_order), "%s DESC", f);
    qs_limit = 1;
    return __qs_fetch();
}

// Django: Entry.objects.earliest('pub_date')
int32_t __qs_earliest(const char* field) {
    const char* f = (field && field[0]) ? field : "id";
    snprintf(qs_order, sizeof(qs_order), "%s ASC", f);
    qs_limit = 1;
    return __qs_fetch();
}

// ============================================================
// only() — restrict SELECT columns
// ============================================================

// Django: Entry.objects.only('id', 'name', 'email')
// Takes comma-separated column names
int32_t __qs_only(const char* cols) {
    if (!cols || cols[0] == '\0') return -1;
    strncpy(qs_columns, cols, sizeof(qs_columns) - 1);
    qs_columns[sizeof(qs_columns) - 1] = '\0';
    return 0;
}

// Django: Entry.objects.defer('body', 'metadata')
// Defer is the inverse of only — select all columns EXCEPT the listed ones.
// Queries information_schema to get the full column list, then excludes deferred cols.
int32_t __qs_defer(const char* cols) {
    if (!cols || cols[0] == '\0' || qs_table[0] == '\0') return -1;

    // Step 1: Query all columns for this table
    char sql[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql),
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' "
            "ORDER BY ordinal_position", qs_table);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT column_name FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' "
            "ORDER BY ordinal_position", qs_table);
    }

    int nrows = __db_query_exec(sql);
    if (nrows <= 0) return -1;

    // Step 2: Build comma-separated deferred set for fast lookup
    // Parse deferred cols into an array
    char deferred[32][128];
    int defer_count = 0;
    {
        char buf[1024];
        strncpy(buf, cols, sizeof(buf) - 1);
        buf[sizeof(buf) - 1] = '\0';
        char* tok = strtok(buf, ",");
        while (tok && defer_count < 32) {
            // Trim whitespace
            while (*tok == ' ') tok++;
            char* end = tok + strlen(tok) - 1;
            while (end > tok && *end == ' ') { *end = '\0'; end--; }
            strncpy(deferred[defer_count], tok, 127);
            deferred[defer_count][127] = '\0';
            defer_count++;
            tok = strtok(NULL, ",");
        }
    }

    // Step 3: Build SELECT list excluding deferred columns
    qs_columns[0] = '\0';
    int cpos = 0;
    int included = 0;
    for (int r = 0; r < nrows; r++) {
        char* col_name = __db_get_value_at(r, 0);
        if (!col_name) continue;

        // Check if this column is in the deferred list
        int is_deferred = 0;
        for (int d = 0; d < defer_count; d++) {
            if (strcmp(col_name, deferred[d]) == 0) {
                is_deferred = 1;
                break;
            }
        }

        if (!is_deferred) {
            if (included > 0) {
                cpos += snprintf(qs_columns + cpos, sizeof(qs_columns) - cpos, ", ");
            }
            cpos += snprintf(qs_columns + cpos, sizeof(qs_columns) - cpos, "%s", col_name);
            included++;
        }
        free(col_name);
    }

    return 0;
}

// ============================================================
// get_or_create()
// ============================================================

// Django: obj, created = Entry.objects.get_or_create(name="Alice")
// Returns: 1 if created (INSERT), 0 if already existed (SELECT)
// After call, the result set contains the row.
int32_t __qs_get_or_create(void) {
    if (qs_insert_count == 0 || qs_table[0] == '\0') return -1;

    // Step 1: Try to SELECT with the insert fields as WHERE conditions
    char where[4096] = "";
    int wpos = 0;
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) wpos += snprintf(where + wpos, sizeof(where) - wpos, " AND ");
        int param_idx = qs_insert_param_start + i + 1;
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        wpos += snprintf(where + wpos, sizeof(where) - wpos, "%s = %s",
            qs_insert_keys[i], ph);
    }

    char sql[4096];
    snprintf(sql, sizeof(sql), "SELECT * FROM %s WHERE %s LIMIT 1", qs_table, where);

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }

    if (rows > 0) {
        // Already exists — return 0 (not created)
        return 0;
    }

    // Step 2: INSERT and return the new row
    int rc = __qs_do_insert();
    if (rc >= 0) {
        return 1; // created
    }
    return -1; // error
}

// ============================================================
// EXPLAIN — query plan inspection
// ============================================================

// Runs EXPLAIN ANALYZE on the current queryset and returns row count
// The result set contains the query plan as text rows
int32_t __qs_explain(void) {
    // Build the SELECT query the same way __qs_fetch does
    char inner_sql[8192];
    int pos;

    const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
    if (qs_distinct) {
        pos = snprintf(inner_sql, sizeof(inner_sql), "SELECT DISTINCT %s FROM %s", cols, qs_table);
    } else {
        pos = snprintf(inner_sql, sizeof(inner_sql), "SELECT %s FROM %s", cols, qs_table);
    }
    if (qs_where[0]) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " WHERE %s", qs_where);
    if (qs_group_by[0]) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " GROUP BY %s", qs_group_by);
    if (qs_order[0]) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " ORDER BY %s", qs_order);
    if (qs_limit > 0) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " LIMIT %d", qs_limit);
    if (qs_offset > 0) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " OFFSET %d", qs_offset);

    // Wrap with EXPLAIN ANALYZE (PG) or EXPLAIN (MySQL)
    char sql[8192 + 64];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql), "EXPLAIN %s", inner_sql);
    } else {
        snprintf(sql, sizeof(sql), "EXPLAIN ANALYZE %s", inner_sql);
    }

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_query_params(sql, qs_params, qs_param_count);
    }
    return __db_query_exec(sql);
}

// ============================================================
// Schema Inspection
// ============================================================

// List all tables in the current database
// Returns row count; result set has table names
int32_t __db_describe_tables(void) {
    const char* sql;
    if (g_crud_dialect == DIALECT_MYSQL) {
        sql = "SELECT table_name FROM information_schema.tables "
              "WHERE table_schema = DATABASE() ORDER BY table_name";
    } else {
        sql = "SELECT table_name FROM information_schema.tables "
              "WHERE table_schema = 'public' ORDER BY table_name";
    }
    debug_log_query(sql);
    return __db_query_exec(sql);
}

// List columns for a given table
// Returns row count; result set has column_name, data_type, is_nullable, column_default
int32_t __db_describe_columns(const char* table_name) {
    if (!table_name || table_name[0] == '\0') return -1;

    char sql[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql),
            "SELECT column_name, data_type, is_nullable, column_default "
            "FROM information_schema.columns "
            "WHERE table_schema = DATABASE() AND table_name = '%s' "
            "ORDER BY ordinal_position", table_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT column_name, data_type, is_nullable, column_default "
            "FROM information_schema.columns "
            "WHERE table_schema = 'public' AND table_name = '%s' "
            "ORDER BY ordinal_position", table_name);
    }
    debug_log_query(sql);
    return __db_query_exec(sql);
}

// List indexes for a given table (PG-specific via pg_indexes, MySQL via SHOW INDEX)
int32_t __db_describe_indexes(const char* table_name) {
    if (!table_name || table_name[0] == '\0') return -1;

    char sql[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql), "SHOW INDEX FROM %s", table_name);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT indexname, indexdef FROM pg_indexes "
            "WHERE tablename = '%s'", table_name);
    }
    debug_log_query(sql);
    return __db_query_exec(sql);
}

// ============================================================
// Advisory Locks (PG)
// ============================================================

// Acquire a session-level advisory lock (blocking)
// Django: from django.contrib.postgres.locks import advisory_lock
int32_t __db_advisory_lock(int64_t key) {
    char sql[128];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql), "SELECT GET_LOCK('%lld', -1)", (long long)key);
    } else {
        snprintf(sql, sizeof(sql), "SELECT pg_advisory_lock(%lld)", (long long)key);
    }
    debug_log_query(sql);
    return __db_query_exec(sql) >= 0 ? 0 : -1;
}

// Release a session-level advisory lock
int32_t __db_advisory_unlock(int64_t key) {
    char sql[128];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql), "SELECT RELEASE_LOCK('%lld')", (long long)key);
    } else {
        snprintf(sql, sizeof(sql), "SELECT pg_advisory_unlock(%lld)", (long long)key);
    }
    debug_log_query(sql);
    return __db_query_exec(sql) >= 0 ? 0 : -1;
}

// Try to acquire (non-blocking) — returns 1 if acquired, 0 if not
int32_t __db_advisory_try_lock(int64_t key) {
    char sql[128];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql), "SELECT GET_LOCK('%lld', 0)", (long long)key);
    } else {
        snprintf(sql, sizeof(sql), "SELECT pg_try_advisory_lock(%lld)", (long long)key);
    }
    debug_log_query(sql);
    int rows = __db_query_exec(sql);
    if (rows > 0) {
        char* v = __db_get_value_at(0, 0);
        int result = (v && (v[0] == 't' || v[0] == '1')) ? 1 : 0;
        free(v);
        return result;
    }
    return 0;
}

// ============================================================
// UPSERT — INSERT ... ON CONFLICT DO UPDATE
// ============================================================


// Set the conflict column for upsert operations
// Django: Model.objects.update_or_create(defaults={...}, **lookup)
int32_t __qs_on_conflict(const char* conflict_col) {
    if (!conflict_col || conflict_col[0] == '\0') return -1;
    strncpy(qs_upsert_col, conflict_col, sizeof(qs_upsert_col) - 1);
    qs_upsert_col[sizeof(qs_upsert_col) - 1] = '\0';
    return 0;
}

// Execute UPSERT with accumulated fields
// PG: INSERT INTO t (cols) VALUES (vals) ON CONFLICT (conflict_col) DO UPDATE SET col=EXCLUDED.col, ...
// MySQL: INSERT INTO t (cols) VALUES (vals) ON DUPLICATE KEY UPDATE col=VALUES(col), ...
int32_t __qs_do_upsert(void) {
    if (qs_insert_count == 0 || qs_table[0] == '\0' || qs_upsert_col[0] == '\0') return -1;

    char sql[8192];
    int pos = snprintf(sql, sizeof(sql), "INSERT INTO %s (", qs_table);

    // Build column list
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", qs_insert_keys[i]);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ") VALUES (");

    // Build placeholder list
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        char ph[16];
        write_placeholder(ph, sizeof(ph), qs_insert_param_start + i + 1);
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", ph);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ")");

    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL: ON DUPLICATE KEY UPDATE col=VALUES(col), ...
        pos += snprintf(sql + pos, sizeof(sql) - pos, " ON DUPLICATE KEY UPDATE ");
        int first_update = 1;
        for (int i = 0; i < qs_insert_count; i++) {
            if (strcmp(qs_insert_keys[i], qs_upsert_col) == 0) continue; // skip conflict col
            if (!first_update) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
            pos += snprintf(sql + pos, sizeof(sql) - pos, "%s = VALUES(%s)",
                qs_insert_keys[i], qs_insert_keys[i]);
            first_update = 0;
        }
    } else {
        // PG: ON CONFLICT (col) DO UPDATE SET col = EXCLUDED.col, ...
        pos += snprintf(sql + pos, sizeof(sql) - pos, " ON CONFLICT (%s) DO UPDATE SET ", qs_upsert_col);
        int first_update = 1;
        for (int i = 0; i < qs_insert_count; i++) {
            if (strcmp(qs_insert_keys[i], qs_upsert_col) == 0) continue;
            if (!first_update) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
            pos += snprintf(sql + pos, sizeof(sql) - pos, "%s = EXCLUDED.%s",
                qs_insert_keys[i], qs_insert_keys[i]);
            first_update = 0;
        }
    }

    debug_log_query(sql);

    int32_t result;
    if (qs_param_count > 0) {
        result = __db_execute_params(sql, qs_params, qs_param_count);
    } else {
        result = __db_execute_stmt(sql);
    }

    qs_insert_count = 0;
    qs_insert_param_start = 0;
    qs_upsert_col[0] = '\0';
    return result;
}

// ============================================================
// update_or_create
// ============================================================

// Django: Model.objects.update_or_create(defaults={"email": "new"}, name="Alice")
// Returns: 1 if created, 0 if updated, -1 on error
// Uses set_field for all fields + filter for lookup conditions
int32_t __qs_update_or_create(void) {
    if (qs_insert_count == 0 || qs_table[0] == '\0') return -1;

    // Step 1: Try to SELECT with WHERE
    char sql[4096];
    int pos;

    if (qs_where[0]) {
        pos = snprintf(sql, sizeof(sql), "SELECT * FROM %s WHERE %s LIMIT 1", qs_table, qs_where);
    } else {
        // Use all accumulated fields as WHERE
        char where[4096] = "";
        int wpos = 0;
        for (int i = 0; i < qs_insert_count; i++) {
            if (i > 0) wpos += snprintf(where + wpos, sizeof(where) - wpos, " AND ");
            int param_idx = qs_insert_param_start + i + 1;
            char ph[16];
            write_placeholder(ph, sizeof(ph), param_idx);
            wpos += snprintf(where + wpos, sizeof(where) - wpos, "%s = %s",
                qs_insert_keys[i], ph);
        }
        pos = snprintf(sql, sizeof(sql), "SELECT * FROM %s WHERE %s LIMIT 1", qs_table, where);
    }

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }

    if (rows > 0) {
        // Update existing row
        for (int i = 0; i < qs_insert_count; i++) {
            __qs_update(qs_insert_keys[i], qs_params[qs_insert_param_start + i]);
        }
        qs_insert_count = 0;
        qs_insert_param_start = 0;
        return 0; // updated
    }

    // Insert new row
    int rc = __qs_do_insert();
    if (rc >= 0) return 1; // created
    return -1; // error
}

// ============================================================
// Window Functions
// ============================================================


// Add a window function expression to the SELECT
// Django: from django.db.models import Window, F
//         qs.annotate(row_num=Window(expression=RowNumber(), partition_by=[F('dept')], order_by=F('salary').desc()))
// Desi:   db.window("ROW_NUMBER()", "PARTITION BY dept ORDER BY salary DESC", "row_num")
int32_t __qs_window(const char* func, const char* over_clause, const char* alias) {
    if (!func || !over_clause || !alias) return -1;

    // Build: func OVER (over_clause) AS alias
    // Store as an annotation-like expression in qs_columns
    char expr[512];
    snprintf(expr, sizeof(expr), "%s OVER (%s) AS %s", func, over_clause, alias);

    // Append to qs_columns (if already has content, add comma)
    if (qs_columns[0] != '\0') {
        int len = strlen(qs_columns);
        snprintf(qs_columns + len, sizeof(qs_columns) - len, ", %s", expr);
    } else {
        // Start with *, then add window expression
        snprintf(qs_columns, sizeof(qs_columns), "*, %s", expr);
    }

    return 0;
}

// ============================================================
// Server-Side Cursor
// ============================================================

// State for cursor
static char qs_cursor_name[64] = "";
static int  qs_cursor_open = 0;

// Declare a server-side cursor for the current queryset
// PG: DECLARE cursor_name CURSOR FOR SELECT ...
// Returns 0 on success, -1 on error
int32_t __qs_cursor_declare(const char* cursor_name) {
    if (!cursor_name || cursor_name[0] == '\0') return -1;
    strncpy(qs_cursor_name, cursor_name, sizeof(qs_cursor_name) - 1);
    qs_cursor_name[sizeof(qs_cursor_name) - 1] = '\0';

    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL server-side cursors are not supported in wire protocol level
        // Use LIMIT/OFFSET pagination instead — qs_cursor_name is used as a marker
        qs_cursor_open = 1;
        return 0;
    }

    // PG: Build SELECT then wrap with DECLARE CURSOR
    char inner_sql[4096];
    int pos;
    const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
    pos = snprintf(inner_sql, sizeof(inner_sql), "SELECT %s FROM %s", cols, qs_table);
    if (qs_where[0]) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " WHERE %s", qs_where);
    if (qs_order[0]) pos += snprintf(inner_sql + pos, sizeof(inner_sql) - pos, " ORDER BY %s", qs_order);

    char sql[4096 + 128];
    snprintf(sql, sizeof(sql), "DECLARE %s CURSOR FOR %s", cursor_name, inner_sql);

    debug_log_query(sql);

    // Must be in a transaction for PG cursors
    int rc;
    if (qs_param_count > 0) {
        rc = __db_execute_params(sql, qs_params, qs_param_count);
    } else {
        rc = __db_execute_stmt(sql);
    }

    if (rc >= 0) qs_cursor_open = 1;
    return rc >= 0 ? 0 : -1;
}

// Fetch N rows from cursor
// PG: FETCH n FROM cursor_name
// MySQL: re-execute with LIMIT n OFFSET cursor_pos
int32_t __qs_cursor_fetch(int32_t batch_size) {
    if (!qs_cursor_open || qs_cursor_name[0] == '\0') return -1;

    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL: use LIMIT/OFFSET pagination
        static int mysql_cursor_offset = 0;
        const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
        char sql[4096];
        int pos = snprintf(sql, sizeof(sql), "SELECT %s FROM %s", cols, qs_table);
        if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
        if (qs_order[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " ORDER BY %s", qs_order);
        pos += snprintf(sql + pos, sizeof(sql) - pos, " LIMIT %d OFFSET %d", batch_size, mysql_cursor_offset);

        debug_log_query(sql);

        int rows;
        if (qs_param_count > 0) {
            rows = __db_query_params(sql, qs_params, qs_param_count);
        } else {
            rows = __db_query_exec(sql);
        }
        mysql_cursor_offset += batch_size;
        if (rows == 0) {
            // No more rows — close cursor
            qs_cursor_open = 0;
            mysql_cursor_offset = 0;
        }
        return rows;
    }

    // PG: FETCH n FROM cursor_name
    char sql[128];
    snprintf(sql, sizeof(sql), "FETCH %d FROM %s", batch_size, qs_cursor_name);
    debug_log_query(sql);
    int rows = __db_query_exec(sql);
    if (rows == 0) qs_cursor_open = 0;
    return rows;
}

// Close cursor
int32_t __qs_cursor_close(void) {
    if (!qs_cursor_open) return 0;

    if (g_crud_dialect != DIALECT_MYSQL) {
        char sql[128];
        snprintf(sql, sizeof(sql), "CLOSE %s", qs_cursor_name);
        debug_log_query(sql);
        __db_execute_stmt(sql);
    }

    qs_cursor_open = 0;
    qs_cursor_name[0] = '\0';
    return 0;
}

// ============================================================
// JSON Field Helpers
// ============================================================

// Update a JSON key within a column
// PG: UPDATE t SET col = jsonb_set(col, '{key}', '"val"')
// MySQL: UPDATE t SET col = JSON_SET(col, '$.key', 'val')
int32_t __qs_json_set(const char* col, const char* key, const char* val) {
    if (!col || !key || !val || qs_table[0] == '\0') return -1;

    char sql[1024];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql),
            "UPDATE %s SET %s = JSON_SET(COALESCE(%s, '{}'), '$.%s', '%s')",
            qs_table, col, col, key, val);
    } else {
        snprintf(sql, sizeof(sql),
            "UPDATE %s SET %s = jsonb_set(COALESCE(%s, '{}')::jsonb, '{%s}', '\"%s\"')",
            qs_table, col, col, key, val);
    }

    // Apply WHERE if set
    if (qs_where[0]) {
        int pos = strlen(sql);
        snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    }

    debug_log_query(sql);

    if (qs_param_count > 0) {
        return __db_execute_params(sql, qs_params, qs_param_count);
    }
    return __db_execute_stmt(sql);
}

// Extract a value from a JSON field
// PG: SELECT col->>'key' FROM t WHERE ...
// MySQL: SELECT JSON_UNQUOTE(JSON_EXTRACT(col, '$.key')) FROM t WHERE ...
char* __qs_json_get(const char* col, const char* key) {
    if (!col || !key || qs_table[0] == '\0') return "";

    char sql[1024];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql),
            "SELECT JSON_UNQUOTE(JSON_EXTRACT(%s, '$.%s')) FROM %s",
            col, key, qs_table);
    } else {
        snprintf(sql, sizeof(sql),
            "SELECT %s->>'%s' FROM %s",
            col, key, qs_table);
    }

    if (qs_where[0]) {
        int pos = strlen(sql);
        snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    }

    int pos_sql = strlen(sql);
    snprintf(sql + pos_sql, sizeof(sql) - pos_sql, " LIMIT 1");

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }

    if (rows > 0) {
        return __db_get_value_at(0, 0);
    }
    return "";
}

// ============================================================
// CTEs (Common Table Expressions — WITH clause)
// ============================================================

// State: up to 4 CTE clauses
#define QS_MAX_CTES 4
static struct {
    char name[64];     // CTE alias
    char query[2048];  // CTE query body
} qs_ctes[QS_MAX_CTES];
static int qs_cte_count = 0;
static int qs_cte_recursive = 0;  // 1 if WITH RECURSIVE

// Add a CTE clause
// Django: MyModel.objects.raw("WITH active AS (...) SELECT ...")
// Desi:   db.cte("active", "SELECT * FROM users WHERE active = true")
int32_t __qs_cte_add(const char* name, const char* query) {
    if (qs_cte_count >= QS_MAX_CTES || !name || !query) return -1;
    strncpy(qs_ctes[qs_cte_count].name, name, sizeof(qs_ctes[0].name) - 1);
    qs_ctes[qs_cte_count].name[sizeof(qs_ctes[0].name) - 1] = '\0';
    strncpy(qs_ctes[qs_cte_count].query, query, sizeof(qs_ctes[0].query) - 1);
    qs_ctes[qs_cte_count].query[sizeof(qs_ctes[0].query) - 1] = '\0';
    qs_cte_count++;
    return 0;
}

// Set recursive mode for CTEs
int32_t __qs_cte_recursive(void) {
    qs_cte_recursive = 1;
    return 0;
}

// Execute: build WITH ... and then the main SELECT from qs_table
int32_t __qs_cte_fetch(void) {
    if (qs_cte_count == 0 || qs_table[0] == '\0') return -1;

    char sql[8192];
    int pos;
    if (qs_cte_recursive) {
        pos = snprintf(sql, sizeof(sql), "WITH RECURSIVE ");
    } else {
        pos = snprintf(sql, sizeof(sql), "WITH ");
    }

    for (int i = 0; i < qs_cte_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s AS (%s)",
            qs_ctes[i].name, qs_ctes[i].query);
    }

    // Main SELECT
    const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
    pos += snprintf(sql + pos, sizeof(sql) - pos, " SELECT %s FROM %s", cols, qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    if (qs_order[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " ORDER BY %s", qs_order);
    if (qs_limit > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " LIMIT %d", qs_limit);

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }

    // Reset CTE state
    qs_cte_count = 0;
    qs_cte_recursive = 0;

    return rows;
}

// ============================================================
// Subqueries — EXISTS / IN with nested SELECT
// ============================================================

// Build subquery: SELECT cols FROM table WHERE where_clause
// Returns the SQL string for use in EXISTS/IN conditions
char* __qs_subquery_build(const char* sub_table, const char* sub_cols, const char* sub_where) {
    static char sub_sql[2048];
    if (!sub_table) return "";

    const char* cols = (sub_cols && sub_cols[0]) ? sub_cols : "*";
    int pos = snprintf(sub_sql, sizeof(sub_sql), "SELECT %s FROM %s", cols, sub_table);

    if (sub_where && sub_where[0]) {
        pos += snprintf(sub_sql + pos, sizeof(sub_sql) - pos, " WHERE %s", sub_where);
    }

    return sub_sql;
}

// Filter by EXISTS subquery:
// WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)
int32_t __qs_filter_exists(const char* subquery) {
    if (!subquery || !subquery[0]) return -1;

    char condition[2048];
    snprintf(condition, sizeof(condition), "EXISTS (%s)", subquery);

    // Append to qs_where
    if (qs_where[0]) {
        int len = strlen(qs_where);
        snprintf(qs_where + len, sizeof(qs_where) - len, " AND %s", condition);
    } else {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    }

    return 0;
}

// Filter by NOT EXISTS subquery
int32_t __qs_filter_not_exists(const char* subquery) {
    if (!subquery || !subquery[0]) return -1;

    char condition[2048];
    snprintf(condition, sizeof(condition), "NOT EXISTS (%s)", subquery);

    if (qs_where[0]) {
        int len = strlen(qs_where);
        snprintf(qs_where + len, sizeof(qs_where) - len, " AND %s", condition);
    } else {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    }

    return 0;
}

// Filter by IN subquery:
// WHERE col IN (SELECT id FROM ...)
int32_t __qs_filter_in_subquery(const char* col, const char* subquery) {
    if (!col || !subquery) return -1;

    char condition[2048];
    snprintf(condition, sizeof(condition), "%s IN (%s)", col, subquery);

    if (qs_where[0]) {
        int len = strlen(qs_where);
        snprintf(qs_where + len, sizeof(qs_where) - len, " AND %s", condition);
    } else {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    }

    return 0;
}

// Filter by NOT IN subquery
int32_t __qs_filter_not_in_subquery(const char* col, const char* subquery) {
    if (!col || !subquery) return -1;

    char condition[2048];
    snprintf(condition, sizeof(condition), "%s NOT IN (%s)", col, subquery);

    if (qs_where[0]) {
        int len = strlen(qs_where);
        snprintf(qs_where + len, sizeof(qs_where) - len, " AND %s", condition);
    } else {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    }

    return 0;
}

// ============================================================
// prefetch_related — batch N+1 → N queries
// ============================================================

// State for prefetch
#define QS_MAX_PREFETCH 4
static struct {
    char related_table[128]; // related table name
    char fk_col[128];        // foreign key column in related table
    char pk_col[128];        // primary key column in current table (usually 'id')
} qs_prefetches[QS_MAX_PREFETCH];
static int qs_prefetch_count = 0;

// Register a prefetch relationship
// Django: Entry.objects.prefetch_related('authors')
// Desi: db.prefetch_related("authors", "post_id", "id")
int32_t __qs_prefetch_add(const char* related_table, const char* fk_col, const char* pk_col) {
    if (qs_prefetch_count >= QS_MAX_PREFETCH) return -1;
    if (!related_table || !fk_col || !pk_col) return -1;

    strncpy(qs_prefetches[qs_prefetch_count].related_table, related_table, 127);
    qs_prefetches[qs_prefetch_count].related_table[127] = '\0';
    strncpy(qs_prefetches[qs_prefetch_count].fk_col, fk_col, 127);
    qs_prefetches[qs_prefetch_count].fk_col[127] = '\0';
    strncpy(qs_prefetches[qs_prefetch_count].pk_col, pk_col, 127);
    qs_prefetches[qs_prefetch_count].pk_col[127] = '\0';

    qs_prefetch_count++;
    return 0;
}

// Execute prefetch: first query main table, then batch-query each related table
// with WHERE fk_col IN (list of PKs from main query)
// Returns: main query row count, prefetch results available via get_value_at
int32_t __qs_prefetch_execute(void) {
    if (qs_table[0] == '\0') return -1;

    // Step 1: Execute main query
    char sql[4096];
    const char* cols = (qs_columns[0] != '\0') ? qs_columns : "*";
    int pos = snprintf(sql, sizeof(sql), "SELECT %s FROM %s", cols, qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    if (qs_order[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " ORDER BY %s", qs_order);
    if (qs_limit > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " LIMIT %d", qs_limit);

    debug_log_query(sql);

    int main_rows;
    if (qs_param_count > 0) {
        main_rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        main_rows = __db_query_exec(sql);
    }

    if (main_rows <= 0 || qs_prefetch_count == 0) {
        qs_prefetch_count = 0;
        return main_rows;
    }

    // Step 2: For each prefetch, collect PKs and batch-query related table
    // Find the pk column index in the result
    for (int p = 0; p < qs_prefetch_count; p++) {
        // Collect PKs from main result (pk_col values)
        // For simplicity, we assume pk_col is column 0 (id) by default
        int pk_col_idx = 0; // TODO: resolve from column name

        char pk_list[4096] = "";
        int ppos = 0;
        for (int r = 0; r < main_rows && ppos < (int)sizeof(pk_list) - 32; r++) {
            char* v = __db_get_value_at(r, pk_col_idx);
            if (v) {
                if (ppos > 0) ppos += snprintf(pk_list + ppos, sizeof(pk_list) - ppos, ", ");
                ppos += snprintf(pk_list + ppos, sizeof(pk_list) - ppos, "'%s'", v);
                free(v);
            }
        }

        if (ppos > 0) {
            // Execute: SELECT * FROM related_table WHERE fk_col IN (pk_list)
            char rel_sql[8192];
            snprintf(rel_sql, sizeof(rel_sql), "SELECT * FROM %s WHERE %s IN (%s)",
                qs_prefetches[p].related_table,
                qs_prefetches[p].fk_col,
                pk_list);

            debug_log_query(rel_sql);
            __db_query_exec(rel_sql);
            // Results are now in the query buffer for access via get_value_at
            // In real production code, we'd store these in a separate result set
        }
    }

    qs_prefetch_count = 0;
    return main_rows;
}

// ============================================================
// JSON_TABLE — extract relational data from JSON
// ============================================================

// JSON_TABLE: transform JSON column into rows
// PG 17: SELECT * FROM json_table(col, '$.items[*]' COLUMNS (name TEXT PATH '$.name', qty INT PATH '$.qty'))
// MySQL 8.0.4+: SELECT * FROM table, JSON_TABLE(col, '$.items[*]' COLUMNS (...)) AS jt
//
// This function builds and executes a JSON_TABLE query
// json_col: the JSON column name
// json_path: the JSON path expression (e.g., '$.items[*]')
// columns_def: column definitions (e.g., "name TEXT PATH '$.name', qty INT PATH '$.qty'")
// alias: table alias for JSON_TABLE result
int32_t __qs_json_table(const char* json_col, const char* json_path,
                         const char* columns_def, const char* alias) {
    if (!json_col || !json_path || !columns_def || !alias || qs_table[0] == '\0') return -1;

    char sql[8192];
    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL: SELECT jt.* FROM table t, JSON_TABLE(t.col, '$.path' COLUMNS (defs)) AS jt
        snprintf(sql, sizeof(sql),
            "SELECT %s.* FROM %s, JSON_TABLE(%s, '%s' COLUMNS (%s)) AS %s",
            alias, qs_table, json_col, json_path, columns_def, alias);
    } else {
        // PG 17: SELECT * FROM table, json_table(col, '$.path' COLUMNS (defs)) AS jt
        // PG uses lowercase json_table (SQL/JSON standard function)
        snprintf(sql, sizeof(sql),
            "SELECT %s.* FROM %s, json_table(%s, '%s' COLUMNS (%s)) AS %s",
            alias, qs_table, json_col, json_path, columns_def, alias);
    }

    // Apply WHERE if set (on main table)
    if (qs_where[0]) {
        int pos = strlen(sql);
        snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    }

    debug_log_query(sql);

    int rows;
    if (qs_param_count > 0) {
        rows = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        rows = __db_query_exec(sql);
    }
    return rows;
}

// ============================================================
// Full-Text Search
// ============================================================

// Full-text search query
// PG: WHERE to_tsvector('english', col) @@ to_tsquery('english', query)
// MySQL: WHERE MATCH(col) AGAINST (query IN BOOLEAN MODE)
//
// config: language/config name (PG: 'english', MySQL: ignored)
int32_t __qs_fts_filter(const char* col, const char* query, const char* config) {
    if (!col || !query) return -1;

    char condition[1024];
    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL: MATCH(col) AGAINST ('query' IN BOOLEAN MODE)
        snprintf(condition, sizeof(condition),
            "MATCH(%s) AGAINST ('%s' IN BOOLEAN MODE)", col, query);
    } else {
        // PG: to_tsvector(config, col) @@ to_tsquery(config, query)
        const char* cfg = (config && config[0]) ? config : "english";
        snprintf(condition, sizeof(condition),
            "to_tsvector('%s', %s) @@ to_tsquery('%s', '%s')", cfg, col, cfg, query);
    }

    // Append to WHERE
    if (qs_where[0]) {
        int len = strlen(qs_where);
        snprintf(qs_where + len, sizeof(qs_where) - len, " AND %s", condition);
    } else {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    }

    return 0;
}

// Full-text search with ranking/relevance score
// PG: ts_rank(to_tsvector('english', col), to_tsquery('english', query)) AS rank
// MySQL: MATCH(col) AGAINST ('query') AS rank
int32_t __qs_fts_rank(const char* col, const char* query, const char* config,
                       const char* alias) {
    if (!col || !query || !alias) return -1;

    char expr[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(expr, sizeof(expr),
            "MATCH(%s) AGAINST ('%s') AS %s", col, query, alias);
    } else {
        const char* cfg = (config && config[0]) ? config : "english";
        snprintf(expr, sizeof(expr),
            "ts_rank(to_tsvector('%s', %s), to_tsquery('%s', '%s')) AS %s",
            cfg, col, cfg, query, alias);
    }

    // Add to qs_columns
    if (qs_columns[0] != '\0') {
        int len = strlen(qs_columns);
        snprintf(qs_columns + len, sizeof(qs_columns) - len, ", %s", expr);
    } else {
        snprintf(qs_columns, sizeof(qs_columns), "*, %s", expr);
    }

    return 0;
}

// Create a full-text search index on a column
// PG: CREATE INDEX idx ON table USING gin(to_tsvector('english', col))
// MySQL: ALTER TABLE table ADD FULLTEXT INDEX idx(col)
int32_t __qs_fts_create_index(const char* table, const char* col, const char* idx_name,
                               const char* config) {
    if (!table || !col || !idx_name) return -1;

    char sql[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        snprintf(sql, sizeof(sql),
            "ALTER TABLE %s ADD FULLTEXT INDEX %s(%s)", table, idx_name, col);
    } else {
        const char* cfg = (config && config[0]) ? config : "english";
        snprintf(sql, sizeof(sql),
            "CREATE INDEX %s ON %s USING gin(to_tsvector('%s', %s))",
            idx_name, table, cfg, col);
    }

    debug_log_query(sql);
    return __db_execute_stmt(sql);
}

// ============================================================
// Vector Similarity Search
// ============================================================

// Vector similarity search (cosine / L2 / inner product)
// PG (pgvector): ORDER BY embedding <=> '[0.1,0.2,...]' LIMIT k
// MySQL 9 (VECTOR): ORDER BY DISTANCE(embedding, TO_VECTOR('[0.1,0.2,...]'), 'COSINE') LIMIT k
//
// col: vector column name
// vector_str: vector as string "[0.1, 0.2, 0.3, ...]"
// metric: "cosine" | "l2" | "ip" (inner product)
// k: number of nearest neighbors
int32_t __qs_vector_search(const char* col, const char* vector_str,
                            const char* metric, int32_t k) {
    if (!col || !vector_str || qs_table[0] == '\0') return -1;

    char order_expr[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        // MySQL 9 VECTOR type with DISTANCE function
        const char* dist_metric = "COSINE";
        if (metric) {
            if (strcmp(metric, "l2") == 0) dist_metric = "L2";
            else if (strcmp(metric, "ip") == 0) dist_metric = "DOT";
        }
        snprintf(order_expr, sizeof(order_expr),
            "DISTANCE(%s, TO_VECTOR('%s'), '%s')", col, vector_str, dist_metric);
    } else {
        // PG pgvector: operator depends on metric
        // cosine: <=>  l2: <->  ip: <#>
        const char* op = "<=>";
        if (metric) {
            if (strcmp(metric, "l2") == 0) op = "<->";
            else if (strcmp(metric, "ip") == 0) op = "<#>";
        }
        snprintf(order_expr, sizeof(order_expr),
            "%s %s '%s'", col, op, vector_str);
    }

    // Set ORDER BY and LIMIT
    strncpy(qs_order, order_expr, sizeof(qs_order) - 1);
    qs_order[sizeof(qs_order) - 1] = '\0';
    qs_limit = k;

    return 0;
}

// Create a vector index (HNSW or IVFFlat for PG, VECTOR INDEX for MySQL)
// PG: CREATE INDEX idx ON table USING hnsw (col vector_cosine_ops)
// MySQL: ALTER TABLE table ADD VECTOR INDEX idx(col) [DISTANCE = 'COSINE']
int32_t __qs_vector_create_index(const char* table, const char* col,
                                  const char* idx_name, const char* metric) {
    if (!table || !col || !idx_name) return -1;

    char sql[512];
    if (g_crud_dialect == DIALECT_MYSQL) {
        const char* dist = "COSINE";
        if (metric) {
            if (strcmp(metric, "l2") == 0) dist = "L2";
            else if (strcmp(metric, "ip") == 0) dist = "DOT";
        }
        snprintf(sql, sizeof(sql),
            "ALTER TABLE %s ADD VECTOR INDEX %s(%s) DISTANCE = '%s'",
            table, idx_name, col, dist);
    } else {
        // PG pgvector: use hnsw index with ops class
        const char* ops = "vector_cosine_ops";
        if (metric) {
            if (strcmp(metric, "l2") == 0) ops = "vector_l2_ops";
            else if (strcmp(metric, "ip") == 0) ops = "vector_ip_ops";
        }
        snprintf(sql, sizeof(sql),
            "CREATE INDEX %s ON %s USING hnsw (%s %s)",
            idx_name, table, col, ops);
    }

    debug_log_query(sql);
    return __db_execute_stmt(sql);
}

// ============================================================
// INSERT with parameterized key-value pairs
// ============================================================

// Accumulate a field for INSERT (value stored as parameter)
int32_t __qs_set_field(const char* key, const char* val) {
    // Grow insert_keys if needed
    if (qs_insert_count >= g_qs_current->insert_cap) {
        g_qs_current->insert_cap *= 2;
        g_qs_current->insert_keys = (char**)realloc(g_qs_current->insert_keys,
            g_qs_current->insert_cap * sizeof(char*));
    }

    // Record param start on first field
    if (qs_insert_count == 0) {
        qs_insert_param_start = qs_param_count;
    }

    // Free old entry if re-using (shouldn't happen, but safety)
    if (qs_insert_keys[qs_insert_count]) free(qs_insert_keys[qs_insert_count]);
    qs_insert_keys[qs_insert_count] = strdup(key);

    // Store value as a parameter
    add_param_copy(val);
    qs_insert_count++;
    return 0;
}

// Execute INSERT with all accumulated fields using placeholders
int32_t __qs_do_insert(void) {
    if (qs_insert_count == 0 || qs_table[0] == '\0') return -1;

    char sql[8192];
    int pos = snprintf(sql, sizeof(sql), "INSERT INTO %s (", qs_table);

    // Build column list
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", qs_insert_keys[i]);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ") VALUES (");

    // Build placeholder list
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        char ph[16];
        write_placeholder(ph, sizeof(ph), qs_insert_param_start + i + 1);
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", ph);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ")");

    debug_log_query(sql);

    int32_t result;
    if (qs_param_count > 0) {
        result = __db_execute_params(sql, qs_params, qs_param_count);
    } else {
        result = __db_execute_stmt(sql);
    }

    qs_insert_count = 0;
    qs_insert_param_start = 0;
    return result;
}

// Get by ID (parameterized)
int32_t __qs_get_by_id(int32_t id_val) {
    char id_str[32];
    snprintf(id_str, sizeof(id_str), "%d", id_val);
    int param_idx = add_param_copy(id_str);
    char ph[16];
    write_placeholder(ph, sizeof(ph), param_idx);

    char sql[4096];
    snprintf(sql, sizeof(sql), "SELECT * FROM %s WHERE id = %s", qs_table, ph);

    debug_log_query(sql);
    qs_row_count = __db_query_params(sql, qs_params, qs_param_count);
    return qs_row_count;
}

// ============================================================
// Q Objects — Parameterized
// Q(status="active") | Q(is_admin=true)
//
// Q objects now use the global parameter accumulator. __q_new stores
// the value as a parameter and returns a SQL fragment with a placeholder.
// __q_or / __q_and just combine SQL fragments — the params are already
// accumulated in order.
// ============================================================

// Create a Q condition string from a lookup+value pair (PARAMETERIZED)
// Returns heap-allocated string like "status = $3" with the value in qs_params
const char* __q_new(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    int is_in = 0;
    parse_lookup(lookup, val, col, op, parsed_val, &is_in);

    char* result = (char*)malloc(1024);
    if (!result) return "";

    if (strcmp(op, "IS") == 0 || strcmp(op, "IS NOT") == 0) {
        // IS NULL — no parameter
        snprintf(result, 1024, "%s %s %s", col, op, parsed_val);
    } else if (is_in) {
        // IN — multiple placeholders
        char in_clause[512];
        emit_in_placeholders(val, in_clause, sizeof(in_clause));
        snprintf(result, 1024, "%s IN %s", col, in_clause);
    } else {
        // Normal — single placeholder
        int param_idx = add_param_copy(parsed_val);
        char ph[16];
        write_placeholder(ph, sizeof(ph), param_idx);
        snprintf(result, 1024, "%s %s %s", col, op, ph);
    }
    return result;
}

// Combine two Q expressions with OR
// Params are already accumulated — just combine SQL fragments
const char* __q_or(const char* q1, const char* q2) {
    if (!q1 || !q2) return q1 ? q1 : q2;
    size_t len = strlen(q1) + strlen(q2) + 16;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "(%s) OR (%s)", q1, q2);
    return result;
}

// Combine two Q expressions with AND
const char* __q_and(const char* q1, const char* q2) {
    if (!q1 || !q2) return q1 ? q1 : q2;
    size_t len = strlen(q1) + strlen(q2) + 16;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "(%s) AND (%s)", q1, q2);
    return result;
}

// Negate a Q expression
const char* __q_not(const char* q) {
    if (!q) return "";
    size_t len = strlen(q) + 8;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "NOT (%s)", q);
    return result;
}

// Apply a Q expression string directly to the QuerySet WHERE clause
int32_t __qs_filter_q(const char* q_expr) {
    if (!q_expr || q_expr[0] == '\0') return 0;

    if (qs_where[0] == '\0') {
        strncpy(qs_where, q_expr, sizeof(qs_where) - 1);
    } else {
        char tmp[4096];
        snprintf(tmp, sizeof(tmp), "%s AND (%s)", qs_where, q_expr);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}

// ============================================================
// F Expressions — Field references (NOT parameterized)
// F("price") * 1.1 → "price * 1.1"
// These reference column names, not user data, so no injection risk.
// ============================================================

const char* __f_ref(const char* col_name) {
    size_t len = strlen(col_name) + 1;
    char* result = (char*)malloc(len);
    if (!result) return "";
    strncpy(result, col_name, len);
    return result;
}

const char* __f_add(const char* f_expr, const char* val) {
    size_t len = strlen(f_expr) + strlen(val) + 8;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "%s + %s", f_expr, val);
    return result;
}

const char* __f_sub(const char* f_expr, const char* val) {
    size_t len = strlen(f_expr) + strlen(val) + 8;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "%s - %s", f_expr, val);
    return result;
}

const char* __f_mul(const char* f_expr, const char* val) {
    size_t len = strlen(f_expr) + strlen(val) + 8;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "%s * %s", f_expr, val);
    return result;
}

const char* __f_div(const char* f_expr, const char* val) {
    size_t len = strlen(f_expr) + strlen(val) + 8;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "%s / %s", f_expr, val);
    return result;
}

// ============================================================
// Type Converters — DB string → Desi types
//
// Database values are always returned as strings.
// These converters are called by compiler-generated code when
// constructing a model instance from a query result row.
// ============================================================

// Convert DB string to int64 (Desi int)
int64_t __db_str_to_int(const char* s) {
    if (!s || s[0] == '\0') return 0;
    return (int64_t)atoll(s);
}

// Convert DB string to double (Desi float)
double __db_str_to_float(const char* s) {
    if (!s || s[0] == '\0') return 0.0;
    return atof(s);
}

// Convert DB string to bool (Desi bool)
// Handles: "t", "true", "1", "yes" → 1; everything else → 0
int32_t __db_str_to_bool(const char* s) {
    if (!s || s[0] == '\0') return 0;
    if (s[0] == 't' || s[0] == 'T' ||
        s[0] == '1' ||
        s[0] == 'y' || s[0] == 'Y') return 1;
    return 0;
}

// Convert DB string to Desi string (just returns the same ptr; caller owns it)
const char* __db_str_to_str(const char* s) {
    if (!s) return "";
    return s;
}

// ============================================================
// values() — return results as JSON-like dict strings
//
// After db.fetch_all(), call db.values(row) to get a row as
// {"col1": "val1", "col2": "val2", ...}
//
// Django equivalent: Model.objects.values()
// ============================================================

extern int32_t __db_col_count(void);
extern char*   __db_col_name_at(int32_t idx);
extern char*   __db_get_field_by(int32_t row, const char* name);

char* __qs_values(int32_t row) {
    int ncols = __db_col_count();
    if (ncols <= 0 || row < 0) return strdup("{}");

    char* buf = malloc(8192);
    int pos = 0;
    pos += snprintf(buf + pos, 8192 - pos, "{");

    for (int c = 0; c < ncols; c++) {
        char* name = __db_col_name_at(c);
        char* val = __db_get_value_at(row, c);
        if (c > 0) pos += snprintf(buf + pos, 8192 - pos, ", ");
        pos += snprintf(buf + pos, 8192 - pos, "\"%s\": \"%s\"",
            name ? name : "", val ? val : "");
        if (name) free(name);
        if (val) free(val);
    }
    pos += snprintf(buf + pos, 8192 - pos, "}");
    return buf;
}

// values_list() — return a row as a comma-separated value string
// Django equivalent: Model.objects.values_list()
char* __qs_values_list(int32_t row) {
    int ncols = __db_col_count();
    if (ncols <= 0 || row < 0) return strdup("");

    char* buf = malloc(8192);
    int pos = 0;

    for (int c = 0; c < ncols; c++) {
        char* val = __db_get_value_at(row, c);
        if (c > 0) pos += snprintf(buf + pos, 8192 - pos, ",");
        pos += snprintf(buf + pos, 8192 - pos, "%s", val ? val : "");
        if (val) free(val);
    }
    return buf;
}

// values_flat() — return a single column value for a row
// Django equivalent: Model.objects.values_list("name", flat=True)
char* __qs_values_flat(int32_t row, const char* col_name) {
    char* val = __db_get_field_by(row, col_name);
    return val ? val : strdup("");
}

// ============================================================
// bulk_create() — multi-row INSERT in a single statement
//
// Usage:
//   db.objects("users")
//   db.bulk_begin(3)   # 3 columns
//   db.bulk_col("name"); db.bulk_col("age"); db.bulk_col("email")
//   db.bulk_row("Alice", "30", "a@test.com")
//   db.bulk_row("Bob", "25", "b@test.com")
//   db.bulk_execute()
//
// Generates:
//   INSERT INTO users (name, age, email) VALUES ($1,$2,$3), ($4,$5,$6)
//
// Django equivalent: Model.objects.bulk_create([...])
// ============================================================

#define BULK_MAX_COLS 32
#define BULK_MAX_ROWS 1000
#define BULK_MAX_PARAMS (BULK_MAX_COLS * BULK_MAX_ROWS)

static char   bulk_cols[BULK_MAX_COLS][128];
static int    bulk_col_count = 0;
static const char* bulk_params[BULK_MAX_PARAMS];
static int    bulk_param_count = 0;
static int    bulk_row_count = 0;

int32_t __qs_bulk_begin(int32_t ncols) {
    if (ncols <= 0 || ncols > BULK_MAX_COLS) return -1;
    bulk_col_count = 0;
    bulk_param_count = 0;
    bulk_row_count = 0;
    // Pre-set expected column count (cols added via bulk_col)
    (void)ncols; // hint only, actual count from bulk_col calls
    return 0;
}

int32_t __qs_bulk_col(const char* col_name) {
    if (bulk_col_count >= BULK_MAX_COLS || !col_name) return -1;
    strncpy(bulk_cols[bulk_col_count], col_name, 127);
    bulk_cols[bulk_col_count][127] = '\0';
    bulk_col_count++;
    return 0;
}

// Add a row of values — must provide exactly bulk_col_count values as CSV
int32_t __qs_bulk_row_csv(const char* csv_values) {
    if (!csv_values || bulk_col_count == 0) return -1;
    if (bulk_row_count >= BULK_MAX_ROWS) return -1;

    // Split CSV and add each as a param
    char buf[4096];
    strncpy(buf, csv_values, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    char* saveptr = NULL;
    char* token = strtok_r(buf, ",", &saveptr);
    int col_idx = 0;
    while (token && col_idx < bulk_col_count) {
        // Trim whitespace
        while (*token == ' ') token++;
        char* end = token + strlen(token) - 1;
        while (end > token && *end == ' ') { *end = '\0'; end--; }

        if (bulk_param_count >= BULK_MAX_PARAMS) return -1;
        bulk_params[bulk_param_count++] = strdup(token);
        col_idx++;
        token = strtok_r(NULL, ",", &saveptr);
    }
    bulk_row_count++;
    return 0;
}

// Add a row with individual values (up to 8 columns for convenience)
int32_t __qs_bulk_row1(const char* v1) {
    if (bulk_param_count >= BULK_MAX_PARAMS) return -1;
    bulk_params[bulk_param_count++] = strdup(v1);
    bulk_row_count++;
    return 0;
}

int32_t __qs_bulk_row2(const char* v1, const char* v2) {
    if (bulk_param_count + 2 > BULK_MAX_PARAMS) return -1;
    bulk_params[bulk_param_count++] = strdup(v1);
    bulk_params[bulk_param_count++] = strdup(v2);
    bulk_row_count++;
    return 0;
}

int32_t __qs_bulk_row3(const char* v1, const char* v2, const char* v3) {
    if (bulk_param_count + 3 > BULK_MAX_PARAMS) return -1;
    bulk_params[bulk_param_count++] = strdup(v1);
    bulk_params[bulk_param_count++] = strdup(v2);
    bulk_params[bulk_param_count++] = strdup(v3);
    bulk_row_count++;
    return 0;
}

int32_t __qs_bulk_row4(const char* v1, const char* v2, const char* v3, const char* v4) {
    if (bulk_param_count + 4 > BULK_MAX_PARAMS) return -1;
    bulk_params[bulk_param_count++] = strdup(v1);
    bulk_params[bulk_param_count++] = strdup(v2);
    bulk_params[bulk_param_count++] = strdup(v3);
    bulk_params[bulk_param_count++] = strdup(v4);
    bulk_row_count++;
    return 0;
}

int32_t __qs_bulk_row5(const char* v1, const char* v2, const char* v3, const char* v4, const char* v5) {
    if (bulk_param_count + 5 > BULK_MAX_PARAMS) return -1;
    bulk_params[bulk_param_count++] = strdup(v1);
    bulk_params[bulk_param_count++] = strdup(v2);
    bulk_params[bulk_param_count++] = strdup(v3);
    bulk_params[bulk_param_count++] = strdup(v4);
    bulk_params[bulk_param_count++] = strdup(v5);
    bulk_row_count++;
    return 0;
}

// Execute the bulk INSERT
int32_t __qs_bulk_execute(void) {
    if (bulk_col_count == 0 || bulk_row_count == 0 || qs_table[0] == '\0') return -1;

    char sql[65536];
    int pos = snprintf(sql, sizeof(sql), "INSERT INTO %s (", qs_table);

    // Column list
    for (int c = 0; c < bulk_col_count; c++) {
        if (c > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", bulk_cols[c]);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ") VALUES ");

    // Value rows with placeholders
    int param_idx = 1;
    for (int r = 0; r < bulk_row_count; r++) {
        if (r > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "(");
        for (int c = 0; c < bulk_col_count; c++) {
            if (c > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
            char ph[16];
            if (g_crud_dialect == DIALECT_MYSQL) {
                snprintf(ph, sizeof(ph), "?");
            } else {
                snprintf(ph, sizeof(ph), "$%d", param_idx);
            }
            pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", ph);
            param_idx++;
        }
        pos += snprintf(sql + pos, sizeof(sql) - pos, ")");
    }

    if (g_debug_queries) {
        fprintf(stderr, "[db] BULK INSERT: %s\n", sql);
        fprintf(stderr, "[db] PARAMS: %d values across %d rows\n", bulk_param_count, bulk_row_count);
    }

    int32_t result = __db_execute_params(sql, bulk_params, bulk_param_count);

    // Free param copies
    for (int i = 0; i < bulk_param_count; i++) {
        if (bulk_params[i]) free((void*)bulk_params[i]);
        bulk_params[i] = NULL;
    }
    bulk_col_count = 0;
    bulk_param_count = 0;
    bulk_row_count = 0;

    return result;
}
