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
| `BoolField` | `pub active: BoolField(default=true)` | `BOOLEAN DEFAULT FALSE` |
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
    #   active BOOLEAN NOT NULL DEFAULT FALSE,
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
