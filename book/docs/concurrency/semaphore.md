# Semaphore

A **Semaphore** is a counting semaphore that limits how many things can happen at once. Think of it like a parking lot with a limited number of spaces.

## Basic Usage

```desi
import sync

# Create a semaphore with 3 permits
let sem = sync.Semaphore(3)

# Acquire a permit (get a parking space)
sem.acquire()

# Do your work...

# Release the permit (leave the parking space)
sem.release()
```

## When to Use Semaphore

Use a semaphore when you want to:
- **Limit concurrent access** - Only allow N workers at once
- **Rate limiting** - Control how many operations happen simultaneously
- **Resource pooling** - Manage a pool of limited resources

## Methods

| Method | What it does |
|--------|-------------|
| `acquire()` | Wait for and take a permit (blocks if none available) |
| `release()` | Return a permit (wakes up one waiting thread) |
| `try_acquire()` | Try to take a permit without waiting (returns `true`/`false`) |

## Example: Limiting Concurrent Workers

```desi
import sync

# Only allow 2 workers at a time
let sem = sync.Semaphore(2)

sem.acquire()
print("Worker 1 started")
# ... do work ...
sem.release()

sem.acquire()
print("Worker 2 started")
# ... do work ...
sem.release()
```

## Non-Blocking with try_acquire

If you don't want to wait, use `try_acquire()`:

```desi
import sync

let sem = sync.Semaphore(1)
sem.acquire()  # Take the only permit

if sem.try_acquire():
    print("Got a permit!")
else:
    print("No permits available right now")
    # Do something else instead
```

## Edge Cases

### Zero Initial Count

You can start with zero permits and add them later:

```desi
let sem = sync.Semaphore(0)

# try_acquire fails - no permits yet
if not sem.try_acquire():
    print("Cannot acquire (correct)")

# Add a permit
sem.release()

# Now acquire works
sem.acquire()
```

### High Counts

Semaphores work with any count:

```desi
let sem = sync.Semaphore(100)  # 100 permits available
```

## Semaphore vs Mutex

| Feature | Semaphore | Mutex |
|---------|-----------|-------|
| Concurrent access | Up to N | Only 1 |
| Protects data | No (just counting) | Yes (guards a value) |
| Use case | Rate limiting | Data protection |

## Common Patterns

### Resource Pool
```desi
# Database connection pool with 5 connections
let pool = sync.Semaphore(5)

sem.acquire()  # Get a connection
# ... use connection ...
sem.release()  # Return connection
```

### Producer-Consumer Signaling
```desi
# Producer adds permits, consumer takes them
let ready = sync.Semaphore(0)

# Producer: signal that item is ready
ready.release()

# Consumer: wait for item
ready.acquire()
```
