/*
 * db_crud.c — ORM CRUD operations for Desi
 *
 * Provides C-level functions for:
 *   - Django-style lookup parsing (__lt, __gt, __contains, etc.)
 *   - QuerySet SQL generation (SELECT/UPDATE/DELETE with WHERE)
 *   - INSERT with key-value pairs
 *   - Aggregations (COUNT, SUM, AVG, MIN, MAX)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ---- External: dispatch layer ----
extern int32_t __db_query_exec(const char* sql);
extern int32_t __db_execute_stmt(const char* sql);
extern char*   __db_get_value_at(int32_t row, int32_t col);
extern int32_t __db_is_connected(void);

// ============================================================
// QuerySet state (shared, single-threaded)
// ============================================================

static char   qs_table[128] = "";
static char   qs_where[2048] = "";
static char   qs_order[256] = "";
static int    qs_limit = 0;
static int    qs_offset = 0;
static int    qs_row_count = 0;

// INSERT field accumulator (up to 32 fields)
#define QS_MAX_FIELDS 32
static char   qs_insert_keys[QS_MAX_FIELDS][128];
static char   qs_insert_vals[QS_MAX_FIELDS][512];
static int    qs_insert_count = 0;

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
    return 0;
}

// ============================================================
// Django-style lookup parser
// ============================================================

// Parse "age__lt" → col="age", op="<"
// Parse "name__contains" → col="name", op="LIKE", val modified to "%val%"
// Parse "age" → col="age", op="=" (default: exact match)
static void parse_lookup(const char* lookup, const char* val,
                         char* col_out, char* op_out, char* val_out) {
    // Copy defaults
    strncpy(col_out, lookup, 127);
    col_out[127] = '\0';
    strcpy(op_out, "=");
    strncpy(val_out, val, 511);
    val_out[511] = '\0';

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
        snprintf(val_out, 512, "'%%%s%%'", val);
    } else if (strcmp(suffix, "startswith") == 0) {
        strcpy(op_out, "LIKE");
        snprintf(val_out, 512, "'%s%%'", val);
    } else if (strcmp(suffix, "endswith") == 0) {
        strcpy(op_out, "LIKE");
        snprintf(val_out, 512, "'%%%s'", val);
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
        snprintf(val_out, 512, "(%s)", val);
    } else {
        // Unknown suffix — treat as exact match on full name
        // Restore the dunder
        *dunder = '_';
        *(dunder + 1) = '_';
    }
}

// ============================================================
// Filter / Exclude — add WHERE conditions
// ============================================================

int32_t __qs_filter(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    parse_lookup(lookup, val, col, op, parsed_val);

    char condition[1024];
    snprintf(condition, sizeof(condition), "%s %s %s", col, op, parsed_val);

    if (qs_where[0] == '\0') {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    } else {
        char tmp[2048];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_where, condition);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}

int32_t __qs_exclude(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    parse_lookup(lookup, val, col, op, parsed_val);

    char condition[1024];
    snprintf(condition, sizeof(condition), "NOT (%s %s %s)", col, op, parsed_val);

    if (qs_where[0] == '\0') {
        strncpy(qs_where, condition, sizeof(qs_where) - 1);
    } else {
        char tmp[2048];
        snprintf(tmp, sizeof(tmp), "%s AND %s", qs_where, condition);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}

// ============================================================
// Order / Limit / Offset
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
// ============================================================

// Build SELECT SQL and execute
int32_t __qs_fetch(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT * FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    if (qs_order[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " ORDER BY %s", qs_order);
    if (qs_limit > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " LIMIT %d", qs_limit);
    if (qs_offset > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, " OFFSET %d", qs_offset);

    qs_row_count = __db_query_exec(sql);
    return qs_row_count;
}

int32_t __qs_row_count(void) {
    return qs_row_count;
}

// SELECT COUNT(*)
int32_t __qs_count(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT COUNT(*) FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    int rows = __db_query_exec(sql);
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

// UPDATE table SET col=val WHERE ...
int32_t __qs_update(const char* col, const char* val) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "UPDATE %s SET %s = '%s'", qs_table, col, val);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    return __db_execute_stmt(sql);
}

// DELETE FROM table WHERE ...
int32_t __qs_delete(void) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "DELETE FROM %s", qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    return __db_execute_stmt(sql);
}

// ============================================================
// Aggregations
// ============================================================

char* __qs_aggregate(const char* func, const char* col) {
    char sql[4096];
    int pos = snprintf(sql, sizeof(sql), "SELECT %s(%s) FROM %s", func, col, qs_table);
    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);

    int rows = __db_query_exec(sql);
    if (rows > 0) {
        return __db_get_value_at(0, 0);
    }
    return strdup("0");
}

// ============================================================
// INSERT with key-value pairs (up to 5 fields)
// ============================================================

int32_t __qs_create1(const char* k1, const char* v1) {
    char sql[4096];
    snprintf(sql, sizeof(sql), "INSERT INTO %s (%s) VALUES ('%s')", qs_table, k1, v1);
    return __db_execute_stmt(sql);
}

int32_t __qs_create2(const char* k1, const char* v1, const char* k2, const char* v2) {
    char sql[4096];
    snprintf(sql, sizeof(sql), "INSERT INTO %s (%s, %s) VALUES ('%s', '%s')",
        qs_table, k1, k2, v1, v2);
    return __db_execute_stmt(sql);
}

int32_t __qs_create3(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3) {
    char sql[4096];
    snprintf(sql, sizeof(sql), "INSERT INTO %s (%s, %s, %s) VALUES ('%s', '%s', '%s')",
        qs_table, k1, k2, k3, v1, v2, v3);
    return __db_execute_stmt(sql);
}

int32_t __qs_create4(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3, const char* k4, const char* v4) {
    char sql[4096];
    snprintf(sql, sizeof(sql), "INSERT INTO %s (%s, %s, %s, %s) VALUES ('%s', '%s', '%s', '%s')",
        qs_table, k1, k2, k3, k4, v1, v2, v3, v4);
    return __db_execute_stmt(sql);
}

int32_t __qs_create5(const char* k1, const char* v1, const char* k2, const char* v2,
                     const char* k3, const char* v3, const char* k4, const char* v4,
                     const char* k5, const char* v5) {
    char sql[4096];
    snprintf(sql, sizeof(sql),
        "INSERT INTO %s (%s, %s, %s, %s, %s) VALUES ('%s', '%s', '%s', '%s', '%s')",
        qs_table, k1, k2, k3, k4, k5, v1, v2, v3, v4, v5);
    return __db_execute_stmt(sql);
}

// Get by ID (SELECT * WHERE id = ?)
int32_t __qs_get_by_id(int32_t id_val) {
    char sql[4096];
    snprintf(sql, sizeof(sql), "SELECT * FROM %s WHERE id = %d", qs_table, id_val);
    qs_row_count = __db_query_exec(sql);
    return qs_row_count;
}

// ============================================================
// Unlimited-field INSERT: set fields then execute
// ============================================================

// Accumulate a field for INSERT
int32_t __qs_set_field(const char* key, const char* val) {
    if (qs_insert_count >= QS_MAX_FIELDS) return -1;
    strncpy(qs_insert_keys[qs_insert_count], key, 127);
    qs_insert_keys[qs_insert_count][127] = '\0';
    strncpy(qs_insert_vals[qs_insert_count], val, 511);
    qs_insert_vals[qs_insert_count][511] = '\0';
    qs_insert_count++;
    return 0;
}

// Execute INSERT with all accumulated fields
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

    // Build value list
    for (int i = 0; i < qs_insert_count; i++) {
        if (i > 0) pos += snprintf(sql + pos, sizeof(sql) - pos, ", ");
        pos += snprintf(sql + pos, sizeof(sql) - pos, "'%s'", qs_insert_vals[i]);
    }
    pos += snprintf(sql + pos, sizeof(sql) - pos, ")");

    qs_insert_count = 0;  // reset after insert
    return __db_execute_stmt(sql);
}

// ============================================================
// Q Objects — Django-style complex lookups
// Q(status="active") | Q(is_admin=true)
// ============================================================

// Create a Q condition string from a lookup+value pair
// Returns heap-allocated string like "status = 'active'"
const char* __q_new(const char* lookup, const char* val) {
    char col[128], op[16], parsed_val[512];
    parse_lookup(lookup, val, col, op, parsed_val);

    char* result = (char*)malloc(1024);
    if (!result) return "";

    // For LIKE/IN/IS operators, val is already formatted by parse_lookup
    if (strcmp(op, "LIKE") == 0 || strcmp(op, "IN") == 0 ||
        strcmp(op, "IS") == 0 || strcmp(op, "IS NOT") == 0) {
        snprintf(result, 1024, "%s %s %s", col, op, parsed_val);
    } else {
        snprintf(result, 1024, "%s %s '%s'", col, op, parsed_val);
    }
    return result;
}

// Combine two Q expressions with OR
// Returns heap-allocated string like "(cond1) OR (cond2)"
const char* __q_or(const char* q1, const char* q2) {
    if (!q1 || !q2) return q1 ? q1 : q2;
    size_t len = strlen(q1) + strlen(q2) + 16;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "(%s) OR (%s)", q1, q2);
    return result;
}

// Combine two Q expressions with AND
// Returns heap-allocated string like "(cond1) AND (cond2)"
const char* __q_and(const char* q1, const char* q2) {
    if (!q1 || !q2) return q1 ? q1 : q2;
    size_t len = strlen(q1) + strlen(q2) + 16;
    char* result = (char*)malloc(len);
    if (!result) return "";
    snprintf(result, len, "(%s) AND (%s)", q1, q2);
    return result;
}

// Negate a Q expression
// Returns heap-allocated string like "NOT (cond)"
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
        char tmp[2048];
        snprintf(tmp, sizeof(tmp), "%s AND (%s)", qs_where, q_expr);
        strncpy(qs_where, tmp, sizeof(qs_where) - 1);
    }
    return 0;
}
