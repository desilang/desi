# MySQL with Desi

MySQL-specific features and generated SQL.

## Setup

```desi
import db
db.use_mysql()
```

## Generated CREATE TABLE

```sql
CREATE TABLE IF NOT EXISTS users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  email VARCHAR(255) NOT NULL UNIQUE,
  age INTEGER NOT NULL DEFAULT 0,
  bio TEXT,
  active TINYINT(1) NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  metadata JSON
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
```

## MySQL-Specific Types

| Desi Field | MySQL Type |
|---|---|
| `auto_field` | `INT AUTO_INCREMENT PRIMARY KEY` |
| `bool_field` | `TINYINT(1)` |
| `datetime_field` | `DATETIME` |
| `json_field` | `JSON` (MySQL 5.7+) |
| `float_field` | `DOUBLE` |

## MySQL Features

- **ENGINE=InnoDB** — Auto-set for transaction support
- **utf8mb4** — Full Unicode support by default
- **CURRENT_TIMESTAMP** — Used for `auto_now_add` datetime fields
- **JSON** — Available in MySQL 5.7+ (plain JSON, not binary)

## Example

```desi
import db

db.use_mysql()

# Define model
db.model("products")
db.auto_field("id")
db.char_field("name", 200, 0, 0)
db.text_field("description", 1)
db.float_field("price", 0)
db.int_field("stock", 0, 0)
db.bool_field("available", 1, 0)

# Generate migration SQL
let sql = db.create_table_sql("products")
print(sql)
```
