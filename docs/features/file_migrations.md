# File-Based Migration System — Internals

**Status**: ✅ Implemented  
**Since**: v0.11  
**Related**: [ORM Models](orm_models.md), [Multi-Database](multi_database.md)

---

## Overview

The file-based migration system generates, applies, and manages `.desi` migration files using an **operation-based DSL**. Migration files are database-agnostic — SQL is generated at apply-time based on the active driver (PostgreSQL or MySQL).

### Key Design Decisions

1. **Operation-based DSL over raw SQL** — Migration files use `db.op_create_table()` instead of `db.execute("CREATE TABLE ...")`. SQL is generated at apply time for the active dialect.
2. **Field spec strings** — Portable colon-delimited format: `name:varchar(100):unique` → parsed by `field_spec_to_sql()`.
3. **Multi-app with topo-sort** — Django-style app labels and `# Depends:` headers. `migrate_all()` builds a dependency graph and applies in topological order.
4. **Transactional DDL** — PG: BEGIN/COMMIT wrapping. MySQL: best-effort (DDL is auto-committed).
5. **SQL injection prevention** — All filename/value interpolations use `sql_escape_alloc()`.

## Architecture

```
migrate.c (2,400 LOC)
├── Data Structures
│   ├── DynBuf      — growable string buffer (replaces static char arrays)
│   ├── StrList     — growable string list (directory listings)
│   ├── OpList      — growable migration operation list
│   ├── MigOp       — single operation (action, table, column, specs)
│   ├── MigNode     — dependency graph node (app, file, depends)
│   └── MigNodeList — growable node list for topo-sort
│
├── Core Functions
│   ├── ensure_tracking()         — create/auto-migrate _desi_migrations table
│   ├── extract_app_label()       — dir path → app name
│   ├── migration_applied_for_app() — check if migration already applied
│   ├── record_migration_with_app() — insert tracking record
│   ├── column_exists()           — information_schema check
│   └── sql_escape_alloc()        — SQL string escaping
│
├── Field Spec Engine
│   ├── field_spec_to_sql()       — "name:type:mod" → dialect SQL
│   ├── op_create_table_sql()     — specs → CREATE TABLE
│   ├── op_drop_table_sql()       — → DROP TABLE IF EXISTS
│   ├── op_add_column_sql()       — spec → ALTER TABLE ADD COLUMN
│   ├── op_drop_column_sql()      — → ALTER TABLE DROP COLUMN
│   ├── op_alter_column_sql()     — → ALTER COLUMN TYPE / MODIFY COLUMN
│   ├── op_rename_column_sql()    — → RENAME COLUMN
│   └── op_add_index_sql()        — → CREATE INDEX IF NOT EXISTS
│
├── Parser
│   ├── parse_section()           — extract forward/rollback SQL from migration file
│   ├── try_parse_op_call()       — dispatch db.op_*() patterns
│   ├── extract_string_arg()      — handle """ and " quoted args
│   └── skip_arg_sep()            — skip commas/whitespace between args
│
├── Public API
│   ├── __db_makemigrations(dir)  — generate migration from ORM diff
│   ├── __db_migrate_dir(dir)     — apply pending migrations (single app)
│   ├── __db_rollback_dir(dir)    — undo last migration (single app)
│   ├── __db_migration_status_dir(dir) — show applied/pending status
│   ├── __db_migrate_all(dirs)    — cross-app migration with topo-sort
│   └── __migrate_reset_state()   — reset cached driver state
│
├── Header Parser
│   └── parse_header_field()      — extract "# Key: value" from migration content
│
└── Dependency Resolution
    ├── MigNode / MigNodeList     — dependency graph data structures
    ├── find_node()               — lookup node by app_label + filename prefix
    ├── topo_visit()              — DFS topological sort with cycle detection
    └── mignodelist_free()        — cleanup
```

## Field Spec Format

The `field_spec_to_sql()` function parses colon-delimited specs into dialect-specific SQL:

```
Input:  "email:varchar(255):unique:nullable"
PG:     "email VARCHAR(255) UNIQUE"
MySQL:  "email VARCHAR(255) UNIQUE"

Input:  "author_id:fk(users.id):cascade"
PG:     "author_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE"
MySQL:  "author_id INTEGER NOT NULL, CONSTRAINT fk_author_id FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE"

Input:  "created_at:datetime:auto_now"
PG:     "created_at TIMESTAMPTZ DEFAULT NOW()"
MySQL:  "created_at DATETIME DEFAULT CURRENT_TIMESTAMP"
```

### Supported Types

| Spec Type | PG | MySQL |
|---|---|---|
| `auto` | `SERIAL PRIMARY KEY` | `INT AUTO_INCREMENT PRIMARY KEY` |
| `bigauto` | `BIGSERIAL PRIMARY KEY` | `BIGINT AUTO_INCREMENT PRIMARY KEY` |
| `int` | `INTEGER` | `INTEGER` |
| `bigint` | `BIGINT` | `BIGINT` |
| `varchar(N)` | `VARCHAR(N)` | `VARCHAR(N)` |
| `text` | `TEXT` | `TEXT` |
| `bool` | `BOOLEAN` | `TINYINT(1)` |
| `float` | `REAL` | `FLOAT` |
| `double` | `DOUBLE PRECISION` | `DOUBLE` |
| `datetime` | `TIMESTAMPTZ` | `DATETIME` |
| `date` | `DATE` | `DATE` |
| `json` | `JSONB` | `JSON` |
| `uuid` | `UUID` | `CHAR(36)` |
| `inet` | `INET` | `VARCHAR(45)` |
| `decimal(P,S)` | `DECIMAL(P,S)` | `DECIMAL(P,S)` |
| `array(T)` | `T[]` | `JSON` |
| `fk(table.col)` | `REFERENCES table(col)` | `FOREIGN KEY ... REFERENCES` |

### Supported Modifiers

| Modifier | Effect |
|---|---|
| `nullable` | Omit `NOT NULL` |
| `unique` | Add `UNIQUE` |
| `default(val)` | Add `DEFAULT val` |
| `auto_now` | `DEFAULT NOW()` / `CURRENT_TIMESTAMP` |
| `auto_now_add` | Same as `auto_now` |
| `cascade` | `ON DELETE CASCADE` (FK) |
| `set_null` / `setnull` | `ON DELETE SET NULL` (FK) |
| `protect` | `ON DELETE RESTRICT` (FK) |
| `restrict` | `ON DELETE RESTRICT` (FK) |
| `set_default` / `setdefault` | `ON DELETE SET DEFAULT` (FK) |
| `no_action` / `noaction` | `ON DELETE NO ACTION` (FK) |

## Parser: Dual-Format Support

`parse_section()` handles both formats in the same file:

```python
# New: Operation-based (generates SQL at parse-time)
db.op_create_table("users", """id:auto
name:varchar(100)""")

# Legacy: Raw SQL (passed through)
db.execute("""CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL
)""")
```

The parser uses `try_parse_op_call()` to dispatch:
- Recognizes `db.op_create_table`, `db.op_drop_table`, `db.op_add_column`, etc.
- Uses `extract_string_arg()` for both `"..."` and `"""..."""` quoting
- Falls through to raw `db.execute()` handling for legacy format

## Makemigrations: Diff Engine

`__db_makemigrations(dir)` compares the ORM model registry against existing migration files:

1. **Scan ORM registry** — `__orm_model_count()`, `__orm_model_name()`, `__orm_field_count()`, `__orm_field_spec()`
2. **Scan existing files** — read all `NNNN_*.desi` files from the migration directory
3. **Parse existing tables** — extract `op_create_table` / `op_add_column` from existing migration files
4. **Diff** — identify new tables, new columns, dropped tables, dropped columns, type changes
5. **Generate** — write new migration file with `db.op_*` operations + rollback section

The diff populates `MigOp.field_specs` for `create_table`/`add_column`, and `MigOp.old_type`/`new_type` for `alter_type`.

## Multi-App: Dependency Resolution

### Data Structures

```c
typedef struct MigNode {
    char* app_label;     // "accounts"
    char* filename;      // "0001_initial.desi"
    char* dir;           // "accounts/migrations"
    char* depends;       // "accounts.0001_initial" or "none"
    int visited;         // 0=unvisited, 1=in-progress, 2=done
    struct MigNode** deps;
    int dep_count;
} MigNode;
```

### Algorithm

1. Build graph: for each directory, scan migration files → create `MigNode` per file
2. Parse headers: read `# App:` and `# Depends:` from each file
3. Resolve edges: `find_node()` matches `app.filename_prefix` → dependency link
4. Topological sort: DFS with 3-state visited flag (cycle detection)
5. Apply in order: skip already-applied, parse+execute remaining

### Cycle Detection

If `topo_visit()` encounters a node with `visited == 1` (in-progress), it means we've found a back-edge → circular dependency. Returns -1 with diagnostic.

## Tracking Table Schema

```sql
CREATE TABLE IF NOT EXISTS _desi_migrations (
    id SERIAL PRIMARY KEY,         -- PG
    app_label VARCHAR(128) NOT NULL DEFAULT '',
    filename VARCHAR(256) NOT NULL,
    applied_sql TEXT,
    rollback_sql TEXT,
    applied_at TIMESTAMPTZ DEFAULT NOW()
);
```

Auto-migration: if an existing table lacks the `app_label` column, `ensure_tracking()` detects this via `information_schema` and runs `ALTER TABLE ADD COLUMN`.

## File Map

| File | Role |
|---|---|
| `compiler/runtime/db/migrate.c` | Core migration engine (2,400 LOC) |
| `compiler/runtime/db/orm.c` | `__orm_field_spec()` — model → spec string |
| `compiler/runtime/db/dispatch.c` | `__migrate_reset_state()` call |
| `compiler/lib/db.desi` | Public API bindings |

## Testing

Verified with standalone C unit tests:
- Operations: 47/47 (field_spec_to_sql, op_*_sql, parse_section)
- Dependencies: 37/37 (extract_app_label, parse_header_field, find_node, topo_sort, cycle_detection)

Integration tests:
- `examples/460_migration_test.desi` — inline migration
- `examples/462_file_migrations_test.desi` — file-based migration
- `examples/463_migration_ops_test.desi` — op-based DSL
