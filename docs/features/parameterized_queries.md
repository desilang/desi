# Parameterized Queries

**Status**: ✅ Implemented  
**Since**: v0.10  
**Related**: [ORM Models](orm_models.md), [Macros](macros.md)

---

## Overview

All ORM CRUD operations use **parameterized queries** to prevent SQL injection. User-supplied values are never interpolated into SQL strings. Instead, the SQL contains placeholders and the values are sent separately to the database.

```
SQL:    SELECT * FROM users WHERE name = $1 AND age > $2
Params: ["Alice", "21"]
```

This architecture mirrors Django's approach: parameterization at the driver level for maximum security.

## How It Works

### PostgreSQL: Extended Query Protocol

For PostgreSQL, the runtime uses the **PG v3 Extended Query Protocol** (Parse → Bind → Execute → Sync). Parameters are sent as a separate array in the Bind message:

```
→ Parse:    "SELECT * FROM users WHERE name = $1"  (no type OIDs, server infers)
→ Bind:     params = ["Alice"]  (text format)
→ Describe: portal
→ Execute:  all rows
→ Sync
← ParseComplete, BindComplete, RowDescription, DataRow..., CommandComplete, ReadyForQuery
```

All messages are batched in a single TCP write for efficiency.

### MySQL: Client-Side Escaping

For MySQL, the runtime uses proper client-side escaping (equivalent to `mysql_real_escape_string`):

- Single quotes are doubled: `'` → `''`
- Backslashes are escaped: `\` → `\\`
- Null bytes, newlines, carriage returns, Ctrl-Z are escaped
- Escaped values are wrapped in single quotes
- The escaped SQL is sent via `COM_QUERY`

This provides the same SQL injection protection as MySQL's prepared statement protocol, with simpler wire protocol requirements.

## Placeholder Format

| Database | Placeholder | Example |
|----------|-------------|---------|
| PostgreSQL | `$1`, `$2`, `$3` | `WHERE name = $1 AND age > $2` |
| MySQL | `?`, `?`, `?` | `WHERE name = ? AND age > ?` |

The CRUD layer automatically selects the correct format based on the active driver.

## What's Parameterized

| Operation | Before (unsafe) | After (parameterized) |
|-----------|----------------|----------------------|
| `filter(name="Ali")` | `WHERE name = 'Ali'` | `WHERE name = $1` + params=["Ali"] |
| `filter(age__gt="21")` | `WHERE age > '21'` | `WHERE age > $1` + params=["21"] |
| `filter(name__contains="test")` | `WHERE name LIKE '%test%'` | `WHERE name LIKE $1` + params=["%test%"] |
| `filter(id__in="1,2,3")` | `WHERE id IN (1,2,3)` | `WHERE id IN ($1, $2, $3)` + params=["1","2","3"] |
| `create(name="Ali")` | `INSERT ... VALUES ('Ali')` | `INSERT ... VALUES ($1)` + params=["Ali"] |
| `update("name", "Bob")` | `SET name = 'Bob'` | `SET name = $1` + params=["Bob"] |
| `Q(name="Ali")` | `name = 'Ali'` | `name = $1` + params=["Ali"] |
| `Q(a="x") \| Q(b="y")` | `(a = 'x') OR (b = 'y')` | `(a = $1) OR (b = $2)` + params=["x","y"] |

### What's NOT Parameterized (by design)

- **Table names**: Always controlled by the compiler, never user input
- **Column names**: Same — from model field definitions
- **ORDER BY**: Column names, not values
- **LIMIT/OFFSET**: Integer constants
- **F expressions**: Column references (`F("price") * 1.1` → `price * 1.1`)
- **IS NULL**: No value to parameterize

## Debug Queries

Configure in `desi.mod`:

```ini
[database]
engine = "postgres"
debug_queries = true
```

When enabled, every parameterized query prints to stderr:

```
[db] QUERY: SELECT * FROM users WHERE name = $1 AND age > $2
[db] PARAMS: ["Alice", "21"]
```

## SQL Injection Prevention

The parameterized approach prevents these attacks:

```desi
# Attack: value contains SQL injection
User.objects.filter(name="'; DROP TABLE users; --")

# Generated SQL (SAFE):
#   SELECT * FROM users WHERE name = $1
#   params = ["'; DROP TABLE users; --"]
#
# The database treats the entire param as a string value.
# It never executes the DROP TABLE.
```

## File Map

| File | Role |
|------|------|
| `runtime/db/crud.c` | Parameter accumulator, placeholder generation, Q object params |
| `runtime/db/dispatch.c` | `__db_query_params` / `__db_execute_params` dispatch |
| `runtime/db/postgres.c` | `__pg_query_params` — PG Extended Query Protocol |
| `runtime/db/mysql.c` | `__my_query_params` — client-side escaping |
| `compiler/internal/project/manifest.go` | `Database.DebugQueries` config |
| `compiler/internal/lower/module_lower.go` | `__db_set_debug_queries` injection |
| `compiler/lib/db.desi` | Extern declarations for new C functions |
