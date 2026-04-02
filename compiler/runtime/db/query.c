/*
 * db_query.c — SQL Query Builder for Desi stdlib
 *
 * Generates SQL for both PostgreSQL and MySQL:
 *   - SELECT with WHERE, ORDER BY, LIMIT, OFFSET, JOIN
 *   - INSERT with VALUES
 *   - UPDATE with SET and WHERE
 *   - DELETE with WHERE
 *   - SQL dialect differences handled transparently
 *
 * Design: Chainable builder pattern stored in global state.
 * Each query is built up via calls, then rendered to SQL string.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ============================================================
// Query Builder State
// ============================================================

#define MAX_COLUMNS 32
#define MAX_CONDITIONS 16
#define MAX_SETS 32
#define MAX_JOINS 8
#define MAX_ORDER 8

typedef enum { DB_POSTGRES, DB_MYSQL } DbDialect;

typedef struct {
    char column[128];
    char op[8];
    char value[256];
} WhereClause;

typedef struct {
    char column[128];
    char value[256];
} SetClause;

typedef struct {
    char type[16];     // "INNER", "LEFT", "RIGHT"
    char table[128];
    char on_clause[256];
} JoinClause;

typedef struct {
    char column[128];
    int desc;          // 1 for DESC
} OrderClause;

typedef struct {
    // Operation
    int op_type;       // 0=SELECT, 1=INSERT, 2=UPDATE, 3=DELETE
    DbDialect dialect;

    // Table
    char table[128];

    // SELECT columns
    char columns[MAX_COLUMNS][128];
    int column_count;

    // WHERE clauses
    WhereClause wheres[MAX_CONDITIONS];
    int where_count;

    // SET clauses (INSERT/UPDATE)
    SetClause sets[MAX_SETS];
    int set_count;

    // JOIN
    JoinClause joins[MAX_JOINS];
    int join_count;

    // ORDER BY
    OrderClause orders[MAX_ORDER];
    int order_count;

    // LIMIT/OFFSET
    int limit;
    int offset;

    // RETURNING (Postgres only)
    int returning;
} QueryBuilder;

static QueryBuilder g_qb = {0};

// ============================================================
// Builder API
// ============================================================

// Reset builder
static void reset_builder(void) {
    memset(&g_qb, 0, sizeof(QueryBuilder));
    g_qb.limit = -1;
    g_qb.offset = -1;
}

// Set dialect: 0=postgres, 1=mysql
int32_t __db_set_dialect(int32_t dialect) {
    g_qb.dialect = (dialect == 1) ? DB_MYSQL : DB_POSTGRES;
    return 0;
}

// Start SELECT
int32_t __db_select(const char* table) {
    reset_builder();
    g_qb.op_type = 0;
    if (table) strncpy(g_qb.table, table, sizeof(g_qb.table) - 1);
    return 0;
}

// Start INSERT
int32_t __db_insert_into(const char* table) {
    reset_builder();
    g_qb.op_type = 1;
    if (table) strncpy(g_qb.table, table, sizeof(g_qb.table) - 1);
    return 0;
}

// Start UPDATE
int32_t __db_update_table(const char* table) {
    reset_builder();
    g_qb.op_type = 2;
    if (table) strncpy(g_qb.table, table, sizeof(g_qb.table) - 1);
    return 0;
}

// Start DELETE
int32_t __db_delete_from(const char* table) {
    reset_builder();
    g_qb.op_type = 3;
    if (table) strncpy(g_qb.table, table, sizeof(g_qb.table) - 1);
    return 0;
}

// Add column to SELECT
int32_t __db_columns(const char* col) {
    if (g_qb.column_count < MAX_COLUMNS && col) {
        strncpy(g_qb.columns[g_qb.column_count++], col, 127);
    }
    return 0;
}

// Add WHERE clause
int32_t __db_where(const char* column, const char* op, const char* value) {
    if (g_qb.where_count < MAX_CONDITIONS) {
        WhereClause* w = &g_qb.wheres[g_qb.where_count++];
        if (column) strncpy(w->column, column, 127);
        if (op) strncpy(w->op, op, 7);
        if (value) strncpy(w->value, value, 255);
    }
    return 0;
}

// Add SET clause
int32_t __db_set(const char* column, const char* value) {
    if (g_qb.set_count < MAX_SETS) {
        SetClause* s = &g_qb.sets[g_qb.set_count++];
        if (column) strncpy(s->column, column, 127);
        if (value) strncpy(s->value, value, 255);
    }
    return 0;
}

// Add JOIN
int32_t __db_join(const char* type, const char* table, const char* on_clause) {
    if (g_qb.join_count < MAX_JOINS) {
        JoinClause* j = &g_qb.joins[g_qb.join_count++];
        if (type) strncpy(j->type, type, 15);
        if (table) strncpy(j->table, table, 127);
        if (on_clause) strncpy(j->on_clause, on_clause, 255);
    }
    return 0;
}

// Add ORDER BY
int32_t __db_order_by(const char* column, int32_t desc) {
    if (g_qb.order_count < MAX_ORDER && column) {
        g_qb.orders[g_qb.order_count].desc = desc;
        strncpy(g_qb.orders[g_qb.order_count++].column, column, 127);
    }
    return 0;
}

// Set LIMIT
int32_t __db_limit(int32_t n) {
    g_qb.limit = n;
    return 0;
}

// Set OFFSET
int32_t __db_offset(int32_t n) {
    g_qb.offset = n;
    return 0;
}

// Enable RETURNING (Postgres only)
int32_t __db_returning(void) {
    g_qb.returning = 1;
    return 0;
}

// ============================================================
// SQL Generation
// ============================================================

// Escape a value for SQL (basic — wraps in quotes, doubles internal quotes)
static void write_value(char* buf, int* pos, const char* val, DbDialect dialect) {
    // Check if it's a number
    int is_num = 1;
    int has_dot = 0;
    for (int i = 0; val[i]; i++) {
        if (val[i] == '.' && !has_dot) { has_dot = 1; continue; }
        if (val[i] == '-' && i == 0) continue;
        if (val[i] < '0' || val[i] > '9') { is_num = 0; break; }
    }
    
    if (is_num && strlen(val) > 0) {
        *pos += sprintf(buf + *pos, "%s", val);
    } else {
        *pos += sprintf(buf + *pos, "'");
        for (int i = 0; val[i]; i++) {
            if (val[i] == '\'') {
                *pos += sprintf(buf + *pos, "''");
            } else {
                buf[(*pos)++] = val[i];
            }
        }
        *pos += sprintf(buf + *pos, "'");
    }
}

// Build the SQL string
char* __db_build_sql(void) {
    char* sql = malloc(4096);
    int pos = 0;
    sql[0] = '\0';

    switch (g_qb.op_type) {
        case 0: { // SELECT
            pos += sprintf(sql + pos, "SELECT ");
            if (g_qb.column_count == 0) {
                pos += sprintf(sql + pos, "*");
            } else {
                for (int i = 0; i < g_qb.column_count; i++) {
                    if (i > 0) pos += sprintf(sql + pos, ", ");
                    pos += sprintf(sql + pos, "%s", g_qb.columns[i]);
                }
            }
            pos += sprintf(sql + pos, " FROM %s", g_qb.table);

            // JOINs
            for (int i = 0; i < g_qb.join_count; i++) {
                pos += sprintf(sql + pos, " %s JOIN %s ON %s",
                    g_qb.joins[i].type, g_qb.joins[i].table, g_qb.joins[i].on_clause);
            }

            // WHERE
            if (g_qb.where_count > 0) {
                pos += sprintf(sql + pos, " WHERE ");
                for (int i = 0; i < g_qb.where_count; i++) {
                    if (i > 0) pos += sprintf(sql + pos, " AND ");
                    pos += sprintf(sql + pos, "%s %s ", g_qb.wheres[i].column, g_qb.wheres[i].op);
                    write_value(sql, &pos, g_qb.wheres[i].value, g_qb.dialect);
                }
            }

            // ORDER BY
            if (g_qb.order_count > 0) {
                pos += sprintf(sql + pos, " ORDER BY ");
                for (int i = 0; i < g_qb.order_count; i++) {
                    if (i > 0) pos += sprintf(sql + pos, ", ");
                    pos += sprintf(sql + pos, "%s%s", g_qb.orders[i].column,
                        g_qb.orders[i].desc ? " DESC" : "");
                }
            }

            // LIMIT/OFFSET
            if (g_qb.limit >= 0) pos += sprintf(sql + pos, " LIMIT %d", g_qb.limit);
            if (g_qb.offset >= 0) pos += sprintf(sql + pos, " OFFSET %d", g_qb.offset);
            break;
        }

        case 1: { // INSERT
            pos += sprintf(sql + pos, "INSERT INTO %s (", g_qb.table);
            for (int i = 0; i < g_qb.set_count; i++) {
                if (i > 0) pos += sprintf(sql + pos, ", ");
                pos += sprintf(sql + pos, "%s", g_qb.sets[i].column);
            }
            pos += sprintf(sql + pos, ") VALUES (");
            for (int i = 0; i < g_qb.set_count; i++) {
                if (i > 0) pos += sprintf(sql + pos, ", ");
                write_value(sql, &pos, g_qb.sets[i].value, g_qb.dialect);
            }
            pos += sprintf(sql + pos, ")");

            if (g_qb.returning && g_qb.dialect == DB_POSTGRES) {
                pos += sprintf(sql + pos, " RETURNING *");
            }
            break;
        }

        case 2: { // UPDATE
            pos += sprintf(sql + pos, "UPDATE %s SET ", g_qb.table);
            for (int i = 0; i < g_qb.set_count; i++) {
                if (i > 0) pos += sprintf(sql + pos, ", ");
                pos += sprintf(sql + pos, "%s = ", g_qb.sets[i].column);
                write_value(sql, &pos, g_qb.sets[i].value, g_qb.dialect);
            }

            if (g_qb.where_count > 0) {
                pos += sprintf(sql + pos, " WHERE ");
                for (int i = 0; i < g_qb.where_count; i++) {
                    if (i > 0) pos += sprintf(sql + pos, " AND ");
                    pos += sprintf(sql + pos, "%s %s ", g_qb.wheres[i].column, g_qb.wheres[i].op);
                    write_value(sql, &pos, g_qb.wheres[i].value, g_qb.dialect);
                }
            }
            break;
        }

        case 3: { // DELETE
            pos += sprintf(sql + pos, "DELETE FROM %s", g_qb.table);

            if (g_qb.where_count > 0) {
                pos += sprintf(sql + pos, " WHERE ");
                for (int i = 0; i < g_qb.where_count; i++) {
                    if (i > 0) pos += sprintf(sql + pos, " AND ");
                    pos += sprintf(sql + pos, "%s %s ", g_qb.wheres[i].column, g_qb.wheres[i].op);
                    write_value(sql, &pos, g_qb.wheres[i].value, g_qb.dialect);
                }
            }
            break;
        }
    }

    sql[pos] = '\0';
    return sql;
}
