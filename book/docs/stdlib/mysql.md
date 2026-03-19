# MySQL with Desi

Desi connects to MySQL using a **pure C wire protocol** — no `libmysqlclient` dependency. Everything is bundled in `libdesi.a`.

## Connecting

```desi
import db

# Connect to MySQL
let status = db.connect("mysql", "localhost", 3306, "mydb", "root", "password")
if db.is_connected() != 1:
    print(f"Error: {db.last_error()}")
```

The driver accepts `"mysql"`, `"my"`, or `"mariadb"`.

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
```

## Raw SQL — Full MySQL Power

```desi
# Subqueries
db.query("SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)")

# JSON support (MySQL 5.7+)
db.query("SELECT JSON_EXTRACT(metadata, '$.theme') FROM settings")

# Multi-table UPDATE
db.execute("UPDATE orders o JOIN users u ON o.user_id = u.id SET o.status = 'active' WHERE u.active = 1")

# USE another database
db.execute("USE other_database")

# Variables
db.execute("SET @max_age = 50")
db.query("SELECT * FROM users WHERE age < @max_age")
```

## ORM Field Types

| ORM Field | MySQL Type | Notes |
|---|---|---|
| `auto_field` | `INT AUTO_INCREMENT PRIMARY KEY` | |
| `int_field` | `INTEGER` | |
| `bigint_field` | `BIGINT` | |
| `char_field` | `VARCHAR(n)` | |
| `text_field` | `TEXT` | |
| `bool_field` | `TINYINT(1)` | 0/1 |
| `float_field` | `DOUBLE` | |
| `decimal_field` | `DECIMAL(p,s)` | Exact numeric |
| `datetime_field` | `DATETIME` | No timezone conversion |
| `json_field` | `JSON` | MySQL 5.7+ |
| `uuid_field` | `VARCHAR(36)` | Stored as string |
| `array_field` | `JSON` | Arrays stored as JSON |
| `inet_field` | `VARCHAR(45)` | IPv4/IPv6 as string |
| `foreign_key` | `INTEGER REFERENCES` | InnoDB FK |
| `custom_field` | *(any type)* | Pass-through |

## ORM Example

```desi
import db

# Define model
db.model("products")
db.auto_field("id")
db.uuid_field("sku", 0, 1)
db.char_field("name", 200, 0, 0)
db.decimal_field("price", 10, 2, 0)
db.bool_field("available", 1, 0)
db.datetime_field("created_at", 0, 1, 0)
db.json_field("attributes", 1)
db.custom_field("status", "ENUM('draft','published','archived')", 0)

# Connect and create
db.connect("mysql", "localhost", 3306, "shop", "root", "pass")
let sql = db.create_table_sql("products")
db.execute(sql)
```

Generated SQL:
```sql
CREATE TABLE IF NOT EXISTS products (
  id INT AUTO_INCREMENT PRIMARY KEY,
  sku VARCHAR(36) NOT NULL UNIQUE,
  name VARCHAR(200) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  available TINYINT(1) NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  attributes JSON,
  status ENUM('draft','published','archived') NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
```

## Debugging

```desi
db.set_debug(1)           # logs all protocol messages to stderr
db.dump_results()         # prints formatted result table
print(db.connection_info())
print(db.driver())        # → "mysql"
```

## Authentication

Supported MySQL auth methods:
- **mysql_native_password** — SHA1-based (MySQL 5.x default)
- **caching_sha2_password** — Fast auth (MySQL 8.x default)
- **Auth method switch** — Server-initiated plugin change

## MySQL-Specific Notes

- **ENGINE=InnoDB** — Auto-set on all ORM tables for transaction support
- **utf8mb4** — Full Unicode by default (supports emoji)
- **TIMESTAMP vs DATETIME** — ORM uses `DATETIME` (no UTC auto-conversion; use `TIMESTAMP` via `custom_field` if needed)

## Cleanup

```desi
db.close()
```
