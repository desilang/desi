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

// ---- External: dispatch layer ----
extern int32_t __db_query_exec(const char* sql);
extern int32_t __db_query_params(const char* sql, const char** params, int nparams);
extern int32_t __db_execute_stmt(const char* sql);
extern int32_t __db_execute_params(const char* sql, const char** params, int nparams);
extern char*   __db_get_value_at(int32_t row, int32_t col);
extern int32_t __db_is_connected(void);

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
// QuerySet state (shared, single-threaded)
// ============================================================

#define QS_MAX_PARAMS 128

static char   qs_table[128] = "";
static char   qs_where[4096] = "";
static char   qs_order[256] = "";
static int    qs_limit = 0;
static int    qs_offset = 0;
static int    qs_row_count = 0;

// Parameter accumulator — values are stored here, SQL has placeholders
static const char* qs_params[QS_MAX_PARAMS];
static int    qs_param_count = 0;

// INSERT field accumulator (up to 32 fields)
#define QS_MAX_FIELDS 32
static char   qs_insert_keys[QS_MAX_FIELDS][128];
// INSERT values are stored in qs_params (starting at qs_insert_param_start)
static int    qs_insert_count = 0;
static int    qs_insert_param_start = 0;

// Helper: write a placeholder for the current dialect
static int write_placeholder(char* buf, int buf_size, int param_index) {
    if (g_crud_dialect == DIALECT_MYSQL) {
        return snprintf(buf, buf_size, "?");
    } else {
        return snprintf(buf, buf_size, "$%d", param_index);
    }
}

// Helper: add a parameter and return its 1-based index
static int add_param(const char* val) {
    if (qs_param_count >= QS_MAX_PARAMS) return qs_param_count;
    qs_params[qs_param_count] = val;  // caller must ensure lifetime
    qs_param_count++;
    return qs_param_count;
}

// Helper: add a heap-allocated copy of val as parameter
static int add_param_copy(const char* val) {
    return add_param(strdup(val));
}

// Debug: print query + params to stderr
static void debug_log_query(const char* sql) {
    if (!g_debug_queries) return;
    fprintf(stderr, "[db] QUERY: %s\n", sql);
    if (qs_param_count > 0) {
        fprintf(stderr, "[db] PARAMS: [");
        for (int i = 0; i < qs_param_count; i++) {
            if (i > 0) fprintf(stderr, ", ");
            fprintf(stderr, "\"%s\"", qs_params[i] ? qs_params[i] : "NULL");
        }
        fprintf(stderr, "]\n");
    }
}

// Free all parameter copies
static void free_params(void) {
    for (int i = 0; i < qs_param_count; i++) {
        if (qs_params[i]) {
            free((void*)qs_params[i]);
            qs_params[i] = NULL;
        }
    }
    qs_param_count = 0;
}

// Reset queryset state
int32_t __qs_reset(const char* table) {
    strncpy(qs_table, table, sizeof(qs_table) - 1);
    qs_table[sizeof(qs_table) - 1] = '\0';
    qs_where[0] = '\0';
    qs_order[0] = '\0';
    qs_limit = 0;
    qs_offset = 0;
    qs_row_count = 0;
    qs_insert_count = 0;
    qs_insert_param_start = 0;
    free_params();
    return 0;
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
    } else if (strcmp(suffix, "exact") == 0) {
        strcpy(op_out, "=");
    } else if (strcmp(suffix, "contains") == 0) {
        strcpy(op_out, "LIKE");
        // Modify the VALUE (not the SQL) to add wildcards
        snprintf(val_out, 512, "%%%s%%", val);
    } else if (strcmp(suffix, "startswith") == 0) {
        strcpy(op_out, "LIKE");
        snprintf(val_out, 512, "%s%%", val);
    } else if (strcmp(suffix, "endswith") == 0) {
        strcpy(op_out, "LIKE");
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
    } else {
        // Unknown suffix — treat as exact match on full name
        *dunder = '_';
        *(dunder + 1) = '_';
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
    } else if (is_in) {
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
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT * FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    if (qs_order[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " ORDER BY %s", qs_order);
    if (qs_limit > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " LIMIT %d", qs_limit);
    if (qs_offset > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " OFFSET %d", qs_offset);

    debug_log_query(sql);

    if (qs_param_count > 0) {
        qs_row_count = __db_query_params(sql, qs_params, qs_param_count);
    } else {
        qs_row_count = __db_query_exec(sql);
    }
    return qs_row_count;
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
// INSERT with parameterized key-value pairs
// ============================================================

// Accumulate a field for INSERT (value stored as parameter)
int32_t __qs_set_field(const char* key, const char* val) {
    if (qs_insert_count >= QS_MAX_FIELDS) return -1;

    // Record param start on first field
    if (qs_insert_count == 0) {
        qs_insert_param_start = qs_param_count;
    }

    strncpy(qs_insert_keys[qs_insert_count], key, 127);
    qs_insert_keys[qs_insert_count][127] = '\0';

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

// Legacy fixed-arg create functions (kept for backward compat, now use params)
int32_t __qs_create1(const char* k1, const char* v1) {
    __qs_set_field(k1, v1);
    return __qs_do_insert();
}

int32_t __qs_create2(const char* k1, const char* v1, const char* k2, const char* v2) {
    __qs_set_field(k1, v1);
    __qs_set_field(k2, v2);
    return __qs_do_insert();
}

int32_t __qs_create3(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3) {
    __qs_set_field(k1, v1);
    __qs_set_field(k2, v2);
    __qs_set_field(k3, v3);
    return __qs_do_insert();
}

int32_t __qs_create4(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3, const char* k4, const char* v4) {
    __qs_set_field(k1, v1);
    __qs_set_field(k2, v2);
    __qs_set_field(k3, v3);
    __qs_set_field(k4, v4);
    return __qs_do_insert();
}

int32_t __qs_create5(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3, const char* k4, const char* v4,
                     const char* k5, const char* v5) {
    __qs_set_field(k1, v1);
    __qs_set_field(k2, v2);
    __qs_set_field(k3, v3);
    __qs_set_field(k4, v4);
    __qs_set_field(k5, v5);
    return __qs_do_insert();
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
