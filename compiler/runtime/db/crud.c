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
// QuerySet state (shared, single-threaded)
// ============================================================

#define QS_MAX_PARAMS 128

static char   qs_table[128] = "";
static char   qs_where[4096] = "";
static char   qs_order[256] = "";
static int    qs_limit = 0;
static int    qs_offset = 0;
static int    qs_distinct = 0;
static int    qs_row_count = 0;
static char   qs_columns[1024] = "";  // only() column list, empty = *

// Parameter accumulator — values are stored here, SQL has placeholders
static const char* qs_params[QS_MAX_PARAMS];
static int    qs_param_count = 0;

// INSERT field accumulator (up to 32 fields)
#define QS_MAX_FIELDS 32
static char   qs_insert_keys[QS_MAX_FIELDS][128];
// INSERT values are stored in qs_params (starting at qs_insert_param_start)
static int    qs_insert_count = 0;
static int    qs_insert_param_start = 0;

// select_related — FK field names to LEFT JOIN
#define QS_MAX_RELATED 8
static char   qs_related[QS_MAX_RELATED][128];
static int    qs_related_count = 0;

// annotate + GROUP BY
#define QS_MAX_ANNOTATIONS 8
typedef struct {
    char alias[128];    // output alias, e.g. "total"
    char func[16];      // aggregate function: SUM, COUNT, AVG, MIN, MAX
    char col[128];      // column name, e.g. "amount"
} Annotation;

static Annotation qs_annotations[QS_MAX_ANNOTATIONS];
static int    qs_annotation_count = 0;
static char   qs_group_by[512] = "";

// Forward declarations for functions used before their definition
int32_t __qs_do_insert(void);

// Phase 2 state (declared here so __qs_reset can access them)
static char qs_upsert_col[128];
static char qs_window_expr[512];
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
    qs_distinct = 0;
    qs_row_count = 0;
    qs_insert_count = 0;
    qs_insert_param_start = 0;
    qs_related_count = 0;
    qs_annotation_count = 0;
    qs_group_by[0] = '\0';
    qs_columns[0] = '\0';
    qs_upsert_col[0] = '\0';
    qs_window_expr[0] = '\0';
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
    // ---- Phase 2: Date lookups ----
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
    // ---- Phase 2: JSON field lookups ----
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
    char sql[8192];
    int pos;

    // Build column list and JOIN clauses for select_related
    if (qs_annotation_count > 0) {
        // Annotated query: SELECT group_cols, AGG(col) AS alias, ...
        pos = snprintf(sql, sizeof(sql), "SELECT %s",
            qs_distinct ? "DISTINCT " : "");

        // If GROUP BY is set, include those columns first
        if (qs_group_by[0]) {
            pos += snprintf(sql + pos, sizeof(sql) - pos, "%s", qs_group_by);
        } else {
            pos += snprintf(sql + pos, sizeof(sql) - pos, "%s.*", qs_table);
        }

        // Add annotations: SUM(col) AS alias
        for (int a = 0; a < qs_annotation_count; a++) {
            pos += snprintf(sql + pos, sizeof(sql) - pos, ", %s(%s) AS %s",
                qs_annotations[a].func, qs_annotations[a].col, qs_annotations[a].alias);
        }

        pos += snprintf(sql + pos, sizeof(sql) - pos, " FROM %s", qs_table);
    } else if (qs_related_count > 0) {
        // Build: SELECT t.*, r1.col1 AS r1__col1, r1.col2 AS r1__col2, ...
        pos = snprintf(sql, sizeof(sql), "SELECT %s%s.*",
            qs_distinct ? "DISTINCT " : "", qs_table);

        // For each related FK, add aliased columns from the related table
        for (int r = 0; r < qs_related_count; r++) {
            char ref_table[128] = "", ref_field[64] = "";
            if (__orm_fk_info(qs_table, qs_related[r], ref_table, sizeof(ref_table),
                             ref_field, sizeof(ref_field))) {
                int fcount = __orm_field_count(ref_table);
                for (int f = 0; f < fcount; f++) {
                    const char* fname = __orm_field_name(ref_table, f);
                    pos += snprintf(sql + pos, sizeof(sql) - pos,
                        ", %s_rel%d.%s AS %s__%s",
                        qs_related[r], r, fname, qs_related[r], fname);
                }
            }
        }

        pos += snprintf(sql + pos, sizeof(sql) - pos, " FROM %s", qs_table);

        // Add LEFT JOINs
        for (int r = 0; r < qs_related_count; r++) {
            char ref_table[128] = "", ref_field[64] = "";
            if (__orm_fk_info(qs_table, qs_related[r], ref_table, sizeof(ref_table),
                             ref_field, sizeof(ref_field))) {
                pos += snprintf(sql + pos, sizeof(sql) - pos,
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
            pos = snprintf(sql, sizeof(sql), "SELECT DISTINCT %s FROM %s", cols, qs_table);
        } else {
            pos = snprintf(sql, sizeof(sql), "SELECT %s FROM %s", cols, qs_table);
        }
    }

    if (qs_where[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " WHERE %s", qs_where);
    if (qs_group_by[0]) pos += snprintf(sql + pos, sizeof(sql) - pos, " GROUP BY %s", qs_group_by);
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

// __qs_annotate — add an aggregate annotation
// Django equivalent: .annotate(total=Sum("amount"))
// func: "SUM", "COUNT", "AVG", "MIN", "MAX"
int32_t __qs_annotate(const char* alias, const char* func, const char* col) {
    if (qs_annotation_count >= QS_MAX_ANNOTATIONS || !alias || !func || !col) return -1;
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
// Phase 1: latest / earliest (user-specified field)
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
// Phase 1: only() — restrict SELECT columns
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
// Phase 1: get_or_create()
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
// Phase 1: EXPLAIN — query plan inspection
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
// Phase 1: Schema Inspection
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
// Phase 1: Advisory Locks (PG)
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
// Phase 2: UPSERT — INSERT ... ON CONFLICT DO UPDATE
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
// Phase 2: update_or_create
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
// Phase 2: Window Functions
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
// Phase 2: Server-Side Cursor
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
// Phase 2: JSON Field Helpers
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
