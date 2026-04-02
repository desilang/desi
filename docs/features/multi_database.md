# Multi-Database Architecture

Desi ORM supports multiple named database connections, per-query routing,
connection pooling, and cross-database foreign key handling.

## Configuration — `desi.mod`

### Default Database

```toml
[database]
engine = "postgres"
host = "localhost"
name = "myapp"
user = "admin"
password = "secret"
max_conns = "5"
conn_timeout = "30"
```

### Named Databases

```toml
[database.analytics]
engine = "postgres"
host = "analytics.internal"
name = "warehouse"
user = "readonly"
password = "secret2"

[database.legacy]
engine = "mysql"
host = "old-server.internal"
name = "legacy_users"
user = "migrator"
password = "secret3"
```

Named databases use the `[database.name]` section syntax. Each section supports
the same fields as `[database]`. All sections are validated independently
(engine, host, name, user are required unless `schema_only = true`).

## Model Decorator — `@model(db=[...])`

```python
@model                                # → db=["default"] implicitly
@model(db=["analytics"])              # → single named DB
@model(db=["default", "analytics"])   # → multi-DB (table exists on both)
```

- `db` parameter is always `list[str]` — no union types
- First element is the **primary** database (default for reads/writes)
- Omitting `db` is equivalent to `db=["default"]`

## Query Routing

### Priority Cascade

| Priority | Mechanism | Scope |
|----------|-----------|-------|
| 1 (highest) | `.using("name")` | Per-query override |
| 2 | `db.use("name")` | Block-level override |
| 3 | `@model(db=[...])` | Per-model default (first element) |
| 4 (lowest) | `"default"` | Implicit fallback |

### Examples

```python
@model
class User:
    name: str

@model(db=["analytics"])
class PageView:
    url: str
    count: int

# Uses model defaults
users = User.objects.all()              # → "default"
views = PageView.objects.all()          # → "analytics"

# Per-query override
users = User.objects.using("analytics").all()  # → "analytics"
```

## Connection Pool API

```python
import db

# Initialize a pool with 5 connections
db.pool_init("postgres", "localhost", 5432, "myapp", "admin", "pass", 5)

# Acquire/release pattern
db.pool_acquire()
# ... run queries ...
db.pool_release()

# Stats
db.pool_size()       # → 5
db.pool_available()  # → 4 (one acquired)

# Cleanup
db.pool_close()
```

Pool features:
- Thread-safe with `pthread_mutex`
- Blocking acquire with 30-second timeout
- Fixed-size pool (configured at init time)

## Named Connections API

```python
import db

# Register named connections (auto-registered from desi.mod)
db.register_conn("analytics", "postgres", "analytics.internal", 5432, "warehouse", "ro", "pass")

# Switch active connection
db.use("analytics")

# Query against analytics DB
n = db.query("SELECT COUNT(*) FROM pageviews")

# Inspect
db.current_conn()  # → "analytics"
db.conn_count()    # → 1
db.list_conns()    # → "analytics"

# Cleanup
db.close_conn("analytics")
db.close_all()
```

## Cross-Database Foreign Keys

### Storage

FK fields referencing models on a different database are stored as plain
columns (no SQL constraint):

| Scenario | Migration SQL |
|----------|--------------|
| Same-DB FK | `user_id INTEGER REFERENCES users(id)` |
| Cross-DB FK | `user_id INTEGER` — bare column, no REFERENCES |

### Querying

Cross-DB prefetch uses two separate queries + application-level join:

```
Query 1 → analytics DB:  SELECT * FROM pageviews
         → extracts user_id values [1, 4, 7]

Query 2 → default DB:    SELECT * FROM users WHERE id IN (1, 4, 7)

App-level stitch: attach User objects to PageView.user fields
```

### Limitations

| Feature | Same-DB | Cross-DB |
|---------|---------|----------|
| SQL FK constraint | ✅ | ❌ |
| SQL JOIN (`select_related`) | ✅ | ❌ |
| Prefetch (two queries) | ✅ | ✅ |
| `ON DELETE CASCADE` (SQL) | ✅ | ❌ |
| App-level cascade | ✅ | ✅ |

## Error Handling

If `.using("analytics")` targets a database that doesn't have the table:
- The SQL query executes and the database returns an error
- The C runtime returns `-1` and sets the error string
- This is a **runtime error** — the compiler cannot know what tables exist on
  each server

## C Runtime Functions

### Pool (`db_pool.c`)

| Function | Purpose |
|----------|---------|
| `__db_pool_init(driver, host, port, db, user, pass, size)` | Initialize pool |
| `__db_pool_acquire()` | Get connection from pool |
| `__db_pool_release()` | Return connection to pool |
| `__db_pool_close()` | Close all pool connections |
| `__db_pool_size()` | Get total pool size |
| `__db_pool_available()` | Get free connection count |

### Named Connections (`db_connections.c`)

| Function | Purpose |
|----------|---------|
| `__db_register_conn(name, driver, host, port, db, user, pass)` | Register connection |
| `__db_use_conn(name)` | Switch active connection |
| `__db_close_conn(name)` | Close named connection |
| `__db_close_all_conns()` | Close all connections |
| `__db_current_conn()` | Get active connection name |
| `__db_conn_count()` | Count registered connections |
| `__db_list_conns()` | List connection names (CSV) |

### QuerySet Routing (`db_crud.c`)

| Function | Purpose |
|----------|---------|
| `__db_using(name)` | Per-query connection switch |
| `__qs_all()` | Execute SELECT (alias for fetch) |
| `__qs_first()` | LIMIT 1 + fetch |
| `__qs_last()` | ORDER BY id DESC + LIMIT 1 + fetch |
| `__qs_distinct()` | Set SELECT DISTINCT flag |
