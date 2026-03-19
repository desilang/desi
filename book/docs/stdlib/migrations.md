# Migrations

Desi's migration system automatically creates and alters tables to match your ORM models.

**No auto-migration** — you always call `db.migrate()` explicitly.

## Quick Start

```desi
import db

# 1. Connect
db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")

# 2. Define models
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.char_field("email", 255, 0, 1)
db.datetime_field("created_at", 0, 1, 0)

# 3. Apply
let applied = db.migrate()
print(f"Applied {str(applied)} migration(s)")
```

## How It Works

1. `db.migrate()` compares your registered models against the actual database schema
2. **New table?** → Runs `CREATE TABLE`
3. **New column?** → Runs `ALTER TABLE ADD COLUMN`
4. **No changes?** → Does nothing (safe to call repeatedly)
5. Every operation is recorded in `_desi_migrations` with rollback SQL

## Rollback

```desi
# Undo the last migration version
let rolled = db.rollback()
print(f"Rolled back {str(rolled)} operation(s)")
```

Rollback executes in **reverse order** — tables with foreign keys are dropped before their parent tables.

## Status

```desi
let status = db.migration_status()
print(status)
```

Output:
```
Migration Status (version 1)
---
  v1  users           create       2026-03-19 10:59:43
  v1  posts           create       2026-03-19 10:59:43
```

## Schema Introspection

```desi
if db.table_exists("users") == 1:
    print("users table exists")
```

## Tracking Table

Migrations are tracked in `_desi_migrations` (created automatically):

| Column | Type | Description |
|---|---|---|
| `id` | SERIAL/AUTO_INCREMENT | Primary key |
| `version` | INTEGER | Migration version number |
| `table_name` | VARCHAR(128) | Table affected |
| `action` | VARCHAR(20) | `create` or `add_column` |
| `sql_applied` | TEXT | SQL that was executed |
| `sql_rollback` | TEXT | SQL to undo this operation |
| `applied_at` | TIMESTAMPTZ/DATETIME | When applied |

## Full Example

```desi
import db

db.connect("postgres", "localhost", 5432, "myapp", "user", "pass")

# Define models
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.char_field("email", 255, 0, 1)
db.bool_field("active", 1, 0)
db.datetime_field("created_at", 0, 1, 0)

db.model("posts")
db.auto_field("id")
db.char_field("title", 200, 0, 0)
db.text_field("body", 0)
db.foreign_key("author_id", "users", "id", 0)
db.datetime_field("published_at", 0, 0, 1)

# Apply migrations
db.migrate()

# Check status
print(db.migration_status())

# ... later, if you need to undo ...
db.rollback()
```

## Works with Both Databases

```desi
# PostgreSQL
db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")
db.migrate()  # Uses SERIAL, TIMESTAMPTZ, JSONB, etc.

# MySQL
db.connect("mysql", "localhost", 3306, "mydb", "root", "pass")
db.migrate()  # Uses AUTO_INCREMENT, DATETIME, JSON, etc.
```

## See Also

- [db](db.md) — Query builder, ORM, unified API
- [PostgreSQL](postgres.md) — PG-specific types
- [MySQL](mysql.md) — MySQL-specific types
