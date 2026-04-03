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
