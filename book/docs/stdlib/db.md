# db — Database Module

Desi's `db` module provides a unified API for PostgreSQL and MySQL, plus a query builder and ORM.

**Zero dependencies** — pure C wire protocol clients bundled in `libdesi.a`.

## Import

```desi
import db
```

## Connecting

```desi
# PostgreSQL
db.connect("postgres", "localhost", 5432, "mydb", "user", "password")

# MySQL
db.connect("mysql", "localhost", 3306, "mydb", "root", "password")

# Check connection
if db.is_connected() == 1:
    print(f"Connected to {db.driver()}")
```

## Raw SQL

```desi
# SELECT — returns row count
let rows = db.query("SELECT * FROM users WHERE age > 21")

# Access results
for i in range(rows):
    let name = db.get_field(i, "name")     # by column name
    let age = db.get_value(i, 2)           # by column index
    print(f"{name}, age {age}")

# INSERT/UPDATE/DELETE — returns affected row count
db.execute("INSERT INTO users (name) VALUES ('Alice')")
db.execute("UPDATE users SET age = 31 WHERE name = 'Alice'")
db.execute("DELETE FROM users WHERE id = 5")

# Close when done
db.close()
```

## Query Builder

```desi
db.find("users")
db.columns("name")
db.columns("email")
db.where("age", ">", "21")
db.order_by("name", 0)
db.limit(10)
let sql = db.build_sql()
# → SELECT name, email FROM users WHERE age > 21 ORDER BY name LIMIT 10

db.insert_into("users")
db.set_field("name", "Alice")
db.set_field("email", "alice@example.com")
let sql = db.build_sql()
# → INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')
```

## ORM — Model Definition

```desi
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.char_field("email", 255, 0, 1)           # unique
db.int_field("age", 0, 0)
db.bool_field("active", 1, 0)
db.datetime_field("created_at", 0, 1, 0)    # auto_now_add
db.json_field("metadata", 1)                # nullable
db.uuid_field("public_id", 0, 1)            # UUID
db.decimal_field("balance", 10, 2, 0)       # DECIMAL(10,2)
db.array_field("tags", "TEXT", 1)            # PG: TEXT[], MySQL: JSON
db.inet_field("ip", 1)                      # PG: INET, MySQL: VARCHAR(45)
db.custom_field("extra", "HSTORE", 1)       # any native DB type

let sql = db.create_table_sql("users")
db.execute(sql)
```

## Debugging

```desi
db.set_debug(1)            # log protocol messages
db.dump_results()          # print result table to stderr
print(db.connection_info())
print(db.driver())         # "postgres", "mysql", or "none"
```

## ORM QuerySet (Django-style)

```desi
# Django-style filter with __ lookups
db.find("users")
db.filter("age__gte", "18")
db.filter("name__contains", "alice")
db.order_by("-created_at")
db.limit(10)
let rows = db.fetch()

# Aggregation
db.find("orders")
db.annotate("total", "SUM", "amount")
db.group_by("customer_id")
let rows = db.fetch()

# Insert
db.find("users")
db.set("name", "Alice")
db.set("email", "alice@example.com")
db.do_insert()

# Bulk insert
db.find("users")
db.bulk_begin(2)
db.bulk_col("name")
db.bulk_col("email")
db.bulk_row2("Alice", "alice@example.com")
db.bulk_row2("Bob", "bob@example.com")
db.bulk_execute()
```

### Supported Lookups

`exact`, `iexact`, `contains`, `icontains`, `startswith`, `istartswith`, `endswith`, `iendswith`, `gt`, `gte`, `lt`, `lte`, `ne`, `in`, `range`, `isnull`, `year`, `month`, `day`, `hour`, `minute`, `second`, `week`, `quarter`, `json_has`, `json_contains`

### HAVING Clause

Filter aggregate results after `group_by()` + `annotate()`:

```desi
db.objects("orders")
db.group_by("customer_id")
db.annotate("total", "SUM", "amount")
db.having("SUM(amount) > 100")        # raw condition
db.fetch_all()

# Parameterized (safe for user input):
db.objects("orders")
db.group_by("customer_id")
db.annotate("order_count", "COUNT", "*")
db.having_val("COUNT(*) >", "5")       # value is $N-bound
db.fetch_all()
```

### Multi-column Ordering

Append multiple `ORDER BY` columns (unlike `sort_by()` which overwrites):

```desi
db.objects("employees")
db.order_by_add("-salary")   # primary: salary DESC
db.order_by_add("name")     # secondary: name ASC
db.fetch_all()
# → ORDER BY salary DESC, name ASC
```

### Multi-column Update

Update multiple fields in a single SQL statement:

```desi
db.objects("users")
db.filter_by("id", "1")
db.update_set("name", "Alice")
db.update_set("email", "alice@example.com")
db.update_exec()
# → UPDATE users SET name=$1, email=$2 WHERE id = $3
```

### Pagination

Page-based helpers that calculate LIMIT/OFFSET:

```desi
db.objects("products")
db.filter_by("category", "electronics")

# Get total count (ignores LIMIT/OFFSET)
let total = db.total_count()

# Fetch page 2 (25 items per page)
db.paginate(2, 25)    # → LIMIT 25 OFFSET 25
db.fetch_all()

print(f"Showing page 2 of {total} results")
```

### Soft Delete

Mark rows as deleted without removing them from the database:

```desi
# Enable soft-delete mode (column defaults to "is_deleted")
db.objects("users")
db.soft_delete_mode("is_deleted")

# All queries auto-filter: WHERE is_deleted = FALSE
db.fetch_all()   # only active users

# Soft-delete a row (UPDATE SET is_deleted = TRUE)
db.objects("users")
db.soft_delete_mode("is_deleted")
db.filter_by("id", "42")
db.soft_delete()

# Include deleted rows (bypass auto-filter)
db.objects("users")
db.soft_delete_mode("is_deleted")
db.with_deleted()
db.fetch_all()   # all users, including deleted

# Restore a soft-deleted row
db.objects("users")
db.soft_delete_mode("is_deleted")
db.with_deleted()
db.filter_by("id", "42")
db.restore()     # SET is_deleted = FALSE

# Permanently delete (real DELETE FROM, ignores soft-delete)
db.objects("users")
db.filter_by("id", "42")
db.hard_delete()
```

### Row Locking (select_for_update)

Lock rows for concurrent-safe reads within a transaction:

```desi
import db

# Basic row lock — blocks until lock acquired
db.begin()
db.objects("accounts")
db.filter_by("id", "1")
db.select_for_update("")      # FOR UPDATE
db.fetch_all()
db.update("balance", "500")
db.commit()

# Non-blocking — error if row already locked
db.begin()
db.objects("accounts")
db.filter_by("id", "1")
db.select_for_update("nowait")    # FOR UPDATE NOWAIT
db.fetch_all()
db.commit()

# Skip locked rows — useful for job queues
db.begin()
db.objects("jobs")
db.filter_by("status", "pending")
db.select_for_update("skip_locked")  # FOR UPDATE SKIP LOCKED
db.limit(10)
db.fetch_all()
db.commit()
```

Modes: `""` (blocking), `"nowait"` (error if locked), `"skip_locked"` (skip locked rows).
Works with both PostgreSQL and MySQL.

## File-Based Migrations

For real projects, generate version-controlled migration files with portable operations:

```desi
db.makemigrations("migrations")          # Generate from ORM diff
db.migrate_dir("migrations")             # Apply pending
db.rollback_dir("migrations")            # Undo last
db.migration_status_dir("migrations")    # Show applied/pending

# Multi-app: apply across apps in dependency order
db.migrate_all(["accounts/migrations", "orders/migrations"])
```

See [Migrations](migrations.md) for the full guide.

## Internals

Under the hood, each query chain (`db.objects()` → `db.filter_by()` → `db.fetch_all()`) creates a **QuerySet handle** — a heap-allocated struct that holds all query state (table, WHERE clause, parameters, etc.).

The compiler emits explicit lifecycle management for each chain:

1. **Allocate** — `__qs_handle_new("table")` creates a fresh handle
2. **Bind** — `__qs_handle_bind(qs)` installs it as active for the chain
3. **Use** — intermediate calls (`filter`, `order_by`, etc.) operate on the active handle
4. **Free** — `__qs_handle_free(qs)` deallocates after the terminal call

This means:

- **Concurrent queries are safe** — each `spawn`-ed task gets its own isolated query state via thread-local binding.
- **No parameter limits** — parameters, INSERT fields, and UPDATE fields grow dynamically as needed.
- **Automatic cleanup** — the handle is freed after the terminal call (`fetch_all`, `update_exec`, etc.).
- **Deterministic lifetime** — the compiler controls handle allocation and deallocation, preventing leaks.

You don't need to manage handles manually — the compiler and runtime do it for you.

See [QuerySet Handles](../features/queryset_handles.md) for the full architecture guide.

## See Also

- [Migrations](migrations.md) — File-based migrations, op-based DSL, multi-app support
- [ORM Models](models.md) — `@model` decorator and field types
- [PostgreSQL](postgres.md) — PG-specific types, auth, raw SQL examples
- [MySQL](mysql.md) — MySQL-specific types, auth, raw SQL examples

