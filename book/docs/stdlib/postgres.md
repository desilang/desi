# PostgreSQL with Desi

Desi connects to PostgreSQL using a **pure C wire protocol** — no `libpq` dependency. Everything is bundled in `libdesi.a`.

## Connecting

```desi
import db

# Connect to PostgreSQL
let status = db.connect("postgres", "localhost", 5432, "mydb", "user", "password")
if db.is_connected() != 1:
    print(f"Error: {db.last_error()}")
```

The driver accepts `"postgres"`, `"pg"`, or `"postgresql"`.

## Querying

```desi
# SELECT — returns number of rows
let rows = db.query("SELECT id, name, email FROM users WHERE age > 21")

# Access results
for i in range(rows):
    let name = db.get_field(i, "name")
    let email = db.get_field(i, "email")
    print(f"{name}: {email}")

# Or by column index
let first_name = db.get_value(0, 1)
```

## Executing

```desi
# INSERT/UPDATE/DELETE — returns affected row count
let inserted = db.execute("INSERT INTO users (name, age) VALUES ('Alice', 30)")
let updated = db.execute("UPDATE users SET age = 31 WHERE name = 'Alice'")
let deleted = db.execute("DELETE FROM users WHERE id = 5")

# DDL works too
db.execute("CREATE INDEX idx_users_email ON users(email)")
```

## Raw SQL — Full PostgreSQL Power

Since `db.query()` and `db.execute()` pass raw SQL directly to PostgreSQL, **any valid PG SQL works**:

```desi
# CTEs
db.query("WITH active AS (SELECT * FROM users WHERE active) SELECT * FROM active")

# Window functions
db.query("SELECT name, age, ROW_NUMBER() OVER (ORDER BY age) FROM users")

# RETURNING clause
db.execute("INSERT INTO users (name) VALUES ('Bob') RETURNING id, created_at")
let new_id = db.get_value(0, 0)

# JSON operators
db.query("SELECT metadata->>'theme' FROM user_settings WHERE id = 1")

# Array operations
db.query("SELECT * FROM events WHERE 'desi' = ANY(tags)")

# Type casting
db.query("SELECT '2025-01-01'::date + interval '30 days'")
```

## ORM Field Types

| ORM Field | PostgreSQL Type | Notes |
|---|---|---|
| `auto_field` | `SERIAL PRIMARY KEY` | Auto-increment integer |
| `int_field` | `INTEGER` | |
| `bigint_field` | `BIGINT` | |
| `char_field` | `VARCHAR(n)` | |
| `text_field` | `TEXT` | |
| `bool_field` | `BOOLEAN` | Native boolean |
| `float_field` | `DOUBLE PRECISION` | |
| `decimal_field` | `DECIMAL(p,s)` | Exact numeric |
| `datetime_field` | `TIMESTAMPTZ` | Timezone-aware (UTC) |
| `json_field` | `JSONB` | Binary JSON, indexable |
| `uuid_field` | `UUID` | Native UUID type |
| `array_field` | `TEXT[]`, `INTEGER[]` | Native arrays |
| `inet_field` | `INET` | IP addresses |
| `foreign_key` | `INTEGER REFERENCES` | With FK constraint |
| `custom_field` | *(any type)* | Pass-through |

## ORM Example

```desi
import db

# Define model
db.model("events")
db.auto_field("id")
db.uuid_field("event_id", 0, 1)          # NOT NULL, UNIQUE
db.char_field("title", 200, 0, 0)
db.decimal_field("price", 10, 2, 0)
db.datetime_field("created_at", 0, 1, 0) # auto_now_add → DEFAULT NOW()
db.json_field("metadata", 1)             # nullable
db.inet_field("client_ip", 1)
db.array_field("tags", "TEXT", 1)
db.custom_field("location", "POINT", 1)  # any PG type!

# Connect and create
db.connect("postgres", "localhost", 5432, "mydb", "user", "pass")
let sql = db.create_table_sql("events")
db.execute(sql)
```

Generated SQL:
```sql
CREATE TABLE IF NOT EXISTS events (
  id SERIAL PRIMARY KEY,
  event_id UUID NOT NULL UNIQUE,
  title VARCHAR(200) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  metadata JSONB,
  client_ip INET,
  tags TEXT[],
  location POINT
)
```

## Debugging

```desi
db.set_debug(1)           # logs all protocol messages to stderr
db.dump_results()         # prints formatted result table
print(db.connection_info())
print(db.driver())        # → "postgres"
```

## Authentication

Supported auth methods:
- **Trust** — no password (common in dev)
- **Password** — cleartext password
- **MD5** — `md5(md5(password + user) + salt)`

## Cleanup

```desi
db.close()
```
