# ORM Models — Django-Style

Define your database schema as classes with the `@model` decorator. Desi auto-generates SQL, handles foreign keys, and registers models before `main()` runs — zero boilerplate.

## Import

```desi
import db
```

## Defining Models

```desi
import db

db.use_postgres()    # or db.use_mysql()

@model("users")
class User:
    pub name: CharField(100)
    pub email: CharField(200, unique=true)
    pub age: IntField(default=0)
    pub active: BoolField(default=true)
    pub created_at: DateTimeField(auto_now_add=true)
```

- `@model("tablename")` registers the class as a database table
- An `id` primary key is **auto-inserted** if you don't declare one
- Field types (`CharField`, `IntField`, etc.) are auto-injected inside `@model` classes — no imports needed

## Field Types

| Field | Syntax | SQL (Postgres) |
|-------|--------|----------------|
| `AutoField` | `pub id: AutoField` | `SERIAL PRIMARY KEY` |
| `BigAutoField` | `pub id: BigAutoField` | `BIGSERIAL PRIMARY KEY` |
| `CharField` | `pub name: CharField(100)` | `VARCHAR(100)` |
| `TextField` | `pub bio: TextField` | `TEXT` |
| `IntField` | `pub age: IntField(default=0)` | `INTEGER DEFAULT 0` |
| `BigIntField` | `pub count: BigIntField` | `BIGINT` |
| `BoolField` | `pub active: BoolField(default=true)` | `BOOLEAN DEFAULT TRUE` |
| `FloatField` | `pub score: FloatField` | `DOUBLE PRECISION` |
| `DecimalField` | `pub price: DecimalField(10, 2)` | `DECIMAL(10,2)` |
| `DateTimeField` | `pub created: DateTimeField(auto_now_add=true)` | `TIMESTAMPTZ DEFAULT NOW()` |
| `UUIDField` | `pub uid: UUIDField` | `UUID` |
| `JsonField` | `pub data: JsonField(nullable=true)` | `JSONB` |
| `ForeignKey` | `pub author_id: ForeignKey(User, on_delete=CASCADE)` | `INTEGER REFERENCES users(id) ON DELETE CASCADE` |

## Field Options

Pass options as keyword arguments:

```desi
pub name: CharField(100, unique=true)           # UNIQUE constraint
pub bio: TextField(nullable=true)               # allows NULL
pub age: IntField(default=0)                    # DEFAULT 0
pub email: CharField(200, db_index=true)        # CREATE INDEX
pub uid: UUIDField(primary_key=true)            # use as PK instead of auto id
pub created: DateTimeField(auto_now_add=true)   # DEFAULT NOW() on insert
pub updated: DateTimeField(auto_now=true)       # auto-update on save
```

## ForeignKey

Reference another model by **class name** — Desi auto-resolves the table and primary key:

```desi
@model("users")
class User:
    pub name: CharField(100)

@model("posts")
class Post:
    pub title: CharField(200)
    pub author_id: ForeignKey(User, on_delete=CASCADE)
```

### on_delete Options

| Option | SQL | Behavior |
|--------|-----|----------|
| `CASCADE` | `ON DELETE CASCADE` | Delete related rows |
| `PROTECT` | App-level check | Prevent deletion |
| `SET_NULL` | `ON DELETE SET NULL` | Set FK to NULL |
| `SET_DEFAULT` | `ON DELETE SET DEFAULT` | Set FK to default |
| `DO_NOTHING` | `ON DELETE NO ACTION` | No action |
| `RESTRICT` | `ON DELETE RESTRICT` | Prevent if referenced |

## Auto Primary Key

If you don't declare any `AutoField` or `BigAutoField`, Desi auto-inserts:

```desi
pub id: AutoField    # added automatically
```

To use a custom PK:

```desi
@model("sessions")
class Session:
    pub token: UUIDField(primary_key=true)    # no auto id
    pub user_id: ForeignKey(User, on_delete=CASCADE)
```

## Meta Class

Add a nested `class Meta` inside `@model` to declare multi-column constraints and indexes — just like Django:

```desi
@model("posts")
class Post:
    pub title: CharField(200)
    pub author_id: ForeignKey(User, on_delete=CASCADE)

    class Meta:
        unique_together: [title, author_id]
        indexes: [title]
```

### Meta Options

| Option | Syntax | SQL Output |
|--------|--------|------------|
| `unique_together` | `unique_together: [field1, field2]` | `UNIQUE (field1, field2)` in CREATE TABLE |
| `indexes` | `indexes: [field1]` | `CREATE INDEX idx_table_field1 ON table (field1)` |
| `ordering` | `ordering: [-created_at]` | *(query builder, not DDL)* |
| `abstract` | `abstract: true` | *(skip table generation)* |

- Fields are listed in brackets: `[field1, field2]`
- Prefix with `-` for descending order: `[-created_at]`
- Multiple constraints are supported

## Database Configuration

Configure your database engine in `desi.mod`:

```toml
# Schema-only (SQL generation, no live DB)
[database]
engine = "postgres"
schema_only = true

# Full connection
[database]
engine = "postgres"
host = "localhost"
name = "myapp_db"
user = "admin"
password = "secret"
ssl_mode = "require"
charset = "UTF8"
timezone = "UTC"
prefix = "app1_"
```

### Required Fields

| Field | When Required | Description |
|-------|---------------|-------------|
| `engine` | Always | `"postgres"` or `"mysql"` |
| `host` | Not schema_only | Hostname or IP (`"localhost"`, `"192.168.1.100"`) |
| `name` | Not schema_only | Database name |
| `user` | Not schema_only | DB username |

### Optional Fields

| Field | Default | Description |
|-------|---------|-------------|
| `schema_only` | `false` | If `true`, skip connection fields (SQL generation only) |
| `port` | `5432` / `3306` | Auto-defaults per engine |
| `password` | — | DB password |
| `ssl_mode` | — | `"disable"`, `"require"`, `"verify-ca"`, `"verify-full"` |
| `charset` | — | Character encoding (`"utf8mb4"`, `"UTF8"`) |
| `timezone` | — | Connection timezone (`"UTC"`, `"America/Chicago"`) |
| `prefix` | — | Table name prefix for multi-tenancy |
| `options` | — | Extra DSN/connection string parameters |

The compiler auto-configures the SQL dialect — no manual `db.use_postgres()` needed

## Choices (CHECK Constraints)

Add `choices=` to any field to generate a SQL `CHECK` constraint:

```desi
@model("articles")
class Article:
    pub status: CharField(20, choices=[draft, published, archived])
```

Generates: `status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'published', 'archived'))`

## Generated Fields (Computed Columns)

Use `GeneratedField` for columns computed from other fields:

```desi
@model("people")
class Person:
    pub first_name: CharField(50)
    pub last_name: CharField(50)
    pub full_name: GeneratedField(expression="first_name || ' ' || last_name", output=CharField(100))
```

Generates: `full_name VARCHAR(100) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED`

## Composite Primary Keys

Use `primary_key` in `class Meta` for multi-column primary keys (no auto `id` field):

```desi
@model("order_items")
class OrderItem:
    pub order_id: IntField()
    pub product_id: IntField()
    pub quantity: IntField(default=1)

    class Meta:
        primary_key: [order_id, product_id]
```

Generates: `PRIMARY KEY (order_id,product_id)` — the auto `id` field is omitted.

## Generating SQL

```desi
def main() -> int:
    let sql = db.create_table_sql("users")
    print(sql)
    # CREATE TABLE IF NOT EXISTS users (
    #   id SERIAL PRIMARY KEY,
    #   name VARCHAR(100) NOT NULL,
    #   email VARCHAR(200) NOT NULL UNIQUE,
    #   age INTEGER NOT NULL DEFAULT 0,
    #   active BOOLEAN NOT NULL DEFAULT TRUE,
    #   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    # )
    0
```

## Database Dialect

Set the SQL dialect before printing/executing SQL:

```desi
db.use_postgres()    # PostgreSQL (default)
db.use_mysql()       # MySQL
```

## Full Example

```desi
import db

db.use_postgres()

@model("users")
class User:
    pub name: CharField(100)
    pub email: CharField(200, unique=true)
    pub age: IntField(default=0)
    pub active: BoolField(default=true)
    pub created_at: DateTimeField(auto_now_add=true)

@model("posts")
class Post:
    pub title: CharField(200)
    pub body: TextField(nullable=true)
    pub author_id: ForeignKey(User, on_delete=CASCADE)

    class Meta:
        unique_together: [title, author_id]
        indexes: [title]

def main() -> int:
    print(db.create_table_sql("users"))
    print("---")
    print(db.create_table_sql("posts"))
    0
```

## See Also

- [Database & ORM](database.md) — CRUD operations, query builder, transactions
- [PostgreSQL](postgres.md) — PG-specific connection and types
- [MySQL](mysql.md) — MySQL-specific connection and types
