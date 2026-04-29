# Connection Pooling

Desi provides built-in connection pooling for database connections, enabling
efficient reuse of connections in concurrent applications.

## Overview

The connection pool maintains a fixed number of pre-established database
connections. Instead of opening and closing connections for each query,
threads acquire connections from the pool, use them, and release them back.

## Configuration

### Via `desi.mod`

```toml
[database]
engine = "postgres"
host = "localhost"
name = "myapp"
user = "admin"
password = "secret"
max_conns = "5"         # pool size
conn_timeout = "30"     # acquire timeout in seconds
```

### Via API

```python
import db

# Initialize pool: driver, host, port, dbname, user, password, pool_size
db.pool_init("postgres", "localhost", 5432, "myapp", "admin", "secret", 5)
```

## Usage

```python
import db

# Acquire a connection (blocks until available, 30s timeout)
db.pool_acquire()

# Run queries using the acquired connection
db.query("SELECT * FROM users")

# Release connection back to the pool
db.pool_release()
```

## Pool Statistics

```python
db.pool_size()       # Total connections in pool
db.pool_available()  # Free connections ready to be acquired
```

## Cleanup

```python
db.pool_close()      # Close all connections and destroy pool
```

## Thread Safety

- Pool operations are protected by `pthread_mutex`
- `pool_acquire()` blocks with a spin-wait (1ms intervals) up to 30 seconds
- If no connection is available within the timeout, acquire returns `-1`

## Architecture

```
┌──────────────────────────────────────┐
│           Connection Pool            │
│                                      │
│  ┌──────┐ ┌──────┐ ┌──────┐        │
│  │conn_0│ │conn_1│ │conn_2│  ...    │
│  │ free │ │ busy │ │ free │         │
│  └──────┘ └──────┘ └──────┘         │
│                                      │
│  mutex: pthread_mutex_t              │
│  size:  N (configured at init)       │
└──────────────────────────────────────┘
         │               │
    pool_acquire()   pool_release()
         │               │
    ┌─────────┐    ┌─────────┐
    │ Thread A│    │ Thread B│
    └─────────┘    └─────────┘
```

## Implementation

The pool is implemented in `compiler/runtime/db/pool.c` with opaque `void*`
connection handles. Each slot tracks:

- `conn`: opaque pointer to the driver-specific connection
- `in_use`: boolean flag (0 = free, 1 = acquired)

The pool works with both PostgreSQL and MySQL drivers through the unified
dispatch layer (`db/dispatch.c`).
