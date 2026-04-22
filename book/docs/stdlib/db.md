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

## See Also

- [Migrations](migrations.md) — File-based migrations, op-based DSL, multi-app support
- [ORM Models](models.md) — `@model` decorator and field types
- [PostgreSQL](postgres.md) — PG-specific types, auth, raw SQL examples
- [MySQL](mysql.md) — MySQL-specific types, auth, raw SQL examples
