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
#include <stdarg.h>
#include "dynbuf.h"

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
    FIELD_FOREIGN_KEY,
    FIELD_UUID,        // UUID (PG native, VARCHAR(36) on MySQL)
    FIELD_ARRAY,       // Array type (PG only: TEXT[], INT[], etc.)
    FIELD_INET,        // INET type (PG only, VARCHAR(45) on MySQL)
    FIELD_DECIMAL,     // DECIMAL(precision, scale)
    FIELD_CUSTOM,      // pass-through: any native DB type string
    FIELD_GENERATED    // GENERATED ALWAYS AS (expr) STORED
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
    int precision;       // for DECIMAL
    int scale;           // for DECIMAL
    char default_val[256];
    char ref_table[128]; // for FOREIGN_KEY
    char ref_field[64];  // for FOREIGN_KEY
    char on_delete[32];  // for FOREIGN_KEY: CASCADE, PROTECT, SET_NULL, etc.
    char custom_type[128]; // for FIELD_CUSTOM and FIELD_ARRAY element type
    char choices[512];   // comma-separated choices for CHECK constraint
    char expression[256]; // for GENERATED: SQL expression
    char gen_sql_type[64]; // for GENERATED: output SQL type (VARCHAR(255), INTEGER, etc.)
} FieldDef;

typedef struct {
    char fields_csv[256]; // comma-separated field names
} ConstraintDef;

typedef struct {
    char name[128];      // table name
    FieldDef fields[64];
    int field_count;
    ConstraintDef unique_constraints[16];
    int unique_count;
    ConstraintDef indexes[16];
    int index_count;
    char composite_pk[256]; // comma-separated PK fields (empty = use auto PK)
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

// Add BigAutoField (BIGSERIAL primary key)
int32_t __orm_bigauto_field(const char* name) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name ? name : "id", sizeof(f->name) - 1);
    f->type = FIELD_AUTO; // reuse AUTO, SQL gen checks name/pk
    f->primary_key = 1;
    return 0;
}

// Add IntField
int32_t __orm_int_field(const char* name, int32_t has_default, int32_t default_val, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_INT;
    f->nullable = nullable;
    if (has_default) {
        snprintf(f->default_val, sizeof(f->default_val), "%d", default_val);
    }
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
int32_t __orm_bool_field(const char* name, int32_t has_default, int32_t default_val, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_BOOL;
    f->nullable = nullable;
    if (has_default) {
        snprintf(f->default_val, sizeof(f->default_val), "%s", default_val ? "TRUE" : "FALSE");
    }
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

// Add ForeignKey with on_delete
int32_t __orm_foreign_key(const char* name, const char* ref_table, const char* ref_field, const char* on_delete, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_FOREIGN_KEY;
    if (ref_table) strncpy(f->ref_table, ref_table, sizeof(f->ref_table) - 1);
    if (ref_field) strncpy(f->ref_field, ref_field, sizeof(f->ref_field) - 1);
    if (on_delete && strlen(on_delete) > 0) strncpy(f->on_delete, on_delete, sizeof(f->on_delete) - 1);
    else strncpy(f->on_delete, "CASCADE", sizeof(f->on_delete) - 1); // default
    f->nullable = nullable;
    return 0;
}

// ============================================================
// New Field Types
// ============================================================

// Add UUIDField
int32_t __orm_uuid_field(const char* name, int32_t nullable, int32_t unique) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_UUID;
    f->nullable = nullable;
    f->unique = unique;
    return 0;
}

// Add ArrayField (PG only — element type as string: "TEXT", "INTEGER", etc.)
int32_t __orm_array_field(const char* name, const char* element_type, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_ARRAY;
    f->nullable = nullable;
    if (element_type) strncpy(f->custom_type, element_type, sizeof(f->custom_type) - 1);
    else strncpy(f->custom_type, "TEXT", sizeof(f->custom_type) - 1);
    return 0;
}

// Add InetField (PG: INET, MySQL: VARCHAR(45))
int32_t __orm_inet_field(const char* name, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_INET;
    f->nullable = nullable;
    return 0;
}

// Add DecimalField
int32_t __orm_decimal_field(const char* name, int32_t precision, int32_t scale, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_DECIMAL;
    f->precision = precision > 0 ? precision : 10;
    f->scale = scale >= 0 ? scale : 2;
    f->nullable = nullable;
    return 0;
}

// Add CustomField — pass-through for ANY native DB type
int32_t __orm_custom_field(const char* name, const char* type_str, int32_t nullable) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_CUSTOM;
    f->nullable = nullable;
    if (type_str) strncpy(f->custom_type, type_str, sizeof(f->custom_type) - 1);
    return 0;
}

// ============================================================
// Meta Constraint Registration
// ============================================================

// Register a UNIQUE constraint on multiple columns
int32_t __orm_unique_constraint(const char* fields_csv) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->unique_count >= 16) return -1;
    ConstraintDef* c = &m->unique_constraints[m->unique_count++];
    memset(c, 0, sizeof(ConstraintDef));
    if (fields_csv) strncpy(c->fields_csv, fields_csv, sizeof(c->fields_csv) - 1);
    return 0;
}

// Register an INDEX on one or more columns
int32_t __orm_index(const char* fields_csv) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->index_count >= 16) return -1;
    ConstraintDef* c = &m->indexes[m->index_count++];
    memset(c, 0, sizeof(ConstraintDef));
    if (fields_csv) strncpy(c->fields_csv, fields_csv, sizeof(c->fields_csv) - 1);
    return 0;
}

// Register a composite primary key
int32_t __orm_composite_pk(const char* fields_csv) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (fields_csv) strncpy(m->composite_pk, fields_csv, sizeof(m->composite_pk) - 1);
    return 0;
}

// Set choices for the last registered field (generates CHECK constraint)
int32_t __orm_set_choices(const char* choices_csv) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count <= 0) return -1;
    FieldDef* f = &m->fields[m->field_count - 1]; // last field
    if (choices_csv) strncpy(f->choices, choices_csv, sizeof(f->choices) - 1);
    return 0;
}

// Add GeneratedField (computed column)
int32_t __orm_generated_field(const char* name, const char* expression, const char* sql_type) {
    if (g_current_model < 0) return -1;
    ModelDef* m = &g_models[g_current_model];
    if (m->field_count >= 64) return -1;
    FieldDef* f = &m->fields[m->field_count++];
    memset(f, 0, sizeof(FieldDef));
    strncpy(f->name, name, sizeof(f->name) - 1);
    f->type = FIELD_GENERATED;
    if (expression) strncpy(f->expression, expression, sizeof(f->expression) - 1);
    if (sql_type) strncpy(f->gen_sql_type, sql_type, sizeof(f->gen_sql_type) - 1);
    else strncpy(f->gen_sql_type, "TEXT", sizeof(f->gen_sql_type) - 1);
    return 0;
}

// ============================================================
// SQL Generation from Models
// ============================================================

// Get SQL type string for a field
static const char* sql_type(FieldDef* f, int dialect) {
    static char buf[128];
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
            if (dialect == 0) return "TIMESTAMPTZ";  // timezone-aware (Django-style)
            return "DATETIME";
        case FIELD_DATE: return "DATE";
        case FIELD_JSON:
            if (dialect == 0) return "JSONB";
            return "JSON";
        case FIELD_FOREIGN_KEY: return "INTEGER";
        case FIELD_UUID:
            if (dialect == 0) return "UUID";
            return "VARCHAR(36)";
        case FIELD_ARRAY:
            if (dialect == 0) {
                snprintf(buf, sizeof(buf), "%s[]", f->custom_type);
                return buf;
            }
            return "JSON"; // MySQL fallback: store arrays as JSON
        case FIELD_INET:
            if (dialect == 0) return "INET";
            return "VARCHAR(45)";
        case FIELD_DECIMAL:
            snprintf(buf, sizeof(buf), "DECIMAL(%d,%d)", f->precision, f->scale);
            return buf;
        case FIELD_CUSTOM:
            return f->custom_type;  // pass-through: user provides exact DB type
        case FIELD_GENERATED:
            return f->gen_sql_type; // output type from GeneratedField
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

        if (f->type != FIELD_AUTO && f->type != FIELD_GENERATED) {
            if (!f->nullable) pos += sprintf(sql + pos, " NOT NULL");
            if (f->unique) pos += sprintf(sql + pos, " UNIQUE");
            if (strlen(f->default_val) > 0 && f->type != FIELD_FOREIGN_KEY) {
                pos += sprintf(sql + pos, " DEFAULT %s", f->default_val);
            }
            if (f->auto_now_add) {
                if (g_orm_dialect == 0) pos += sprintf(sql + pos, " DEFAULT NOW()");
                else pos += sprintf(sql + pos, " DEFAULT CURRENT_TIMESTAMP");
            }
            // CHECK constraint from choices
            if (strlen(f->choices) > 0) {
                // Build CHECK (col IN ('val1', 'val2', ...))
                char choices_copy[512];
                strncpy(choices_copy, f->choices, sizeof(choices_copy) - 1);
                choices_copy[sizeof(choices_copy) - 1] = '\0';
                pos += sprintf(sql + pos, " CHECK (%s IN (", f->name);
                char* tok = strtok(choices_copy, ",");
                int first = 1;
                while (tok) {
                    if (!first) pos += sprintf(sql + pos, ", ");
                    // Determine if value is numeric
                    int is_numeric = 1;
                    for (char* p = tok; *p; p++) {
                        if ((*p < '0' || *p > '9') && *p != '-' && *p != '.') {
                            is_numeric = 0;
                            break;
                        }
                    }
                    if (is_numeric) {
                        pos += sprintf(sql + pos, "%s", tok);
                    } else {
                        pos += sprintf(sql + pos, "'%s'", tok);
                    }
                    first = 0;
                    tok = strtok(NULL, ",");
                }
                pos += sprintf(sql + pos, "))");
            }
        }

        // GENERATED ALWAYS AS for computed columns
        if (f->type == FIELD_GENERATED) {
            pos += sprintf(sql + pos, " GENERATED ALWAYS AS (%s) STORED", f->expression);
        }

        if (f->type == FIELD_FOREIGN_KEY) {
            pos += sprintf(sql + pos, " REFERENCES %s(%s)", f->ref_table, f->ref_field);
            if (strlen(f->on_delete) > 0) {
                pos += sprintf(sql + pos, " ON DELETE %s", f->on_delete);
            }
        }
    }

    // Append UNIQUE constraints from Meta
    for (int i = 0; i < model->unique_count; i++) {
        // Convert comma-separated field names to SQL: UNIQUE (field1, field2)
        char fields_copy[256];
        strncpy(fields_copy, model->unique_constraints[i].fields_csv, sizeof(fields_copy) - 1);
        fields_copy[sizeof(fields_copy) - 1] = '\0';
        // Replace commas with ", " for pretty output
        pos += sprintf(sql + pos, ",\n  UNIQUE (%s)", fields_copy);
    }

    // Append composite primary key
    if (strlen(model->composite_pk) > 0) {
        pos += sprintf(sql + pos, ",\n  PRIMARY KEY (%s)", model->composite_pk);
    }

    pos += sprintf(sql + pos, "\n)");
    if (g_orm_dialect == 1) {
        pos += sprintf(sql + pos, " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4");
    }

    // Append CREATE INDEX statements from Meta
    for (int i = 0; i < model->index_count; i++) {
        char fields_copy[256];
        strncpy(fields_copy, model->indexes[i].fields_csv, sizeof(fields_copy) - 1);
        fields_copy[sizeof(fields_copy) - 1] = '\0';
        // Generate index name: idx_tablename_field1_field2
        char idx_name[256];
        snprintf(idx_name, sizeof(idx_name), "idx_%s_%s", model->name, fields_copy);
        // Replace commas with underscores in index name
        for (char* p = idx_name; *p; p++) {
            if (*p == ',') *p = '_';
        }
        pos += sprintf(sql + pos, ";\nCREATE INDEX IF NOT EXISTS %s ON %s (%s)", idx_name, model->name, fields_copy);
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

// Get the expected SQL type for a model field (for migration diffing)
char* __orm_column_type_sql(const char* table_name, const char* col_name) {
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

    // Return the SQL type (same function used by CREATE TABLE)
    return strdup(sql_type(field, g_orm_dialect));
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

// Get model name by index (for migration system)
const char* __orm_model_name(int32_t index) {
    if (index < 0 || index >= g_model_count) return "";
    return g_models[index].name;
}

// Get field name by table name and field index (for migration system)
const char* __orm_field_name(const char* table_name, int32_t field_index) {
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            if (field_index < 0 || field_index >= g_models[i].field_count) return "";
            return g_models[i].fields[field_index].name;
        }
    }
    return "";
}

// Get FK metadata for a field — used by select_related
// Returns 1 if found (field is a FK), 0 otherwise
int32_t __orm_fk_info(const char* table_name, const char* field_name,
                      char* ref_table_out, int ref_table_size,
                      char* ref_field_out, int ref_field_size) {
    if (!table_name || !field_name) return 0;

    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) != 0) continue;
        for (int j = 0; j < g_models[i].field_count; j++) {
            FieldDef* f = &g_models[i].fields[j];
            if (f->type == FIELD_FOREIGN_KEY && strcmp(f->name, field_name) == 0) {
                if (ref_table_out) strncpy(ref_table_out, f->ref_table, ref_table_size - 1);
                if (ref_field_out) strncpy(ref_field_out, f->ref_field, ref_field_size - 1);
                return 1;
            }
        }
    }
    return 0;
}

// ============================================================
// Field Spec DSL — Convert ORM fields to portable spec strings
//
// Used by makemigrations to emit db.op_* calls instead of raw SQL.
// Format: "name:type[:modifier1[:modifier2[...]]]"
//
// Examples:
//   "id:auto"
//   "email:varchar(200):unique"
//   "author_id:fk(users.id):cascade"
//   "active:bool:default(TRUE)"
//   "created_at:datetime:auto_now"
// ============================================================

char* __orm_field_spec(const char* table_name, int32_t field_index) {
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            model = &g_models[i];
            break;
        }
    }
    if (!model || field_index < 0 || field_index >= model->field_count)
        return strdup("");

    FieldDef* f = &model->fields[field_index];
    char buf[512];
    int pos = 0;

    // name:type
    pos += sprintf(buf + pos, "%s:", f->name);

    switch (f->type) {
        case FIELD_AUTO:     pos += sprintf(buf + pos, "auto"); break;
        case FIELD_INT:      pos += sprintf(buf + pos, "int"); break;
        case FIELD_BIGINT:   pos += sprintf(buf + pos, "bigint"); break;
        case FIELD_VARCHAR:  pos += sprintf(buf + pos, "varchar(%d)", f->max_length); break;
        case FIELD_TEXT:     pos += sprintf(buf + pos, "text"); break;
        case FIELD_BOOL:     pos += sprintf(buf + pos, "bool"); break;
        case FIELD_FLOAT:    pos += sprintf(buf + pos, "float"); break;
        case FIELD_DOUBLE:   pos += sprintf(buf + pos, "double"); break;
        case FIELD_DATETIME: pos += sprintf(buf + pos, "datetime"); break;
        case FIELD_DATE:     pos += sprintf(buf + pos, "date"); break;
        case FIELD_JSON:     pos += sprintf(buf + pos, "json"); break;
        case FIELD_UUID:     pos += sprintf(buf + pos, "uuid"); break;
        case FIELD_INET:     pos += sprintf(buf + pos, "inet"); break;
        case FIELD_DECIMAL:  pos += sprintf(buf + pos, "decimal(%d,%d)", f->precision, f->scale); break;
        case FIELD_ARRAY:    pos += sprintf(buf + pos, "array(%s)", f->custom_type); break;
        case FIELD_CUSTOM:   pos += sprintf(buf + pos, "%s", f->custom_type); break;
        case FIELD_FOREIGN_KEY:
            pos += sprintf(buf + pos, "fk(%s.%s)", f->ref_table, f->ref_field);
            break;
        case FIELD_GENERATED:
            pos += sprintf(buf + pos, "generated(%s,%s)", f->expression, f->gen_sql_type);
            break;
        default: pos += sprintf(buf + pos, "text"); break;
    }

    // Auto types are always PK NOT NULL — no modifiers needed
    if (f->type == FIELD_AUTO) {
        return strdup(buf);
    }

    // Modifiers
    if (f->nullable)  pos += sprintf(buf + pos, ":nullable");
    if (f->unique)    pos += sprintf(buf + pos, ":unique");
    if (f->default_val[0])
        pos += sprintf(buf + pos, ":default(%s)", f->default_val);
    if (f->auto_now || f->auto_now_add)
        pos += sprintf(buf + pos, ":auto_now");

    // FK on_delete action
    if (f->type == FIELD_FOREIGN_KEY && f->on_delete[0]) {
        // Lowercase the on_delete for spec format
        char lower[32] = {0};
        for (int i = 0; f->on_delete[i] && i < 31; i++) {
            lower[i] = (f->on_delete[i] >= 'A' && f->on_delete[i] <= 'Z')
                ? f->on_delete[i] + 32 : f->on_delete[i];
        }
        pos += sprintf(buf + pos, ":%s", lower);
    }

    return strdup(buf);
}

// ============================================================
// Model Instance Hydration + Save
//
// Hydrate: after a query, read row N into a per-model cache
//          so user can access field values by name.
//
// Save: INSERT or UPDATE a model instance. If the PK field
//       has a nonzero value, UPDATE; otherwise INSERT.
//
// Django equivalents:
//   user = User.objects.get(id=1)  →  hydrate
//   user.name = "Alice"            →  set_field
//   user.save()                    →  save
// ============================================================

// Instance field cache — stores field values for the "current" instance
#define MAX_INSTANCE_FIELDS 64
static char g_instance_table[128];
static char g_instance_fields[MAX_INSTANCE_FIELDS][64];
static char g_instance_values[MAX_INSTANCE_FIELDS][4096];
static char g_instance_original[MAX_INSTANCE_FIELDS][4096]; // snapshot for dirty tracking
static int  g_instance_dirty[MAX_INSTANCE_FIELDS];           // 1 = field changed
static int  g_instance_field_count = 0;
static int  g_instance_pk_value = 0;  // 0 = new (INSERT), >0 = existing (UPDATE)

// Extern: access query result values from crud.c
extern char* __db_get_field_by(int32_t row, const char* col_name);
extern int32_t __db_col_count(void);

// __orm_hydrate — populate instance cache from query result row
// After a fetch, call this to load row data into the instance cache.
// table_name: the model table (e.g. "users")
// row: row index from the query result
// Returns: field count loaded, or -1 on error
int32_t __orm_hydrate(const char* table_name, int32_t row) {
    if (!table_name) return -1;

    // Find model definition
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            model = &g_models[i];
            break;
        }
    }
    if (!model) return -1;

    // Reset instance state
    strncpy(g_instance_table, table_name, sizeof(g_instance_table) - 1);
    g_instance_table[sizeof(g_instance_table) - 1] = '\0';
    g_instance_field_count = 0;
    g_instance_pk_value = 0;
    memset(g_instance_dirty, 0, sizeof(g_instance_dirty));

    // Load each field from the result row
    for (int i = 0; i < model->field_count && i < MAX_INSTANCE_FIELDS; i++) {
        FieldDef* f = &model->fields[i];
        strncpy(g_instance_fields[i], f->name, 63);
        g_instance_fields[i][63] = '\0';

        char* val = __db_get_field_by(row, f->name);
        if (val) {
            strncpy(g_instance_values[i], val, 4095);
            g_instance_values[i][4095] = '\0';
            // Snapshot original for dirty tracking
            strncpy(g_instance_original[i], val, 4095);
            g_instance_original[i][4095] = '\0';
            free(val);
        } else {
            g_instance_values[i][0] = '\0';
            g_instance_original[i][0] = '\0';
        }
        g_instance_dirty[i] = 0;

        // Track PK for save() INSERT vs UPDATE detection
        if (f->primary_key || f->type == FIELD_AUTO) {
            g_instance_pk_value = atoi(g_instance_values[i]);
        }

        g_instance_field_count++;
    }

    return g_instance_field_count;
}

// __orm_instance_get — get a field value from the hydrated instance
// Returns heap-allocated string (caller must free)
char* __orm_instance_get(const char* field_name) {
    if (!field_name) return strdup("");
    for (int i = 0; i < g_instance_field_count; i++) {
        if (strcmp(g_instance_fields[i], field_name) == 0) {
            return strdup(g_instance_values[i]);
        }
    }
    return strdup("");
}

// __orm_instance_set — set a field value on the hydrated instance
// This modifies the in-memory cache, not the DB.
int32_t __orm_instance_set(const char* field_name, const char* value) {
    if (!field_name || !value) return -1;

    // Update existing field
    for (int i = 0; i < g_instance_field_count; i++) {
        if (strcmp(g_instance_fields[i], field_name) == 0) {
            // Mark dirty if value actually changed
            if (strcmp(g_instance_values[i], value) != 0) {
                g_instance_dirty[i] = 1;
            }
            strncpy(g_instance_values[i], value, 4095);
            g_instance_values[i][4095] = '\0';
            return 0;
        }
    }

    // Add new field if space available
    if (g_instance_field_count < MAX_INSTANCE_FIELDS) {
        int idx = g_instance_field_count++;
        g_instance_dirty[idx] = 1; // new field is always dirty
        strncpy(g_instance_fields[idx], field_name, 63);
        g_instance_fields[idx][63] = '\0';
        strncpy(g_instance_values[idx], value, 4095);
        g_instance_values[idx][4095] = '\0';
        return 0;
    }

    return -1;
}

// __orm_instance_pk — return the PK value of the hydrated instance
// 0 = new instance (no PK yet), >0 = existing row
int32_t __orm_instance_pk(void) {
    return g_instance_pk_value;
}

// __orm_instance_table — return the table name of the hydrated instance
char* __orm_instance_table(void) {
    return strdup(g_instance_table);
}

// Extern: set_field and QuerySet ops from crud.c
extern int32_t __qs_set_field(const char* key, const char* val);
extern int32_t __qs_save(void);
extern int32_t __qs_filter(const char* key, const char* val);
// Saving an instance writes every changed field in one statement, which is
// __qs_save_update. It is NOT __qs_update: that takes (col, val) and writes a
// single column. This was previously declared here as `__qs_update(void)` and
// called with no arguments, so the two-argument definition in crud.c read
// whatever happened to be in the argument registers as `col` and `val` — the
// declaration mismatch is invisible to the linker and the UPDATE path was
// corrupt rather than merely missing.
extern int32_t __qs_save_update(void);
extern int32_t __db_execute_stmt(const char* sql);
extern void    __qs_clear(void);

// Forward declarations
void __orm_instance_clear(void);

// __orm_save — persist the hydrated instance to the database
// If PK > 0: UPDATE table SET field1=val1, ... WHERE pk=pk_value
// If PK == 0: INSERT into table (field1, ...) VALUES (val1, ...)
//
// Uses the QuerySet infrastructure under the hood.
// Returns: 0 on success, -1 on error
int32_t __orm_save(void) {
    if (g_instance_table[0] == '\0' || g_instance_field_count == 0) return -1;

    // Find the model to identify PK + auto_now fields
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, g_instance_table) == 0) {
            model = &g_models[i];
            break;
        }
    }
    if (!model) return -1;

    // Find PK field name
    const char* pk_field = "id";
    for (int i = 0; i < model->field_count; i++) {
        if (model->fields[i].primary_key || model->fields[i].type == FIELD_AUTO) {
            pk_field = model->fields[i].name;
            break;
        }
    }

    // Auto-populate auto_now fields (update timestamps on every save)
    for (int i = 0; i < model->field_count; i++) {
        if (model->fields[i].auto_now) {
            // Set the field to NOW() placeholder — the DB handles actual value
            __orm_instance_set(model->fields[i].name, "NOW()");
        }
    }

    // Fire pre_save signal
    extern int32_t __orm_fire_signal(const char* table, int32_t signal_type);
    int32_t sig_rc = __orm_fire_signal(g_instance_table, 0); // SIGNAL_PRE_SAVE
    if (sig_rc != 0) return -1; // handler vetoed

    // Clear QuerySet and set table
    extern int32_t __qs_reset(const char* table);
    __qs_reset(g_instance_table);

    int32_t result;
    if (g_instance_pk_value > 0) {
        // UPDATE: only dirty fields (if dirty tracking enabled)
        int any_dirty = 0;
        for (int i = 0; i < g_instance_field_count; i++) {
            if (g_instance_dirty[i]) { any_dirty = 1; break; }
        }

        for (int i = 0; i < g_instance_field_count; i++) {
            if (strcmp(g_instance_fields[i], pk_field) == 0) continue;
            // If we have dirty tracking data and this field isn't dirty, skip
            if (any_dirty && !g_instance_dirty[i]) continue;
            // Skip auto/generated fields
            int skip = 0;
            for (int j = 0; j < model->field_count; j++) {
                if (strcmp(model->fields[j].name, g_instance_fields[i]) == 0) {
                    if (model->fields[j].type == FIELD_AUTO ||
                        model->fields[j].type == FIELD_GENERATED) {
                        skip = 1;
                    }
                    break;
                }
            }
            if (skip) continue;
            __qs_set_field(g_instance_fields[i], g_instance_values[i]);
        }

        char pk_str[32];
        snprintf(pk_str, sizeof(pk_str), "%d", g_instance_pk_value);
        __qs_filter(pk_field, pk_str);
        result = __qs_save_update();
    } else {
        // INSERT: set all non-auto fields
        for (int i = 0; i < g_instance_field_count; i++) {
            int skip = 0;
            for (int j = 0; j < model->field_count; j++) {
                if (strcmp(model->fields[j].name, g_instance_fields[i]) == 0) {
                    if (model->fields[j].type == FIELD_AUTO ||
                        model->fields[j].type == FIELD_GENERATED) {
                        skip = 1;
                    }
                    break;
                }
            }
            if (skip) continue;
            __qs_set_field(g_instance_fields[i], g_instance_values[i]);
        }
        result = __qs_save();
    }

    // Fire post_save signal
    if (result == 0) {
        __orm_fire_signal(g_instance_table, 1); // SIGNAL_POST_SAVE
        // Reset dirty flags after successful save
        memset(g_instance_dirty, 0, sizeof(g_instance_dirty));
        // Update originals to current values
        for (int i = 0; i < g_instance_field_count; i++) {
            strncpy(g_instance_original[i], g_instance_values[i], 4095);
            g_instance_original[i][4095] = '\0';
        }
    }

    return result;
}

// __orm_instance_delete — delete the hydrated instance from the database
// Django equivalent: user.delete()
int32_t __orm_instance_delete(void) {
    if (g_instance_table[0] == '\0' || g_instance_pk_value <= 0) return -1;

    // Find model for PK field name
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, g_instance_table) == 0) {
            model = &g_models[i]; break;
        }
    }
    if (!model) return -1;

    const char* pk_field = "id";
    for (int i = 0; i < model->field_count; i++) {
        if (model->fields[i].primary_key || model->fields[i].type == FIELD_AUTO) {
            pk_field = model->fields[i].name; break;
        }
    }

    // Fire pre_delete signal
    extern int32_t __orm_fire_signal(const char* table, int32_t signal_type);
    int32_t sig_rc = __orm_fire_signal(g_instance_table, 2); // SIGNAL_PRE_DELETE
    if (sig_rc != 0) return -1;

    extern int32_t __qs_reset(const char* table);
    extern int32_t __qs_delete(void);
    __qs_reset(g_instance_table);

    char pk_str[32];
    snprintf(pk_str, sizeof(pk_str), "%d", g_instance_pk_value);
    __qs_filter(pk_field, pk_str);

    int32_t result = __qs_delete();

    if (result == 0) {
        __orm_fire_signal(g_instance_table, 3); // SIGNAL_POST_DELETE
        // Clear instance after successful delete
        __orm_instance_clear();
    }

    return result;
}

// __orm_refresh_from_db — re-fetch the current instance from DB
// Django equivalent: user.refresh_from_db()
int32_t __orm_refresh_from_db(void) {
    if (g_instance_table[0] == '\0' || g_instance_pk_value <= 0) return -1;

    // Find PK field name
    ModelDef* model = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, g_instance_table) == 0) {
            model = &g_models[i]; break;
        }
    }
    if (!model) return -1;

    const char* pk_field = "id";
    for (int i = 0; i < model->field_count; i++) {
        if (model->fields[i].primary_key || model->fields[i].type == FIELD_AUTO) {
            pk_field = model->fields[i].name; break;
        }
    }

    // Query: SELECT * FROM table WHERE pk = pk_value
    extern int32_t __qs_reset(const char* table);
    extern int32_t __qs_fetch(void);
    __qs_reset(g_instance_table);

    char pk_str[32];
    snprintf(pk_str, sizeof(pk_str), "%d", g_instance_pk_value);
    __qs_filter(pk_field, pk_str);

    int32_t rc = __qs_fetch();
    if (rc <= 0) return -1;

    // Re-hydrate from row 0
    return __orm_hydrate(g_instance_table, 0);
}

// __orm_is_dirty — check if any field has been modified since hydration
int32_t __orm_is_dirty(void) {
    for (int i = 0; i < g_instance_field_count; i++) {
        if (g_instance_dirty[i]) return 1;
    }
    return 0;
}

// __orm_dirty_fields — return comma-separated list of dirty field names
char* __orm_dirty_fields(void) {
    char buf[2048] = {0};
    int pos = 0;
    for (int i = 0; i < g_instance_field_count; i++) {
        if (g_instance_dirty[i]) {
            if (pos > 0) pos += sprintf(buf + pos, ",");
            pos += sprintf(buf + pos, "%s", g_instance_fields[i]);
        }
    }
    return strdup(buf);
}

// __orm_instance_clear — reset instance state
void __orm_instance_clear(void) {
    g_instance_table[0] = '\0';
    g_instance_field_count = 0;
    g_instance_pk_value = 0;
    memset(g_instance_dirty, 0, sizeof(g_instance_dirty));
}

// ============================================================
// OneToOneField — ForeignKey with UNIQUE constraint
// Django equivalent: OneToOneField(User, on_delete=CASCADE)
// ============================================================

int32_t __orm_one_to_one_field(const char* name, const char* ref_table,
                                const char* ref_field, const char* on_delete,
                                int32_t nullable) {
    // Register as FK with unique=1
    int32_t rc = __orm_foreign_key(name, ref_table, ref_field, on_delete, nullable);
    if (rc != 0) return rc;
    // Set unique on the just-added field
    if (g_current_model >= 0) {
        ModelDef* m = &g_models[g_current_model];
        if (m->field_count > 0) {
            m->fields[m->field_count - 1].unique = 1;
        }
    }
    return 0;
}

// ============================================================
// Signals — pre/post save/delete hooks
//
// Django equivalent:
//   @receiver(pre_save, sender=User)
//   def my_handler(sender, instance, **kwargs): ...
//
// In Desi, signals are C function pointer callbacks registered
// per (table, event) pair. Up to 8 handlers per slot.
// ============================================================

typedef enum {
    SIGNAL_PRE_SAVE,
    SIGNAL_POST_SAVE,
    SIGNAL_PRE_DELETE,
    SIGNAL_POST_DELETE,
    SIGNAL_COUNT
} SignalType;

// Signal callback: receives table name, returns 0 to proceed or -1 to abort
typedef int32_t (*signal_fn)(const char* table);

#define MAX_SIGNAL_HANDLERS 8
#define MAX_SIGNAL_TABLES   16

typedef struct {
    char table[128];
    signal_fn handlers[SIGNAL_COUNT][MAX_SIGNAL_HANDLERS];
    int handler_count[SIGNAL_COUNT];
} SignalSlot;

static SignalSlot g_signals[MAX_SIGNAL_TABLES];
static int g_signal_count = 0;

// Find or create signal slot for a table
static SignalSlot* signal_slot_for(const char* table) {
    for (int i = 0; i < g_signal_count; i++) {
        if (strcmp(g_signals[i].table, table) == 0) return &g_signals[i];
    }
    if (g_signal_count >= MAX_SIGNAL_TABLES) return NULL;
    SignalSlot* s = &g_signals[g_signal_count++];
    memset(s, 0, sizeof(SignalSlot));
    strncpy(s->table, table, sizeof(s->table) - 1);
    return s;
}

// __orm_connect_signal — register a signal handler
// signal_type: 0=pre_save, 1=post_save, 2=pre_delete, 3=post_delete
// Returns 0 on success, -1 on error
int32_t __orm_connect_signal(const char* table, int32_t signal_type, signal_fn handler) {
    if (!table || !handler || signal_type < 0 || signal_type >= SIGNAL_COUNT) return -1;
    SignalSlot* s = signal_slot_for(table);
    if (!s) return -1;
    if (s->handler_count[signal_type] >= MAX_SIGNAL_HANDLERS) return -1;
    s->handlers[signal_type][s->handler_count[signal_type]++] = handler;
    return 0;
}

// Fire all handlers for a signal — returns -1 if any handler returns -1
int32_t __orm_fire_signal(const char* table, int32_t signal_type) {
    if (!table || signal_type < 0 || signal_type >= SIGNAL_COUNT) return 0;

    for (int i = 0; i < g_signal_count; i++) {
        if (strcmp(g_signals[i].table, table) != 0) continue;
        for (int j = 0; j < g_signals[i].handler_count[signal_type]; j++) {
            int32_t rc = g_signals[i].handlers[signal_type][j](table);
            if (rc != 0) return -1;  // handler vetoed the operation
        }
    }
    return 0;
}

// ============================================================
// Field Validation
//
// Django equivalent:
//   name = CharField(max_length=100, validators=[MinLengthValidator(2)])
//
// Validators are registered per (table, field) and checked
// before save operations. Returns error messages on failure.
// ============================================================

typedef struct {
    char table[128];
    char field[64];
    int has_min_length;
    int min_length;
    int has_max_length;
    int max_length;
    int has_min_value;
    int64_t min_value;
    int has_max_value;
    int64_t max_value;
    char regex_pattern[256];  // reserved for future regex support
} FieldValidator;

#define MAX_VALIDATORS 64
static FieldValidator g_validators[MAX_VALIDATORS];
static int g_validator_count = 0;

// __orm_add_validator — register validation rules for a field
int32_t __orm_add_validator(const char* table, const char* field,
                            int32_t min_len, int32_t max_len,
                            int64_t min_val, int64_t max_val) {
    if (!table || !field || g_validator_count >= MAX_VALIDATORS) return -1;

    FieldValidator* v = &g_validators[g_validator_count++];
    memset(v, 0, sizeof(FieldValidator));
    strncpy(v->table, table, sizeof(v->table) - 1);
    strncpy(v->field, field, sizeof(v->field) - 1);

    if (min_len > 0) { v->has_min_length = 1; v->min_length = min_len; }
    if (max_len > 0) { v->has_max_length = 1; v->max_length = max_len; }
    if (min_val != 0) { v->has_min_value = 1; v->min_value = min_val; }
    if (max_val != 0) { v->has_max_value = 1; v->max_value = max_val; }

    return 0;
}

// __orm_validate_field — validate a single field value
// Returns 0 on success, writes error message to err_buf on failure
int32_t __orm_validate_field(const char* table, const char* field,
                              const char* value, char* err_buf, int err_size) {
    if (!table || !field || !value) return 0;

    for (int i = 0; i < g_validator_count; i++) {
        FieldValidator* v = &g_validators[i];
        if (strcmp(v->table, table) != 0 || strcmp(v->field, field) != 0) continue;

        int slen = (int)strlen(value);

        if (v->has_min_length && slen < v->min_length) {
            snprintf(err_buf, err_size, "%s: value too short (min %d, got %d)",
                     field, v->min_length, slen);
            return -1;
        }
        if (v->has_max_length && slen > v->max_length) {
            snprintf(err_buf, err_size, "%s: value too long (max %d, got %d)",
                     field, v->max_length, slen);
            return -1;
        }
        if (v->has_min_value) {
            int64_t num = strtoll(value, NULL, 10);
            if (num < v->min_value) {
                snprintf(err_buf, err_size, "%s: value too small (min %lld, got %lld)",
                         field, (long long)v->min_value, (long long)num);
                return -1;
            }
        }
        if (v->has_max_value) {
            int64_t num = strtoll(value, NULL, 10);
            if (num > v->max_value) {
                snprintf(err_buf, err_size, "%s: value too large (max %lld, got %lld)",
                         field, (long long)v->max_value, (long long)num);
                return -1;
            }
        }
    }
    return 0;
}

// __orm_validate_instance — validate all fields of the current hydrated instance
// Returns 0 if valid, -1 on first failure. Writes error to err_buf.
int32_t __orm_validate_instance(char* err_buf, int err_size) {
    if (g_instance_table[0] == '\0') return 0;

    for (int i = 0; i < g_instance_field_count; i++) {
        int32_t rc = __orm_validate_field(
            g_instance_table, g_instance_fields[i],
            g_instance_values[i], err_buf, err_size);
        if (rc != 0) return -1;
    }
    return 0;
}

// __orm_validate_instance_str — Desi-friendly wrapper
// Returns heap-allocated error string (empty = valid)
char* __orm_validate_instance_str(void) {
    char err[1024] = {0};
    int32_t rc = __orm_validate_instance(err, sizeof(err));
    if (rc != 0) return strdup(err);
    return strdup("");
}

// ============================================================
// Many-to-Many Relationships
//
// Django equivalent:
//   class Article(models.Model):
//       tags = models.ManyToManyField(Tag)
//
// Creates a junction table: article_tags (article_id, tag_id)
//
// Usage:
//   db.m2m_add("article_tags", "1", "5")    # article 1 ↔ tag 5
//   db.m2m_remove("article_tags", "1", "5") # remove link
//   db.m2m_clear("article_tags", "1")       # remove all links for article 1
//   db.m2m_all("article_tags", "1")         # get all tag_ids for article 1
// ============================================================

typedef struct {
    char name[128];          // junction table name (e.g. "article_tags")
    char from_table[128];    // source model table (e.g. "articles")
    char from_col[64];       // source FK column (e.g. "article_id")
    char to_table[128];      // target model table (e.g. "tags")
    char to_col[64];         // target FK column (e.g. "tag_id")
    char through_table[128]; // custom through table (empty = auto)
} M2MDef;

#define MAX_M2M 32
static M2MDef g_m2m[MAX_M2M];
static int g_m2m_count = 0;

// __orm_m2m — register a many-to-many relationship
// Creates a junction table definition
int32_t __orm_m2m(const char* from_table, const char* to_table,
                  const char* junction_name) {
    if (!from_table || !to_table || g_m2m_count >= MAX_M2M) return -1;

    M2MDef* m = &g_m2m[g_m2m_count++];
    memset(m, 0, sizeof(M2MDef));

    // Auto-generate junction name if not provided
    if (junction_name && junction_name[0] != '\0') {
        strncpy(m->name, junction_name, sizeof(m->name) - 1);
    } else {
        snprintf(m->name, sizeof(m->name), "%s_%s", from_table, to_table);
    }

    strncpy(m->from_table, from_table, sizeof(m->from_table) - 1);
    strncpy(m->to_table, to_table, sizeof(m->to_table) - 1);

    // Auto-generate FK column names: table_id
    snprintf(m->from_col, sizeof(m->from_col), "%s_id", from_table);
    snprintf(m->to_col, sizeof(m->to_col), "%s_id", to_table);

    return g_m2m_count - 1;
}

// __orm_m2m_create_sql — generate CREATE TABLE for a junction table
char* __orm_m2m_create_sql(const char* junction_name) {
    M2MDef* m = NULL;
    for (int i = 0; i < g_m2m_count; i++) {
        if (strcmp(g_m2m[i].name, junction_name) == 0) {
            m = &g_m2m[i];
            break;
        }
    }
    if (!m) return strdup("");

    char* sql = malloc(1024);
    int pos = 0;

    if (g_orm_dialect == 0) {
        // PostgreSQL
        pos += sprintf(sql + pos,
            "CREATE TABLE IF NOT EXISTS %s (\n"
            "  id SERIAL PRIMARY KEY,\n"
            "  %s INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,\n"
            "  %s INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,\n"
            "  UNIQUE (%s, %s)\n"
            ")",
            m->name,
            m->from_col, m->from_table,
            m->to_col, m->to_table,
            m->from_col, m->to_col);
    } else {
        // MySQL
        pos += sprintf(sql + pos,
            "CREATE TABLE IF NOT EXISTS %s (\n"
            "  id INT AUTO_INCREMENT PRIMARY KEY,\n"
            "  %s INT NOT NULL,\n"
            "  %s INT NOT NULL,\n"
            "  UNIQUE (%s, %s),\n"
            "  FOREIGN KEY (%s) REFERENCES %s(id) ON DELETE CASCADE,\n"
            "  FOREIGN KEY (%s) REFERENCES %s(id) ON DELETE CASCADE\n"
            ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
            m->name,
            m->from_col, m->to_col,
            m->from_col, m->to_col,
            m->from_col, m->from_table,
            m->to_col, m->to_table);
    }
    sql[pos] = '\0';
    return sql;
}

// Extern: query execution from crud.c
extern int32_t __db_exec_param(const char* sql, const char** params, int param_count);

// __orm_m2m_add — add a link: INSERT INTO junction (from_col, to_col) VALUES (from_id, to_id)
int32_t __orm_m2m_add(const char* junction_name, const char* from_id, const char* to_id) {
    M2MDef* m = NULL;
    for (int i = 0; i < g_m2m_count; i++) {
        if (strcmp(g_m2m[i].name, junction_name) == 0) { m = &g_m2m[i]; break; }
    }
    if (!m || !from_id || !to_id) return -1;

    char sql[512];
    if (g_orm_dialect == 0) {
        snprintf(sql, sizeof(sql),
            "INSERT INTO %s (%s, %s) VALUES (%s, %s) ON CONFLICT DO NOTHING",
            m->name, m->from_col, m->to_col, from_id, to_id);
    } else {
        snprintf(sql, sizeof(sql),
            "INSERT IGNORE INTO %s (%s, %s) VALUES (%s, %s)",
            m->name, m->from_col, m->to_col, from_id, to_id);
    }
    return __db_execute_stmt(sql);
}

// __orm_m2m_remove — remove a link
int32_t __orm_m2m_remove(const char* junction_name, const char* from_id, const char* to_id) {
    M2MDef* m = NULL;
    for (int i = 0; i < g_m2m_count; i++) {
        if (strcmp(g_m2m[i].name, junction_name) == 0) { m = &g_m2m[i]; break; }
    }
    if (!m || !from_id || !to_id) return -1;

    char sql[512];
    snprintf(sql, sizeof(sql),
        "DELETE FROM %s WHERE %s = %s AND %s = %s",
        m->name, m->from_col, from_id, m->to_col, to_id);
    return __db_execute_stmt(sql);
}

// __orm_m2m_clear — remove all links for a given source ID
int32_t __orm_m2m_clear(const char* junction_name, const char* from_id) {
    M2MDef* m = NULL;
    for (int i = 0; i < g_m2m_count; i++) {
        if (strcmp(g_m2m[i].name, junction_name) == 0) { m = &g_m2m[i]; break; }
    }
    if (!m || !from_id) return -1;

    char sql[512];
    snprintf(sql, sizeof(sql),
        "DELETE FROM %s WHERE %s = %s",
        m->name, m->from_col, from_id);
    return __db_execute_stmt(sql);
}

// __orm_m2m_all — query all related IDs, returns count via fetch
// Sets up QuerySet to: SELECT to_col FROM junction WHERE from_col = from_id
int32_t __orm_m2m_all(const char* junction_name, const char* from_id) {
    M2MDef* m = NULL;
    for (int i = 0; i < g_m2m_count; i++) {
        if (strcmp(g_m2m[i].name, junction_name) == 0) { m = &g_m2m[i]; break; }
    }
    if (!m || !from_id) return -1;

    // Use raw SQL via QuerySet for consistent result handling
    char sql[512];
    snprintf(sql, sizeof(sql),
        "SELECT %s FROM %s WHERE %s = %s",
        m->to_col, m->name, m->from_col, from_id);

    extern int32_t __qs_raw(const char* sql);
    return __qs_raw(sql);
}

// __orm_m2m_count — return number of registered M2M relationships
int32_t __orm_m2m_count(void) {
    return g_m2m_count;
}

// __orm_m2m_name — get junction table name by index
const char* __orm_m2m_name(int32_t index) {
    if (index < 0 || index >= g_m2m_count) return "";
    return g_m2m[index].name;
}

// __orm_m2m_set — replace all links atomically: clear + add each
// Django equivalent: article.tags.set([1, 2, 3])
// id_list: comma-separated IDs (e.g. "1,2,3")
int32_t __orm_m2m_set(const char* junction_name, const char* from_id,
                       const char* id_list) {
    if (!junction_name || !from_id || !id_list) return -1;

    // Step 1: clear all existing links
    int32_t rc = __orm_m2m_clear(junction_name, from_id);
    if (rc != 0) return rc;

    // Step 2: add each ID from the comma-separated list
    char buf[4096];
    strncpy(buf, id_list, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    char* tok = strtok(buf, ",");
    while (tok) {
        // Trim whitespace
        while (*tok == ' ') tok++;
        char* end = tok + strlen(tok) - 1;
        while (end > tok && *end == ' ') *end-- = '\0';

        if (*tok != '\0') {
            rc = __orm_m2m_add(junction_name, from_id, tok);
            if (rc != 0) return rc;
        }
        tok = strtok(NULL, ",");
    }

    return 0;
}

// ============================================================
// QuerySet .reverse() — flip current ordering
// Django equivalent: qs.reverse()
// ============================================================

extern int32_t __qs_reverse(void);

// ============================================================
// Abstract Model Inheritance
//
// Django equivalent:
//   class TimestampMixin(models.Model):
//       created_at = DateTimeField(auto_now_add=True)
//       class Meta:
//           abstract = True
//
//   class Article(TimestampMixin):
//       title = CharField(max_length=200)
//
// In Desi, abstract models are registered normally but marked
// abstract. Their fields are copied into child models.
// ============================================================

// Mark a model as abstract (it won't generate a table)
static int g_abstract_models[MAX_MODELS];

int32_t __orm_set_abstract(const char* table_name) {
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            g_abstract_models[i] = 1;
            return 0;
        }
    }
    return -1;
}

// Check if a model is abstract
int32_t __orm_is_abstract(const char* table_name) {
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) {
            return g_abstract_models[i];
        }
    }
    return 0;
}

// __orm_inherit — copy all fields from parent model into child model
// Django equivalent: class Child(Parent) where Parent is abstract
int32_t __orm_inherit(const char* child_table, const char* parent_table) {
    if (!child_table || !parent_table) return -1;

    ModelDef* parent = NULL;
    ModelDef* child = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, parent_table) == 0) parent = &g_models[i];
        if (strcmp(g_models[i].name, child_table) == 0) child = &g_models[i];
    }
    if (!parent || !child) return -1;

    // Copy parent fields into child (skip if child already has them)
    for (int i = 0; i < parent->field_count; i++) {
        FieldDef* pf = &parent->fields[i];

        // Check if child already has this field (avoid duplicates)
        int exists = 0;
        for (int j = 0; j < child->field_count; j++) {
            if (strcmp(child->fields[j].name, pf->name) == 0) {
                exists = 1;
                break;
            }
        }

        if (!exists && child->field_count < 64) {
            memcpy(&child->fields[child->field_count++], pf, sizeof(FieldDef));
        }
    }

    // Copy constraints
    for (int i = 0; i < parent->unique_count && child->unique_count < 16; i++) {
        memcpy(&child->unique_constraints[child->unique_count++],
               &parent->unique_constraints[i], sizeof(ConstraintDef));
    }
    for (int i = 0; i < parent->index_count && child->index_count < 16; i++) {
        memcpy(&child->indexes[child->index_count++],
               &parent->indexes[i], sizeof(ConstraintDef));
    }

    return 0;
}

// __orm_create_all_m2m_sql — generate CREATE TABLE for all M2M junction tables
// Returns semicolon-separated SQL statements
char* __orm_create_all_m2m_sql(void) {
    if (g_m2m_count == 0) return strdup("");

    char* buf = malloc(g_m2m_count * 1024);
    int pos = 0;
    buf[0] = '\0';

    for (int i = 0; i < g_m2m_count; i++) {
        char* sql = __orm_m2m_create_sql(g_m2m[i].name);
        if (sql && sql[0] != '\0') {
            if (pos > 0) pos += sprintf(buf + pos, ";\n");
            pos += sprintf(buf + pos, "%s", sql);
        }
        free(sql);
    }

    buf[pos] = '\0';
    return buf;
}

// ============================================================
// Custom Managers — reusable named query scopes
//
// Django equivalent:
//   class ActiveUserManager(Manager):
//       def get_queryset(self):
//           return super().get_queryset().filter(is_active=True)
//   User.active = ActiveUserManager()
//
// Desi:
//   db.register_manager("users", "active", "is_active__exact=true")
//   db.use_manager("users", "active")  # auto-applies filter
// ============================================================

#define MAX_MANAGERS 64
typedef struct {
    char table[64];
    char name[64];
    char filter_key[128];  // lookup, e.g. "is_active__exact"
    char filter_val[256];  // value, e.g. "true"
} ManagerDef;

static ManagerDef g_managers[MAX_MANAGERS];
static int g_manager_count = 0;

// Register a manager with a predefined filter
int32_t __orm_register_manager(const char* table, const char* name,
                                const char* filter_expr) {
    if (!table || !name || !filter_expr) return -1;
    if (g_manager_count >= MAX_MANAGERS) return -1;

    ManagerDef* m = &g_managers[g_manager_count++];
    strncpy(m->table, table, sizeof(m->table) - 1);
    strncpy(m->name, name, sizeof(m->name) - 1);

    // Parse "key=value" format
    char buf[384];
    strncpy(buf, filter_expr, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';

    char* eq = strchr(buf, '=');
    if (eq) {
        *eq = '\0';
        strncpy(m->filter_key, buf, sizeof(m->filter_key) - 1);
        strncpy(m->filter_val, eq + 1, sizeof(m->filter_val) - 1);
    } else {
        strncpy(m->filter_key, filter_expr, sizeof(m->filter_key) - 1);
        m->filter_val[0] = '\0';
    }

    return 0;
}

// Use a registered manager — sets up QuerySet with pre-defined filter
int32_t __orm_use_manager(const char* table, const char* name) {
    if (!table || !name) return -1;

    for (int i = 0; i < g_manager_count; i++) {
        if (strcmp(g_managers[i].table, table) == 0 &&
            strcmp(g_managers[i].name, name) == 0) {
            extern int32_t __qs_reset(const char* table);
            extern int32_t __qs_filter(const char* lookup, const char* val);
            __qs_reset(table);
            __qs_filter(g_managers[i].filter_key, g_managers[i].filter_val);
            return 0;
        }
    }

    return -1; // manager not found
}

// ============================================================
// Reverse Relations — query from FK target back to FK source
//
// Django equivalent:
//   user.posts.all()  →  Post.objects.filter(author_id=user.pk)
//
// Desi:
//   db.reverse_query("posts", "author_id", "1")
//   # Builds: SELECT * FROM posts WHERE author_id = 1
// ============================================================

int32_t __orm_reverse_query(const char* source_table, const char* fk_field,
                             const char* pk_value) {
    if (!source_table || !fk_field || !pk_value) return -1;

    extern int32_t __qs_reset(const char* table);
    extern int32_t __qs_filter(const char* lookup, const char* val);
    extern int32_t __qs_fetch(void);

    __qs_reset(source_table);
    __qs_filter(fk_field, pk_value);
    return __qs_fetch();
}

// __orm_reverse_count — count reverse related objects
int32_t __orm_reverse_count(const char* source_table, const char* fk_field,
                             const char* pk_value) {
    if (!source_table || !fk_field || !pk_value) return -1;

    extern int32_t __qs_reset(const char* table);
    extern int32_t __qs_filter(const char* lookup, const char* val);
    extern int32_t __qs_count(void);

    __qs_reset(source_table);
    __qs_filter(fk_field, pk_value);
    return __qs_count();
}

// ============================================================
// Multi-Table Inheritance
//
// Django equivalent:
//   class Place(Model):
//       name = CharField()
//   class Restaurant(Place):
//       serves_pizza = BooleanField()
//   # Creates both tables: places + restaurants (with place_ptr_id FK)
//
// Desi:
//   db.model("places")
//   db.auto_field("id")
//   db.char_field("name", 100, 0, 0)
//
//   db.model("restaurants")
//   db.auto_field("id")
//   db.multi_table_inherit("restaurants", "places")
//   db.boolean_field("serves_pizza", 0, 0)
//   # Auto-adds: place_ptr_id INTEGER NOT NULL UNIQUE REFERENCES places(id)
// ============================================================

int32_t __orm_multi_table_inherit(const char* child_table, const char* parent_table) {
    if (!child_table || !parent_table) return -1;

    ModelDef* parent = NULL;
    ModelDef* child = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, parent_table) == 0) parent = &g_models[i];
        if (strcmp(g_models[i].name, child_table) == 0) child = &g_models[i];
    }
    if (!parent || !child) return -1;

    // Find parent's PK field name
    const char* parent_pk = "id";
    for (int i = 0; i < parent->field_count; i++) {
        if (parent->fields[i].primary_key) {
            parent_pk = parent->fields[i].name;
            break;
        }
    }

    // Auto-add parent_ptr_id FK field to child
    char fk_name[128];
    snprintf(fk_name, sizeof(fk_name), "%s_ptr_id", parent_table);

    // Check if already exists
    for (int i = 0; i < child->field_count; i++) {
        if (strcmp(child->fields[i].name, fk_name) == 0) return 0; // already added
    }

    if (child->field_count < 64) {
        FieldDef* f = &child->fields[child->field_count++];
        memset(f, 0, sizeof(FieldDef));
        strncpy(f->name, fk_name, sizeof(f->name) - 1);
        f->type = FIELD_FOREIGN_KEY;
        f->nullable = 0;
        f->unique = 1; // OneToOne semantics
        strncpy(f->ref_table, parent_table, sizeof(f->ref_table) - 1);
        strncpy(f->ref_field, parent_pk, sizeof(f->ref_field) - 1);
        strncpy(f->on_delete, "CASCADE", sizeof(f->on_delete) - 1);
    }

    return 0;
}

// ============================================================
// Proxy Models — same table, different model name
//
// Django equivalent:
//   class UnmanagedUser(User):
//       class Meta:
//           proxy = True
//       objects = ActiveManager()
//
// Desi:
//   db.proxy_model("active_users", "users")
//   db.register_manager("active_users", "default", "is_active__exact=true")
// ============================================================

#define MAX_PROXIES 16
typedef struct {
    char proxy_name[128];
    char base_table[128];
} ProxyDef;

static ProxyDef g_proxies[MAX_PROXIES];
static int g_proxy_count = 0;

int32_t __orm_proxy_model(const char* proxy_name, const char* base_table) {
    if (!proxy_name || !base_table || g_proxy_count >= MAX_PROXIES) return -1;

    // Verify base table exists in model registry
    int found = 0;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, base_table) == 0) { found = 1; break; }
    }
    if (!found) return -1;

    ProxyDef* p = &g_proxies[g_proxy_count++];
    strncpy(p->proxy_name, proxy_name, sizeof(p->proxy_name) - 1);
    strncpy(p->base_table, base_table, sizeof(p->base_table) - 1);
    return 0;
}

// Resolve proxy name to base table name
const char* __orm_resolve_proxy(const char* name) {
    if (!name) return name;
    for (int i = 0; i < g_proxy_count; i++) {
        if (strcmp(g_proxies[i].proxy_name, name) == 0) {
            return g_proxies[i].base_table;
        }
    }
    return name; // not a proxy — return as-is
}

// ============================================================
// Built-in Audit Trail — auto-history table
//
// Django equivalent: django-simple-history
//   class User(Model):
//       history = HistoricalRecords()
//   user.history.all()  →  UserHistoricalChange objects
//
// Desi:
//   db.enable_audit("users")
//   # Creates users_history table automatically
//   # save()/delete() auto-log to history
//   db.audit_log("users", "1")  →  history entries
// ============================================================

#define MAX_AUDITED 32
static char g_audited_tables[MAX_AUDITED][128];
static int g_audited_count = 0;

int32_t __orm_enable_audit(const char* table_name) {
    if (!table_name || g_audited_count >= MAX_AUDITED) return -1;

    // Check not already audited
    for (int i = 0; i < g_audited_count; i++) {
        if (strcmp(g_audited_tables[i], table_name) == 0) return 0;
    }

    strncpy(g_audited_tables[g_audited_count++], table_name, 127);
    return 0;
}

// Check if a table has auditing enabled
int32_t __orm_is_audited(const char* table_name) {
    if (!table_name) return 0;
    for (int i = 0; i < g_audited_count; i++) {
        if (strcmp(g_audited_tables[i], table_name) == 0) return 1;
    }
    return 0;
}

// Create the history table for an audited model.
// Schema: id, action, record_id, changed_at, changed_by, + all model fields
char* __orm_create_audit_table_sql(const char* table_name) {
    if (!table_name) return strdup("");

    ModelDef* m = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) { m = &g_models[i]; break; }
    }
    if (!m) return strdup("");

    DynBuf sql;
    dynbuf_init(&sql, 1024);

    char hist_table[256];
    snprintf(hist_table, sizeof(hist_table), "%s_history", table_name);

    if (g_orm_dialect == 0) { // PG
        dynbuf_appendf(&sql,
            "CREATE TABLE IF NOT EXISTS %s ("
            "history_id SERIAL PRIMARY KEY, "
            "action VARCHAR(10) NOT NULL, "
            "record_id INTEGER NOT NULL, "
            "changed_at TIMESTAMPTZ DEFAULT NOW(), "
            "changed_by VARCHAR(128) DEFAULT ''",
            hist_table);
    } else { // MySQL
        dynbuf_appendf(&sql,
            "CREATE TABLE IF NOT EXISTS %s ("
            "history_id INT AUTO_INCREMENT PRIMARY KEY, "
            "action VARCHAR(10) NOT NULL, "
            "record_id INTEGER NOT NULL, "
            "changed_at DATETIME DEFAULT CURRENT_TIMESTAMP, "
            "changed_by VARCHAR(128) DEFAULT ''",
            hist_table);
    }

    // Add snapshot of all model fields
    for (int i = 0; i < m->field_count; i++) {
        FieldDef* f = &m->fields[i];
        // Skip PK (already captured as record_id)
        if (f->primary_key) continue;

        // Get column type SQL for this field
        char* col_type = __orm_column_type_sql(table_name, f->name);
        if (col_type && col_type[0]) {
            dynbuf_appendf(&sql, ", %s %s", f->name, col_type);
        }
        if (col_type) free(col_type);
    }

    dynbuf_append(&sql, ")");
    if (g_orm_dialect != 0) {
        dynbuf_append(&sql, " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4");
    }

    char* result = sql.data;
    sql.data = NULL;
    return result;
}

// Log an action to the audit history table
// action: "INSERT", "UPDATE", or "DELETE"
int32_t __orm_audit_record(const char* table_name, const char* action,
                            const char* record_id) {
    if (!table_name || !action || !record_id) return -1;
    if (!__orm_is_audited(table_name)) return 0; // not audited, skip silently

    extern int32_t __db_execute_stmt(const char* sql);

    // Build INSERT into history table with current instance field values
    DynBuf sql;
    dynbuf_init(&sql, 512);

    char hist_table[256];
    snprintf(hist_table, sizeof(hist_table), "%s_history", table_name);

    // Find model
    ModelDef* m = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) { m = &g_models[i]; break; }
    }

    dynbuf_appendf(&sql,
        "INSERT INTO %s (action, record_id, changed_by",
        hist_table);

    // Add field names
    DynBuf vals;
    dynbuf_init(&vals, 256);
    dynbuf_appendf(&vals, "'%s', %s, ''", action, record_id);

    if (m) {
        for (int i = 0; i < m->field_count; i++) {
            if (m->fields[i].primary_key) continue;
            dynbuf_appendf(&sql, ", %s", m->fields[i].name);

            // Get current value from hydrated instance
            extern char* __orm_instance_get(const char* field);
            char* val = __orm_instance_get(m->fields[i].name);
            if (val && val[0]) {
                // Escape single quotes
                char escaped[1024];
                int j = 0;
                for (int k = 0; val[k] && j < 1020; k++) {
                    if (val[k] == '\'') escaped[j++] = '\'';
                    escaped[j++] = val[k];
                }
                escaped[j] = '\0';
                dynbuf_appendf(&vals, ", '%s'", escaped);
            } else {
                dynbuf_append(&vals, ", NULL");
            }
            if (val) free(val);
        }
    }

    dynbuf_appendf(&sql, ") VALUES (%s)", vals.data);

    int32_t result = __db_execute_stmt(sql.data);

    dynbuf_free(&sql);
    dynbuf_free(&vals);
    return result;
}

// Query audit history for a record
int32_t __orm_audit_log(const char* table_name, const char* record_id) {
    if (!table_name || !record_id) return -1;

    extern int32_t __qs_reset(const char* table);
    extern int32_t __qs_filter(const char* lookup, const char* val);
    extern int32_t __qs_order_by(const char* field);
    extern int32_t __qs_fetch(void);

    char hist_table[256];
    snprintf(hist_table, sizeof(hist_table), "%s_history", table_name);

    __qs_reset(hist_table);
    __qs_filter("record_id", record_id);
    __qs_order_by("-changed_at"); // newest first
    return __qs_fetch();
}

// ============================================================
// Time-Travel Queries — query historical state
//
// Django equivalent: django-simple-history
//   user.history.as_of(datetime(2024, 1, 1))
//
// Desi:
//   db.as_of("users", "1", "2024-01-01 00:00:00")
//   # Returns the state of record #1 as it was at that time
// ============================================================

int32_t __orm_as_of(const char* table_name, const char* record_id,
                     const char* timestamp) {
    if (!table_name || !record_id || !timestamp) return -1;

    extern int32_t __db_query_exec(const char* sql);

    char hist_table[256];
    snprintf(hist_table, sizeof(hist_table), "%s_history", table_name);

    DynBuf sql;
    dynbuf_init(&sql, 512);
    dynbuf_appendf(&sql,
        "SELECT * FROM %s WHERE record_id = %s AND changed_at <= '%s' "
        "AND action != 'DELETE' "
        "ORDER BY changed_at DESC LIMIT 1",
        hist_table, record_id, timestamp);

    int32_t result = __db_query_exec(sql.data);
    dynbuf_free(&sql);
    return result;
}

// ============================================================
// Runtime Field Validation
//
// Checks if a field name exists in a model's field registry.
// Returns 1 if valid, 0 if unknown field.
// Also provides "did you mean?" suggestion via Levenshtein distance.
// ============================================================

static int levenshtein(const char* s, const char* t) {
    int slen = (int)strlen(s);
    int tlen = (int)strlen(t);
    if (slen == 0) return tlen;
    if (tlen == 0) return slen;

    // Use a single-row DP approach
    int* prev = (int*)malloc((tlen + 1) * sizeof(int));
    int* curr = (int*)malloc((tlen + 1) * sizeof(int));

    for (int j = 0; j <= tlen; j++) prev[j] = j;

    for (int i = 1; i <= slen; i++) {
        curr[0] = i;
        for (int j = 1; j <= tlen; j++) {
            int cost = (s[i-1] == t[j-1]) ? 0 : 1;
            int del = prev[j] + 1;
            int ins = curr[j-1] + 1;
            int sub = prev[j-1] + cost;
            curr[j] = del < ins ? (del < sub ? del : sub) : (ins < sub ? ins : sub);
        }
        int* tmp = prev; prev = curr; curr = tmp;
    }

    int result = prev[tlen];
    free(prev);
    free(curr);
    return result;
}

int32_t __orm_check_field(const char* table_name, const char* field_name) {
    if (!table_name || !field_name) return 0;

    // Resolve proxy
    table_name = __orm_resolve_proxy(table_name);

    ModelDef* m = NULL;
    for (int i = 0; i < g_model_count; i++) {
        if (strcmp(g_models[i].name, table_name) == 0) { m = &g_models[i]; break; }
    }
    if (!m) {
        fprintf(stderr, "[orm] WARNING: unknown model '%s'\n", table_name);
        return 0;
    }

    // Check exact match
    for (int i = 0; i < m->field_count; i++) {
        if (strcmp(m->fields[i].name, field_name) == 0) return 1;
    }

    // Field not found — find closest match
    int best_dist = 999;
    const char* best_match = NULL;
    for (int i = 0; i < m->field_count; i++) {
        int dist = levenshtein(field_name, m->fields[i].name);
        if (dist < best_dist) {
            best_dist = dist;
            best_match = m->fields[i].name;
        }
    }

    if (best_match && best_dist <= 3) {
        fprintf(stderr, "[orm] WARNING: unknown field '%s' on model '%s' — did you mean '%s'?\n",
            field_name, table_name, best_match);
    } else {
        fprintf(stderr, "[orm] WARNING: unknown field '%s' on model '%s'\n",
            field_name, table_name);
    }

    return 0;
}

// ============================================================
// QuerySet Result Caching
//
// Cache the last query result so re-accessing doesn't re-query DB.
// Cache is invalidated on any filter/order_by/reset change.
//
// Django equivalent: QuerySets cache after first evaluation.
// ============================================================

#define QS_CACHE_MAX_ROWS 1000
#define QS_CACHE_MAX_COLS 32
#define QS_CACHE_VAL_SIZE 256

static struct {
    char values[QS_CACHE_MAX_ROWS][QS_CACHE_MAX_COLS][QS_CACHE_VAL_SIZE];
    int rows;
    int cols;
    int valid;
    char table[128];
    char where_hash[256]; // simple hash of filter state
} g_qs_cache = {0};

void __qs_cache_invalidate(void) {
    g_qs_cache.valid = 0;
    g_qs_cache.rows = 0;
    g_qs_cache.cols = 0;
}

int32_t __qs_cache_store(int rows, int cols) {
    if (rows > QS_CACHE_MAX_ROWS || cols > QS_CACHE_MAX_COLS) {
        g_qs_cache.valid = 0;
        return -1; // too large to cache
    }

    extern char* __db_get_value_at(int32_t row, int32_t col);

    for (int r = 0; r < rows; r++) {
        for (int c = 0; c < cols; c++) {
            char* val = __db_get_value_at(r, c);
            if (val) {
                strncpy(g_qs_cache.values[r][c], val, QS_CACHE_VAL_SIZE - 1);
                g_qs_cache.values[r][c][QS_CACHE_VAL_SIZE - 1] = '\0';
            } else {
                g_qs_cache.values[r][c][0] = '\0';
            }
        }
    }

    g_qs_cache.rows = rows;
    g_qs_cache.cols = cols;
    g_qs_cache.valid = 1;
    return 0;
}

int32_t __qs_cache_is_valid(void) {
    return g_qs_cache.valid;
}

const char* __qs_cache_get(int row, int col) {
    if (!g_qs_cache.valid || row >= g_qs_cache.rows || col >= g_qs_cache.cols) {
        return "";
    }
    return g_qs_cache.values[row][col];
}

int32_t __qs_cache_rows(void) {
    return g_qs_cache.valid ? g_qs_cache.rows : 0;
}

int32_t __qs_cache_cols(void) {
    return g_qs_cache.valid ? g_qs_cache.cols : 0;
}
