# Database & ORM

Desi includes a Django-inspired database module with ORM, query builder, migrations, and transactions.

## Quick Start

```desi
import db

# Define model
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.int_field("age", 0, 0)

# Generate SQL
let sql: str = db.create_table_sql("users")
print(sql)
# CREATE TABLE IF NOT EXISTS users (
#   id SERIAL PRIMARY KEY,
#   name VARCHAR(100) NOT NULL,
#   age INTEGER NOT NULL DEFAULT 0
# )
```

## CRUD Operations

### Create (INSERT)

```desi
db.objects("users")
db.create(name="Alice", email="alice@test.com", age="30")
```

Uses `**kwargs` — pass any number of `field="value"` pairs. No fixed argument limits.

### Read (SELECT)

```desi
# Filter with Django-style lookups
db.objects("users")
db.filter(name="Alice", age__gt="21")
let rows: int = db.fetch_all()

# Read results
let name: str = db.get_field(0, "name")
let email: str = db.get_field(0, "email")
```

**Supported lookups:**

| Lookup | SQL | Example |
|--------|-----|---------|
| `name` | `name = 'value'` | `db.filter(name="Alice")` |
| `age__gt` | `age > 'value'` | `db.filter(age__gt="21")` |
| `age__lt` | `age < 'value'` | `db.filter(age__lt="65")` |
| `name__contains` | `name LIKE '%value%'` | `db.filter(name__contains="Ali")` |

### Update

```desi
db.objects("users")
db.filter(name="Alice")
db.update_fields(email="new@test.com", age="31")
```

### Delete

```desi
db.objects("users")
db.filter(name="Alice")
db.delete()
```

### Get Single Row

```desi
db.objects("users")
let found: int = db.get(name="Alice")
if found > 0:
    let name: str = db.get_field(0, "name")
```

## Model Definition

### Field Types

```desi
db.model("products")
db.auto_field("id")                              # Auto-increment PK
db.char_field("name", 200, 0, 0)                 # VARCHAR(200)
db.text_field("description", 1)                  # TEXT, nullable
db.int_field("price", 0, 0)                      # INTEGER
db.float_field("weight", 1)                      # FLOAT, nullable
db.bool_field("active", 1, 0)                    # BOOLEAN, default true
db.datetime_field("created_at", 0, 1, 0)         # TIMESTAMP, auto_now_add
db.json_field("metadata", 1)                     # JSONB/JSON, nullable
db.foreign_key("category_id", "categories", "id", 0) # FK reference
```

### SQL Generation

```desi
let create_sql: str = db.create_table_sql("products")
let drop_sql: str = db.drop_table_sql("products")
let alter_sql: str = db.add_column_sql("products", "sku")
```

## Query Builder

For complex queries, use the low-level query builder:

```desi
db.find("users")                       # SELECT
db.columns("name, email")              # columns
db.where("age", ">", "21")            # WHERE
db.join("INNER", "orders", "users.id = orders.user_id")
db.order_by("name", 0)                # ORDER BY name ASC
db.limit(10)
db.offset(20)
let sql: str = db.build_sql()
```

## Database Connection

```desi
# Connect to PostgreSQL
db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")

# Or MySQL
db.connect("mysql", "localhost", 3306, "mydb", "user", "pass")

# Execute SQL
db.execute(sql)
let rows: int = db.query("SELECT * FROM users")

# Check connection
let connected: int = db.is_connected()
db.close()
```

## Transactions

```desi
db.begin()
db.objects("accounts")
db.filter(id="1")
db.update_fields(balance="900")
db.commit()
# Or: db.rollback_tx()

# Savepoints (nested transactions)
db.begin()
db.savepoint("sp1")
# ... operations ...
db.savepoint_rollback("sp1")  # undo to savepoint
db.commit()
```

## Migrations

```desi
# Apply pending migrations
let applied: int = db.migrate()

# Rollback last migration
db.rollback()

# Check status
let status: str = db.migration_status()
print(status)
```

## See Also

- [Functions](../language/functions.md) — `**kwargs` syntax for CRUD operations
- [Collections](../language/collections.md) — `dict` type used by kwargs
