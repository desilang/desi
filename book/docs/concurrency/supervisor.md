# Supervisor in Desi

Supervisor provides a thread pool with auto-restart capabilities, inspired by Elixir/OTP supervision trees. Use it when you need a pool of workers processing tasks, or long-running processes that automatically recover from crashes.

## Quick Start

```desi
import sync

def worker():
    print("Working!")

def main() -> int:
    using sup = sync.Supervisor():
        sup.submit(worker)   # Enqueue task to the pool
        sup.submit(worker)   # Another task
        sup.stop()           # Drain queue, join threads
    return 0
```

> ⚠️ **Required:** Supervisor **must** be used with `using` guard. Using `let sup = sync.Supervisor()` is a compile error!

## Two Kinds of Work

### Pool Tasks (`submit`)

Short-lived tasks that run once and complete. The supervisor maintains a pool of worker threads that pick up tasks from a shared queue:

```desi
using sup = sync.Supervisor():
    # These tasks run concurrently on pool threads
    sup.submit(process_request)
    sup.submit(process_request)
    sup.submit(process_request)
    sup.stop()
```

**Use for:** request handling, batch processing, parallel computations.

### Persistent Children (`start_child`)

Long-running tasks that are automatically restarted if they crash:

```desi
def heartbeat_loop():
    # This runs forever, sending heartbeats
    # If it crashes, the supervisor restarts it
    print("Heartbeat sent")

using sup = sync.Supervisor():
    sup.start_child(heartbeat_loop)   # Auto-restarts on crash
    # ... supervisor keeps running ...
    sup.stop()
```

**Use for:** background workers, connection managers, health monitors.

**Restart limits:** Max 5 restarts per 60-second window. If a child exceeds this limit, it stops being restarted (prevents infinite crash loops).

## Methods

### submit(fn)

Enqueue a function to run on the thread pool:

```desi
using sup = sync.Supervisor():
    sup.submit(my_worker)    # Runs on a pool thread
    sup.stop()
```

### start_child(fn)

Start a persistent child that auto-restarts on crash:

```desi
using sup = sync.Supervisor():
    sup.start_child(my_service)   # Restarted if it exits
    sup.stop()
```

### stop()

Gracefully shut down the supervisor:
1. Signals all workers and children to stop
2. Drains the work queue (remaining tasks still execute)
3. Joins all threads
4. Frees all resources

```desi
using sup = sync.Supervisor():
    sup.submit(task)
    sup.stop()  # Safe: remaining tasks finish first
```

> 💡 `stop()` is **idempotent** — safe to call multiple times. The automatic `__close__` cleanup also calls stop, so there's no double-free risk.

### pool_size()

Get the number of pool worker threads:

```desi
using sup = sync.Supervisor():
    print(sup.pool_size())  # 4 (default)
```

### child_count()

Get the number of persistent children:

```desi
using sup = sync.Supervisor():
    sup.start_child(service_a)
    sup.start_child(service_b)
    print(sup.child_count())  # 2
    sup.stop()
```

### is_running()

Check if the supervisor is still active:

```desi
using sup = sync.Supervisor():
    print(sup.is_running())   # true
    sup.stop()
    print(sup.is_running())   # false
```

## Complete Example

```desi
import sync

def worker():
    print("Worker task executed")

def main() -> int:
    print("Creating supervisor with 4 workers...")
    using sup = sync.Supervisor():
        print("Pool size: " + str(sup.pool_size()))
        print("Is running: " + str(sup.is_running()))

        # Submit tasks to the pool
        sup.submit(worker)
        sup.submit(worker)
        sup.submit(worker)

        print("Stopping supervisor...")
        sup.stop()

    print("Supervisor test complete!")
    return 0
```

Output:
```
Creating supervisor with 4 workers...
Pool size: 4
Is running: 1
Worker task executed
Worker task executed
Worker task executed
Stopping supervisor...
Supervisor test complete!
```

## Why `using` is Required

1. **Automatic cleanup** — Threads are joined and memory freed when scope ends
2. **Memory safety** — Prevents thread leaks if you forget to stop
3. **Clear lifetime** — Workers belong to the enclosing scope

## Supervisor vs TaskGroup

| Feature | TaskGroup | Supervisor |
|---------|-----------|------------|
| Purpose | Run tasks and wait | Thread pool + auto-restart |
| Workers | Per-task threads | Fixed pool size (reused) |
| Auto-restart | No | Yes (persistent children) |
| Work queue | No | Yes (ring buffer) |
| Best for | Fan-out/fan-in | Server workers, services |

## Import Styles

Both import styles work:

```desi
def worker():
    print("worker")

# Module import
import sync
using sup = sync.Supervisor():
    sup.submit(worker)
    sup.stop()

# Direct import
from sync import Supervisor
using sup2 = Supervisor():
    sup2.submit(worker)
    sup2.stop()
```

## Error Messages

| Code | Meaning | Fix |
|------|---------|-----|
| `DSY0001` | Supervisor without `using` | Wrap with `using sup = sync.Supervisor():` |

## See Also

- [TaskGroup](./taskgroup.md) — For structured fan-out/fan-in concurrency
- [Mutex](./mutex.md) — For protecting shared data
- [Channel](./channel.md) — For message passing between tasks

## Current Limitations

- `ONE_FOR_ALL` strategy is stubbed (only `ONE_FOR_ONE` works fully)
- Restart limits are compile-time defaults (not configurable per instance yet)
- No supervision trees (nested supervisors) yet
