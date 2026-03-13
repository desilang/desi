/*
 * db_orm.c — ORM model/field registry and SQL generation for Desi stdlib
 *
 * Provides:
 *   - Model registration with field definitions
 *   - Field types: auto, int, varchar, text, bool, float, datetime, json
 *   - CREATE TABLE generation (Postgres & MySQL)
 *   - ALTER TABLE generation for migrations
 *   - CRUD record helpers
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// ============================================================
// Model Registry
// ============================================================

typedef enum {
    FIELD_AUTO,        // auto-increment primary key
    FIELD_INT,
    FIELD_BIGINT,
    FIELD_VARCHAR,
    FIELD_TEXT,
    FIELD_BOOL,
    FIELD_FLOAT,
    FIELD_DOUBLE,
    FIELD_DATETIME,
    FIELD_DATE,
    FIELD_JSON,
    FIELD_FOREIGN_KEY
} FieldType;

typedef struct {
    char name[64];
    FieldType type;
    int max_length;      // for VARCHAR
    int nullable;
    int unique;
    int primary_key;
    int auto_now;        // set to current time on create
    int auto_now_add;    // set to current time on update
    char default_val[256];
    char ref_table[128]; // for FOREIGN_KEY
    char ref_field[64];  // for FOREIGN_KEY
} FieldDef;

typedef struct {
    char name[128];      // table name
    FieldDef fields[64];
    int field_count;
} ModelDef;

#define MAX_MODELS 32
static ModelDef g_models[MAX_MODELS];
static int g_model_count = 0;
static int g_current_model = -1;

// Dialect for SQL generation
static int g_orm_dialect = 0; // 0=postgres, 1=mysql

int32_t __orm_set_dialect(int32_t dialect) {
    g_orm_dialect = dialect;
    return 0;
}

// ============================================================
// Model Definition API
// ============================================================

// Start defining a model (table)
int32_t __orm_model(const char* table_name) {
    if (g_model_count >= MAX_MODELS || !table_name) return -1;
    g_current_model = g_model_count++;
    ModelDef* m = &g_models[g_current_model];
    memset(m, 0, sizeof(ModelDef));
    strncpy(m->name, table_name, sizeof(m->name) - 1);
    return g_current_model;
}

// Add AutoField (auto-increment primary key)
int32_t __orm_auto_field(const char* name) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name ? name : "id", sizeof(f->name) - 1);
    f->type = FIELD_AUTO;
    f->primary_key = 1;
    return 0;
}

// Add IntField
int32_t __orm_int_field(const char* name, int32_t default_val, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_INT;
    f->nullable = nullable;
    snprintf(f->default_val, sizeof(f->default_val), "%d", default_val);
    return 0;
}

// Add BigIntField
int32_t __orm_bigint_field(const char* name, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_BIGINT;
    f->nullable = nullable;
    return 0;
}

// Add CharField (varchar with max_length)
int32_t __orm_char_field(const char* name, int32_t max_length, int32_t nullable, int32_t unique) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_VARCHAR;
    f->max_length = max_length > 0 ? max_length : 255;
    f->nullable = nullable;
    f->unique = unique;
    return 0;
}

// Add TextField (unlimited text)
int32_t __orm_text_field(const char* name, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_TEXT;
    f->nullable = nullable;
    return 0;
}

// Add BoolField
int32_t __orm_bool_field(const char* name, int32_t default_val, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_BOOL;
    f->nullable = nullable;
    snprintf(f->default_val, sizeof(f->default_val), "%s", default_val ? "TRUE" : "FALSE");
    return 0;
}

// Add FloatField
int32_t __orm_float_field(const char* name, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_DOUBLE;
    f->nullable = nullable;
    return 0;
}

// Add DateTimeField
int32_t __orm_datetime_field(const char* name, int32_t auto_now, int32_t auto_now_add, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_DATETIME;
    f->auto_now = auto_now;
    f->auto_now_add = auto_now_add;
    f->nullable = nullable;
    return 0;
}

// Add JSONField
int32_t __orm_json_field(const char* name, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_JSON;
    f->nullable = nullable;
    return 0;
}

// Add ForeignKey
int32_t __orm_foreign_key(const char* name, const char* ref_table, const char* ref_field, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_FOREIGN_KEY;
    if (ref_table) strncpy(f->ref_table, ref_table, sizeof(f->ref_table) - 1);
    if (ref_field) strncpy(f->ref_field, ref_field, sizeof(f->ref_field) - 1);
    f->nullable = nullable;
    return 0;
}

// ============================================================
// SQL Generation from Models
// ============================================================

// Get SQL type string for a field
static const char* sql_type(FieldDef* f, int dialect) {
    static char buf[64];
    switch (f->type) {
        case FIELD_AUTO:
            if (dialect == 0) return "SERIAL PRIMARY KEY";
            return "INT AUTO_INCREMENT PRIMARY KEY";
        case FIELD_INT: return "INTEGER";
        case FIELD_BIGINT: return "BIGINT";
        case FIELD_VARCHAR:
            snprintf(buf, sizeof(buf), "VARCHAR(%d)", f->max_length);
            return buf;
        case FIELD_TEXT: return "TEXT";
        case FIELD_BOOL:
            if (dialect == 0) return "BOOLEAN";
            return "TINYINT(1)";
        case FIELD_FLOAT: return "REAL";
        case FIELD_DOUBLE:
            if (dialect == 0) return "DOUBLE PRECISION";
            return "DOUBLE";
        case FIELD_DATETIME:
            if (dialect == 0) return "TIMESTAMP";
            return "DATETIME";
        case FIELD_DATE: return "DATE";
        case FIELD_JSON:
            if (dialect == 0) return "JSONB";
            return "JSON";
        case FIELD_FOREIGN_KEY: return "INTEGER";
        default: return "TEXT";
    }
}

// Generate CREATE TABLE SQL for a model
char* __orm_create_table_sql(const char* table_name) {
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            model = &g_models[i];
            break;
        }
    }
    if (!model) return strdup("");

    char* sql = malloc(4096);
    int pos = 0;
    pos += sprintf(sql + pos, "CREATE TABLE IF NOT EXISTS %s (\n", model->name);

    for (int i = 0; i < model->field_count; i++) {
        FieldDef* f = &model->fields[i];
        if (i > 0) pos += sprintf(sql + pos, ",\n");
        pos += sprintf(sql + pos, "  %s %s", f->name, sql_type(f, g_orm_dialect));

        if (f->type != FIELD_AUTO) {
            if (!f->nullable) pos += sprintf(sql + pos, " NOT NULL");
            if (f->unique) pos += sprintf(sql + pos, " UNIQUE");
            if (strlen(f->default_val) > 0 && f->type != FIELD_FOREIGN_KEY) {
                pos += sprintf(sql + pos, " DEFAULT %s", f->default_val);
            }
            if (f->auto_now_add) {
                if (g_orm_dialect == 0) pos += sprintf(sql + pos, " DEFAULT NOW()");
                else pos += sprintf(sql + pos, " DEFAULT CURRENT_TIMESTAMP");
            }
        }

        if (f->type == FIELD_FOREIGN_KEY) {
            pos += sprintf(sql + pos, " REFERENCES %s(%s)", f->ref_table, f->ref_field);
        }
    }

    pos += sprintf(sql + pos, "\n)");
    if (g_orm_dialect == 1) {
        pos += sprintf(sql + pos, " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4");
    }
    sql[pos] = '\0';
    return sql;
}

// Generate DROP TABLE SQL
char* __orm_drop_table_sql(const char* table_name) {
    char buf[256];
    snprintf(buf, sizeof(buf), "DROP TABLE IF EXISTS %s", table_name);
    return strdup(buf);
}

// Generate ALTER TABLE ADD COLUMN
char* __orm_add_column_sql(const char* table_name, const char* col_name) {
    // Find model and field
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            model = &g_models[i];
            break;
        }
    }
    if (!model) return strdup("");

    FieldDef* field = NULL;
    for (int i = 0; i < model->field_count; i++) {
        if (strcmp(model->fields[i].name, col_name) == 0) {
            field = &model->fields[i];
            break;
        }
    }
    if (!field) return strdup("");

    char* sql = malloc(512);
    int pos = 0;
    pos += sprintf(sql, "ALTER TABLE %s ADD COLUMN %s %s", table_name, field->name, sql_type(field, g_orm_dialect));
    if (!field->nullable) pos += sprintf(sql + pos, " NOT NULL");
    if (field->unique) pos += sprintf(sql + pos, " UNIQUE");
    if (strlen(field->default_val) > 0) {
        pos += sprintf(sql + pos, " DEFAULT %s", field->default_val);
    }
    sql[pos] = '\0';
    return sql;
}

// Get model count
int32_t __orm_model_count(void) {
    return g_model_count;
}

// Get field count for a model
int32_t __orm_field_count(const char* table_name) {
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            return g_models[i].field_count;
        }
    }
    return 0;
}
