# Query Engine Improvements

**Status**: ✅ Implemented  
**Since**: v0.10  
**Related**: [ORM Models](orm_models.md), [Parameterized Queries](parameterized_queries.md), [File Migrations](file_migrations.md)

---

## Overview

Phase 4 adds five major query engine capabilities to the ORM QuerySet system:

1. **HAVING clause** — aggregate filtering after GROUP BY
2. **Multi-column ordering** — append-mode ORDER BY
3. **Multi-column updates** — batch SET in a single SQL statement
4. **Pagination helpers** — page-based LIMIT/OFFSET with total count
5. **Soft delete** — mark rows deleted without removing them

All features use parameterized queries and support both PostgreSQL and MySQL dialects.

---

## Architecture

### C Runtime (`compiler/runtime/db/crud.c`)

New state fields added to the QuerySet struct (originally global, now per-handle since the [handle-based refactor](queryset_handles.md)):

| Field | Type | Purpose |
|---|---|---|
| `having[2048]` | `char[]` | Accumulated HAVING conditions |
| `soft_delete` | `int` | Soft-delete mode flag (0/1) |
| `include_deleted` | `int` | Include-deleted flag (0/1) |
| `soft_delete_col[128]` | `char[]` | Column name for soft-delete (default: `is_deleted`) |
| `update_keys` | `char**` | Multi-update column names (dynamic) |
| `update_count` | `int` | Number of accumulated update fields |
| `update_param_start` | `int` | Param index where update values begin |

All cleared by `__qs_free()` (or `__qs_reset()` for legacy code paths).

### SQL Generation

`__qs_fetch()` was modified to:

1. **Auto-inject soft-delete filter**: If `qs_soft_delete` is active and `qs_include_deleted` is false, `WHERE is_deleted = FALSE` is appended (or AND-ed to existing WHERE).
2. **Emit HAVING**: After GROUP BY, before ORDER BY.

### Desi API (`compiler/lib/db.desi`)

12 new extern C bindings + 12 public API wrappers with docstrings.

---

## Functions

### HAVING Clause

| Function | Signature | Description |
|---|---|---|
| `having(condition)` | `str → int` | Add raw HAVING condition |
| `having_val(expr, val)` | `str, str → int` | Parameterized HAVING (safe for user input) |

**Implementation**: `__qs_having()` appends to `qs_having` buffer. `__qs_having_val()` binds the value via `add_param_copy()` and writes a placeholder.

### Multi-column ORDER BY

| Function | Signature | Description |
|---|---|---|
| `order_by_add(col)` | `str → int` | Append ORDER BY column (prefix `-` for DESC) |

**Implementation**: `__qs_order_by_add()` appends `col ASC` or `col DESC` to `qs_order`, comma-separated.

**Difference from `sort_by()`**: `sort_by()` overwrites the ORDER BY clause; `order_by_add()` appends to it.

### Multi-column UPDATE

| Function | Signature | Description |
|---|---|---|
| `update_set(col, val)` | `str, str → int` | Accumulate a col=val pair |
| `update_exec()` | `→ int` | Execute UPDATE with all accumulated fields |

**Implementation**: Uses an accumulator pattern (like INSERT fields). `update_set()` stores column names in `qs_update_keys[]` and values via `add_param_copy()`. `update_exec()` generates `UPDATE table SET col1=$1, col2=$2 ...` and resets the accumulator.

### Pagination

| Function | Signature | Description |
|---|---|---|
| `paginate(page, page_size)` | `int, int → int` | Set LIMIT/OFFSET from page number |
| `total_count()` | `→ int` | COUNT(*) ignoring LIMIT/OFFSET |

**Implementation**: `paginate()` computes `qs_limit = page_size` and `qs_offset = (page - 1) * page_size`. `total_count()` runs a separate `SELECT COUNT(*)` query with the current WHERE (including soft-delete filter) but no LIMIT/OFFSET.

### Soft Delete

| Function | Signature | Description |
|---|---|---|
| `soft_delete_mode(col)` | `str → int` | Enable soft-delete; set column name |
| `with_deleted()` | `→ int` | Include deleted rows in queries |
| `soft_delete()` | `→ int` | Mark rows deleted (UPDATE SET col=TRUE) |
| `hard_delete()` | `→ int` | Permanently DELETE (ignores soft-delete) |
| `restore()` | `→ int` | Restore deleted rows (SET col=FALSE) |

**Implementation**: `soft_delete_mode()` sets `qs_soft_delete=1` and stores the column name. The auto-filter injection happens in `__qs_fetch()` (line ~654) and `__qs_total_count()`. `soft_delete()` generates an UPDATE instead of DELETE. `hard_delete()` always generates a real DELETE.

---

## Design Decisions

1. **Auto-injection for soft-delete**: Rather than requiring users to manually add `WHERE is_deleted = FALSE` to every query, the filter is automatically injected in `__qs_fetch()`. This mirrors Django's custom manager pattern.

2. **Accumulator pattern for multi-update**: Matches the existing INSERT field accumulator (`qs_insert_keys[]`). This keeps the API consistent: accumulate fields, then execute.

3. **`having_val()` for safety**: Raw `having()` is provided for convenience with literal conditions, but `having_val()` should be preferred when the value comes from user input (parameterized via `add_param_copy()`).

4. **Pagination is 1-based**: Page numbers start at 1, matching typical web framework conventions (Django Paginator, Laravel, etc.).

---

## Testing Notes

- All functions compile and link correctly via `go build ./...`
- Soft-delete auto-filter is tested by verifying the generated SQL includes `WHERE is_deleted = FALSE`
- Multi-column update generates correct parameter indexes even when WHERE clause parameters precede UPDATE values
- HAVING clause is emitted between GROUP BY and ORDER BY in the generated SQL
