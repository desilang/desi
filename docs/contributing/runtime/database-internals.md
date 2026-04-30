# Database Runtime Internals

The database runtime lives in `compiler/runtime/db/` and implements pure-C
wire-protocol clients for PostgreSQL and MySQL. No external libraries
(`libpq`, `libmysqlclient`) are needed — everything is statically linked
into `libdesi.a`.

## File Map

| File | Purpose |
|------|---------|
| `postgres.c` | PostgreSQL v3 wire protocol (connect, auth, query, extended query) |
| `mysql.c` | MySQL wire protocol (connect, auth, COM_QUERY, result parsing) |
| `dispatch.c` | Unified `__db_*` API that routes to PG or MySQL based on active driver |
| `pool.c` | Connection pool with cond-var blocking acquire/release |
| `crud.c` | ORM QuerySet chain, parameter accumulation, DynBuf SQL generation |
| `dynbuf.h` | Header-only growable buffer used by crud.c and migrate.c |
| `connections.c` | Named connection registry for multi-database routing |
| `db_timeout.c` | Shared timeout globals and `db_set_timeouts()` |
| `db_timeout.h` | Timeout infrastructure: `poll()`-based connect, `SO_RCVTIMEO` for reads |
| `query.c` | Query builder (SELECT/INSERT/UPDATE/DELETE assembly) |
| `transaction.c` | BEGIN/COMMIT/ROLLBACK/SAVEPOINT lifecycle |
| `migrate.c` | Schema migration engine |
| `orm.c` | ORM model registry and DDL generation |
| `redis.c` | Redis wire protocol client |

## Compilation Model

All `.c` files compile to individual `.o` files and are archived into
`build/libdesi.a`. The Makefile auto-discovers sources via:

```makefile
RUNTIME_DB_SRCS = $(wildcard $(RUNTIME_DB)/*.c)
```

**Adding a new file:** Just create it in `compiler/runtime/db/`. The next
`make` will pick it up automatically.

## Timeout Architecture

### The Problem (Pre-Refactor)

Timeout globals were originally `static` variables inside `db_timeout.h`.
Because `libdesi.a` is a static library, each translation unit that
`#include`d the header got its own private copy. Calling
`db_set_timeouts()` from `dispatch.c` only updated that file's copy —
`postgres.c` and `mysql.c` never saw the change.

### Current Design

```
db_timeout.c          ← single definition of globals
  int g_db_connect_timeout_ms = 5000;
  int g_db_read_timeout_ms    = 30000;

db_timeout.h          ← extern declarations + inline helpers
  extern int g_db_connect_timeout_ms;
  extern int g_db_read_timeout_ms;
  static inline int db_connect_with_timeout(...) { ... }
  static inline int db_set_read_timeout(...) { ... }
```

- `db_connect_with_timeout()` uses `poll()` (not `select()`/`FD_SET`) to
  avoid `FD_SETSIZE` buffer overflow on high-numbered file descriptors.
- `db_set_read_timeout()` applies `SO_RCVTIMEO` to make `read()`/`recv()`
  time out instead of blocking forever.

### Important: Adding New Timeout Consumers

If you add a new `.c` file that needs timeout values, just
`#include "db_timeout.h"` — the `extern` declarations ensure you share the
same globals as every other file.

**Never** make timeout variables `static` in a header. That recreates the
isolation bug.

## PostgreSQL Prepared Statement Cache

### Overview

`postgres.c` maintains a fixed-size LRU cache of prepared statements to
avoid re-parsing identical SQL:

```c
#define PG_PREP_CACHE_SIZE 64

typedef struct {
    uint64_t sql_hash;                   // FNV-1a hash of SQL text
    char     stmt_name[PG_PREP_NAME_LEN]; // "s0", "s1", ...
    uint64_t lru_tick;                   // monotonic counter for eviction
    int      in_use;
} PGPrepEntry;
```

### Lifecycle

1. **Lookup** (`pg_prep_lookup`): Hash the SQL → scan cache → if hit,
   bump `lru_tick` and return the cached statement name.
2. **Insert** (`pg_prep_insert`): Find a free slot or evict the LRU entry.
   Send a `Close` message for the evicted statement, then insert new entry.
3. **Clear** (`pg_prep_clear`): Zero all entries without server
   communication. Used on `close()` and `reconnect()`.
4. **Close** (`pg_prep_close_stmt`): Send `Close('S', name) + Sync` to the
   server and drain `CloseComplete + ReadyForQuery`.

### Gotchas

- **On failed Parse:** Do NOT call `pg_prep_close_stmt()`. PostgreSQL
  auto-rolls-back failed Parse commands. Sending Close into an errored
  session risks protocol desynchronization.
- **On reconnect:** Always call `pg_free_results()` before
  `pg_prep_clear()` to prevent stale result buffers from the old session.
- **Statement names are globally unique:** `prep_seq` is never reset across
  reconnects, ensuring no name collision with server-side state.

## MySQL Parameter Interpolation

MySQL uses client-side escaping (not server-side prepared statements).
The function `my_interpolate_params()` replaces `?` placeholders with
escaped values.

### Quote-Aware Parsing

The interpolator tracks whether it is inside a single-quoted (`'...'`) or
double-quoted (`"..."`) string, with backslash-escape awareness:

```c
// Only replace ? when NOT inside a string literal
if (sql[i] == '?' && !in_single_quote && !in_double_quote) {
    // substitute parameter
}
```

This prevents misinterpreting `?` inside SQL string values like
`WHERE col = 'what?'`.

## Reconnect Lifecycle

### PostgreSQL

```
__pg_reconnect()
  ├── pg_free_results()      ← clear stale buffers
  ├── pg_prep_clear()        ← invalidate cached statements (no server I/O)
  ├── close(fd)              ← close old socket
  └── __pg_connect(...)      ← full re-auth cycle
```

### MySQL

```
__my_reconnect()
  ├── close(fd)
  └── __my_connect(...)
```

MySQL has no client-side prepared statement cache, so reconnect is simpler.

## Adding a New Driver

1. Create `compiler/runtime/db/newdriver.c`
2. Implement the standard interface functions:
   - `__nd_connect(host, port, db, user, pass)` → `int`
   - `__nd_close()` → `void`
   - `__nd_query(sql)` → `int32_t` (row count)
   - `__nd_execute(sql)` → `int32_t` (affected rows)
   - `__nd_query_params(sql, params, count)` → `int32_t`
   - `__nd_get_value(row, col)` → `const char*`
   - `__nd_get_field(row, name)` → `const char*`
3. Add routing in `dispatch.c` — extend the driver string check
4. `#include "db_timeout.h"` for connect/read timeout support
5. Run `make` — auto-discovered, no Makefile edits needed

## Connection Pool

### Architecture

`pool.c` manages a fixed-size array of opaque connection handles
(`PGConn*` or `MYConn*`). Each slot is either `SLOT_FREE` or
`SLOT_IN_USE`. The pool is protected by a single mutex + condition
variable pair.

### Acquire / Release Flow

```
acquire()                         release()
  ├── lock mutex                    ├── lock mutex
  ├── scan for SLOT_FREE            ├── mark slot FREE
  │   ├── found → claim it          ├── cond_signal (wake one waiter)
  │   │   ├── unlock mutex          └── unlock mutex
  │   │   ├── ping (outside lock)
  │   │   ├── reconnect if dead
  │   │   └── return slot
  │   └── not found
  │       └── cond_timedwait ←───── woken by signal
  └── ETIMEDOUT → return -1
```

**Key design decisions:**

- **Condition variable, not spin-wait.** `pthread_cond_timedwait` uses
  zero CPU while waiting and wakes instantly when `release()` signals.
  The old `usleep(1000)` spin burned 30,000 lock/unlock cycles during
  a 30-second timeout.
- **Ping outside the lock.** Health checks involve network I/O (SELECT 1
  or COM_PING) which can take milliseconds. Holding the mutex during
  ping would block all other threads. Instead, we claim the slot, unlock,
  ping, and re-lock only if reconnect fails.
- **Broadcast on close.** `pool_close()` sends `pthread_cond_broadcast`
  so all waiting acquirers wake up, see `g_pool_initialized == 0`, and
  return `-1` immediately.
- **Hot-reload safe.** After `close()` + `init()`, the cond-var is
  reusable (POSIX guarantees `PTHREAD_COND_INITIALIZER` state).

### Gotchas

- **Never hold the mutex while doing I/O.** The ping/reconnect path
  intentionally unlocks before calling driver functions.
- **Spurious wakeups.** `pthread_cond_timedwait` may return without a
  signal (POSIX allows this). The loop re-scans slots on every wakeup.
- **Thread-local active slot.** `g_pool_active_slot` is a per-process
  variable, not thread-local. The pool currently assumes single-threaded
  acquire/query/release cycles. For true multi-threaded pool usage,
  this should be made `__thread`.

## QuerySet Buffer Model

### Handle-Based Design

Each ORM query gets a heap-allocated `QuerySet` struct via `__qs_new()`.
The struct is installed as the thread-local active handle via
`__qs_handle_bind()` and freed after use via `__qs_handle_free()`.
This makes the runtime fully reentrant — concurrent queries on different
threads use different handles.

### Dynamic vs Fixed Buffers

| Buffer | Type | Size |
|--------|------|------|
| `params` | `char**` (realloc) | Unbounded |
| `insert_keys` | `char**` (realloc) | Unbounded |
| `update_keys` | `char**` (realloc) | Unbounded |
| `related` | `char**` (realloc) | Unbounded |
| `where_clause` | `char[4096]` | Fixed |
| `columns` | `char[4096]` | Fixed |
| `order` | `char[512]` | Fixed |
| SQL assembly (`__qs_fetch`) | `DynBuf` | Unbounded |

The parameter arrays grow dynamically via `realloc`. The WHERE clause
and column list are still fixed-size embedded arrays for `sizeof()`
compatibility with legacy shim macros. The final SQL assembly in
`__qs_fetch()` uses `DynBuf` to support arbitrarily complex queries
with many JOINs, annotations, and clauses.
