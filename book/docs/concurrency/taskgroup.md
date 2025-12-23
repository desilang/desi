# TaskGroup - Structured Concurrency

**TaskGroup** ensures all spawned tasks complete before continuing. This prevents "fire and forget" bugs where tasks might outlive their scope.

## Creating a TaskGroup

```desi
let group = taskgroup_new()
```

## Spawning Tasks

Use `spawn()` to add async work to the group:

```desi
def worker():
    print("Working...")

let group = taskgroup_new()
group.spawn(worker)
group.spawn(worker)
group.spawn(worker)
```

All spawned tasks run concurrently.

## Waiting for Completion

Use `wait()` to block until all tasks finish:

```desi
group.wait()  # Blocks until all workers complete
print("All done!")
```

## Complete Example

```desi
def process_item(id: int):
    print("Processing " + str(id))

def main() -> int:
    let group = taskgroup_new()
    
    # Spawn multiple workers
    group.spawn(lambda: process_item(1))
    group.spawn(lambda: process_item(2))
    group.spawn(lambda: process_item(3))
    
    # Wait for all to complete
    group.wait()
    
    print("All items processed!")
    return 0
```

## Cancellation

Cancel all tasks in a group:

```desi
group.cancel()

# Tasks can check if cancelled
if group.is_cancelled():
    return  # Exit early
```

## When to Use TaskGroup

Use TaskGroup when:
- You need parallel work but must wait for all to finish
- Child tasks shouldn't outlive the parent scope
- You want automatic cleanup on errors

## Comparison with Other Primitives

| Feature | TaskGroup | Channel | Mutex |
|---------|-----------|---------|-------|
| Purpose | Structured spawning | Message passing | Shared state |
| Blocking | `wait()` | `send()`/`recv()` | `lock()` |
| Best for | Fork-join patterns | Pipelines | Caches |

## See Also

- [Channels](./channels.md) - Message passing
- [Mutex](./mutex.md) - Shared state protection
