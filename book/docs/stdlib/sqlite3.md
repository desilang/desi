# SQLite3 Module

The `sqlite3` module provides an embedded SQL database with **zero external dependencies**. The full SQLite3 engine is compiled into the Desi runtime — no system packages or shared libraries required.

## Import

```desi
import sqlite3
```

## API Reference

### Connection

| Function | Returns | Description |
|----------|---------|-------------|
| `connect(path)` | `int` | Open/create database. Use `":memory:"` for in-memory. Returns 0 on success |
| `disconnect()` | `int` | Close database connection |

### Executing SQL

| Function | Returns | Description |
|----------|---------|-------------|
| `execute(sql)` | `int` | Run non-query SQL (CREATE, INSERT, UPDATE, DELETE). Returns 0 on success |
| `query(sql)` | `int` | Run SELECT query and store results. Returns row count |

### Reading Results

| Function | Returns | Description |
|----------|---------|-------------|
| `fetch_next()` | `bool` | Advance to next row. Returns false when done |
| `get(name)` | `str` | Get field value by column name |
| `get_field(col)` | `str` | Get field value by column index |
| `row_count()` | `int` | Number of rows in last query |
| `col_count()` | `int` | Number of columns in last query |
| `col_name(idx)` | `str` | Column name by index |

### Write Info

| Function | Returns | Description |
|----------|---------|-------------|
| `last_insert_id()` | `int` | Rowid of last INSERT |
| `changes()` | `int` | Rows affected by last INSERT/UPDATE/DELETE |
| `last_error()` | `str` | Last error message |

### Transactions

| Function | Returns | Description |
|----------|---------|-------------|
| `begin()` | `int` | Begin transaction |
| `commit()` | `int` | Commit transaction |
| `rollback()` | `int` | Rollback transaction |

## Usage Examples

### Basic CRUD

```desi
import sqlite3

def main() -> int:
    sqlite3.connect(":memory:")

    # Create table
    sqlite3.execute("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)")

    # Insert
    sqlite3.execute("INSERT INTO users (name, age) VALUES ('Alice', 30)")
    sqlite3.execute("INSERT INTO users (name, age) VALUES ('Bob', 25)")
    print(f"Last insert ID: {str(sqlite3.last_insert_id())}")

    # Query
    let count = sqlite3.query("SELECT * FROM users")
    print(f"Found {str(count)} users")

    while sqlite3.fetch_next():
        let name = sqlite3.get("name")
        let age = sqlite3.get("age")
        print(f"  {name} (age {age})")

    sqlite3.disconnect()
    0
```

### Transactions

```desi
import sqlite3

def transfer(from_id: int, to_id: int, amount: int):
    sqlite3.begin()
    sqlite3.execute(f"UPDATE accounts SET balance = balance - {str(amount)} WHERE id = {str(from_id)}")
    sqlite3.execute(f"UPDATE accounts SET balance = balance + {str(amount)} WHERE id = {str(to_id)}")

    if sqlite3.changes() == 0:
        sqlite3.rollback()
        print("Transfer failed — rolling back")
    else:
        sqlite3.commit()
        print("Transfer committed")
```

### In-Memory Cache

```desi
import sqlite3

def init_cache():
    sqlite3.connect(":memory:")
    sqlite3.execute("CREATE TABLE cache (key TEXT PRIMARY KEY, value TEXT, expires INTEGER)")

def cache_set(key: str, value: str):
    sqlite3.execute(f"INSERT OR REPLACE INTO cache (key, value) VALUES ('{key}', '{value}')")

def cache_get(key: str) -> str:
    let n = sqlite3.query(f"SELECT value FROM cache WHERE key = '{key}'")
    if n > 0:
        sqlite3.fetch_next()
        return sqlite3.get("value")
    return ""
```

> [!NOTE]
> SQLite3 is embedded directly in the Desi runtime. No `libsqlite3`, no `apt install`, no `brew install` — it works everywhere Desi compiles: macOS, Linux, and Windows.

> [!TIP]
> Use `":memory:"` for fast in-memory databases that don't persist to disk. Perfect for caches, temporary data, and tests.

## See Also

- [Database Module](db.md) — PostgreSQL/MySQL database
- [Redis Module](redis.md) — Key-value store
- [JSON Module](json.md) — JSON parsing
