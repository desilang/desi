# PostgreSQL with Desi

PostgreSQL-specific features and generated SQL.

## Setup

```desi
import db
db.use_postgres()
```

## Generated CREATE TABLE

```sql
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  email VARCHAR(255) NOT NULL UNIQUE,
  age INTEGER NOT NULL DEFAULT 0,
  bio TEXT,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  metadata JSONB
)
```

## PostgreSQL-Specific Types

| Desi Field | PostgreSQL Type |
|---|---|
| `auto_field` | `SERIAL PRIMARY KEY` |
| `bool_field` | `BOOLEAN` |
| `datetime_field` | `TIMESTAMP` |
| `json_field` | `JSONB` (binary JSON, indexed) |
| `float_field` | `DOUBLE PRECISION` |

## PostgreSQL Features

- **JSONB** — Binary JSON with indexing support (vs plain JSON)
- **RETURNING** — Get inserted row back:
  ```desi
  db.insert_into("users")
  db.set_field("name", "Alice")
  db.returning()
  let sql = db.build_sql()
  # → INSERT INTO users (name) VALUES ('Alice') RETURNING *
  ```
- **NOW()** — Used for `auto_now_add` datetime fields

## Example

```desi
import db

db.use_postgres()

# Define model
db.model("posts")
db.auto_field("id")
db.char_field("title", 200, 0, 0)
db.text_field("body", 0)
db.json_field("tags", 1)
db.foreign_key("author_id", "users", "id", 0)
db.datetime_field("created_at", 0, 1, 0)

# Generate migration SQL
let sql = db.create_table_sql("posts")
print(sql)
```
