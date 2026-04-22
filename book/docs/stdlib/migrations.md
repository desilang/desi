# Migrations

Desi's migration system automatically creates and alters tables to match your ORM models. Supports both inline migrations and Django-style file-based migrations with multi-app dependency resolution.

**No auto-migration** — you always call `db.migrate()` or `db.migrate_dir()` explicitly.

## Quick Start (Inline)

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

## File-Based Migrations

For real projects, use the file-based system — it generates portable migration files that can be version-controlled and shared.

### Generate Migrations

```desi
import db

db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")

# Register models
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.char_field("email", 255, 0, 1)

# Generate migration file
db.makemigrations("migrations")
# → Creates migrations/0001_initial.desi
```

### Generated Migration File

The system generates portable, operation-based migration files:

```python
# Migration: 0001_initial
# Generated: 2026-04-22
# App: myapp
# Depends: none
#
# Operations:
#   create_table(users)

import db

def forward():
	db.op_create_table("users", """id:auto
name:varchar(100)
email:varchar(255):unique""")

def rollback():
	db.op_drop_table("users")
```

### Apply & Rollback

```desi
# Apply all pending migrations in a directory
let applied = db.migrate_dir("migrations")
print(f"Applied {str(applied)} migration(s)")

# Undo the last applied migration
let rolled = db.rollback_dir("migrations")
print(f"Rolled back {str(rolled)} operation(s)")
```

### Check Status

```desi
let status = db.migration_status_dir("migrations")
print(status)
```

Output:
```
Migration Status for 'myapp':
  ✓ 0001_initial.desi           (applied)
  · 0002_add_email.desi         (pending)
```

## Operation-Based DSL

Migration files use portable operations instead of raw SQL. The correct SQL is generated at apply-time based on your database driver.

| Operation | Purpose | Example |
|---|---|---|
| `db.op_create_table(name, specs)` | Create a new table | `db.op_create_table("users", "id:auto\nname:varchar(100)")` |
| `db.op_drop_table(name)` | Drop a table | `db.op_drop_table("users")` |
| `db.op_add_column(table, spec)` | Add a column | `db.op_add_column("users", "email:varchar(255):unique")` |
| `db.op_drop_column(table, col)` | Drop a column | `db.op_drop_column("users", "email")` |
| `db.op_alter_column(table, col, type)` | Change column type | `db.op_alter_column("users", "name", "TEXT")` |
| `db.op_rename_column(table, old, new)` | Rename a column | `db.op_rename_column("users", "name", "full_name")` |
| `db.op_add_index(table, cols)` | Create an index | `db.op_add_index("users", "email")` |
| `db.op_run_sql(sql)` | Run raw SQL | `db.op_run_sql("CREATE EXTENSION pgcrypto")` |

### Field Spec Format

Field specs use a colon-delimited format: `name:type:modifier1:modifier2`

```
id:auto                         → SERIAL PRIMARY KEY (PG) / INT AUTO_INCREMENT (MySQL)
name:varchar(100)               → VARCHAR(100) NOT NULL
email:varchar(255):unique       → VARCHAR(255) NOT NULL UNIQUE
bio:text:nullable               → TEXT
score:int:default(0)            → INTEGER NOT NULL DEFAULT 0
active:bool:default(TRUE)       → BOOLEAN NOT NULL DEFAULT TRUE
author_id:fk(users.id):cascade  → INTEGER REFERENCES users(id) ON DELETE CASCADE
created_at:datetime:auto_now    → TIMESTAMPTZ DEFAULT NOW() (PG) / DATETIME DEFAULT CURRENT_TIMESTAMP (MySQL)
data:json:nullable              → JSONB (PG) / JSON (MySQL)
tags:array(TEXT):nullable        → TEXT[] (PG) / JSON (MySQL)
ip:inet:nullable                → INET (PG) / VARCHAR(45) (MySQL)
```

## Multi-App Migrations

For projects with multiple apps (like Django), use `migrate_all()` to apply migrations across directories in dependency order.

### Project Structure

```
myproject/
├── accounts/
│   └── migrations/
│       ├── 0001_initial.desi
│       └── 0002_add_email.desi
├── orders/
│   └── migrations/
│       ├── 0001_initial.desi          # Depends: accounts.0001_initial
│       └── 0002_add_status.desi
└── main.desi
```

### Cross-App Dependencies

Migration files can declare dependencies on other apps:

```python
# orders/migrations/0001_initial.desi
# Migration: 0001_initial
# App: orders
# Depends: accounts.0001_initial

def forward():
	db.op_create_table("orders", """id:auto
user_id:fk(users.id):cascade
total:decimal(10,2)""")
```

### Apply All

```desi
import db

db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")

# Apply all apps in dependency order
db.migrate_all(["accounts/migrations", "orders/migrations"])
```

The system builds a dependency graph and applies migrations via topological sort. Circular dependencies are detected and reported.

## Tracking Table

Migrations are tracked in `_desi_migrations` (created automatically):

| Column | Type | Description |
|---|---|---|
| `id` | SERIAL/AUTO_INCREMENT | Primary key |
| `app_label` | VARCHAR(128) | App name (derived from directory) |
| `filename` | VARCHAR(256) | Migration file name |
| `applied_sql` | TEXT | SQL that was executed |
| `rollback_sql` | TEXT | SQL to undo this operation |
| `applied_at` | TIMESTAMPTZ/DATETIME | When applied |

> **Note**: Existing tracking tables from older versions are auto-migrated — the `app_label` column is added automatically if missing.

## Rollback

```desi
# Undo the last migration for this app
let rolled = db.rollback_dir("migrations")
print(f"Rolled back {str(rolled)} operation(s)")
```

Rollback executes the `rollback_sql` stored in the tracking table when the migration was originally applied.

## Works with Both Databases

```desi
# PostgreSQL
db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")
db.makemigrations("migrations")  # generates portable ops
db.migrate_dir("migrations")     # → SERIAL, TIMESTAMPTZ, JSONB, TEXT[], INET

# MySQL
db.connect("mysql", "localhost", 3306, "mydb", "root", "pass")
db.migrate_dir("migrations")     # same file → AUTO_INCREMENT, DATETIME, JSON, VARCHAR(45)
```

## See Also

- [db](db.md) — Query builder, ORM, unified API
- [ORM Models](models.md) — `@model` decorator and field types
- [PostgreSQL](postgres.md) — PG-specific types
- [MySQL](mysql.md) — MySQL-specific types
