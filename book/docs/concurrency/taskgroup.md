# TaskGroup in Desi

TaskGroup provides structured concurrency for managing spawned tasks. It ensures all spawned tasks complete before the group is destroyed.

## Quick Start

```desi
import sync

def worker() -> none:
    print("Working...")

def main() -> int:
    using tg = sync.TaskGroup():
        tg.run(worker)  # Spawn worker concurrently
        tg.wait()       # Wait for all tasks to complete
    return 0
```

> ⚠️ **Required:** TaskGroup **must** be used with `using` guard. Using `let tg = sync.TaskGroup()` is a compile error!

## Creating Task Groups

TaskGroup uses RAII (Resource Acquisition Is Initialization) for safety:

```desi
import sync

# ✅ CORRECT: using guard (required)
using tg = sync.TaskGroup():
    tg.run(my_worker)
    tg.wait()

# ❌ WRONG: compile error DSY0001
let tg = sync.TaskGroup()  # Error: TaskGroup must be used with 'using' guard
```

### Why is `using` Required?

1. **Automatic cleanup** - TaskGroup resources are freed when scope ends
2. **Memory safety** - Prevents leaks if you forget to wait
3. **Clear lifetime** - Tasks belong to the enclosing scope

## Methods

### run(fn)

Spawns a function to run concurrently in the task group:

```desi
def my_worker() -> none:
    print("Hello from worker!")

using tg = sync.TaskGroup():
    tg.run(my_worker)   # Spawns my_worker
    tg.run(my_worker)   # Spawns another copy
    tg.wait()           # Wait for both to finish
```

**Function requirements:**
- Must take **no arguments**
- Must return **`none`**

### wait()

Blocks until all tasks in the group complete:

```desi
using tg = sync.TaskGroup():
    tg.run(worker1)
    tg.run(worker2)
    tg.wait()  # Blocks until worker1 AND worker2 finish
print("All workers done!")
```

### cancel()

Marks the group as cancelled. Tasks should check `is_cancelled()`:

```desi
using tg = sync.TaskGroup():
    tg.cancel()  # Mark as cancelled
```

### is_cancelled()

Check if the group has been cancelled:

```desi
using tg = sync.TaskGroup():
    if tg.is_cancelled():
        print("Group was cancelled")
```

## Complete Example

```desi
import sync

def task1() -> none:
    print("Task 1 running")
    print("Task 1 done")

def task2() -> none:
    print("Task 2 running")
    print("Task 2 done")

def main() -> int:
    print("Starting...")
    
    using tg = sync.TaskGroup():
        tg.run(task1)
        tg.run(task2)
        print("Tasks spawned, waiting...")
        tg.wait()
    
    print("All tasks complete!")
    return 0
```

Output (order may vary due to concurrency):
```
Starting...
Tasks spawned, waiting...
Task 1 running
Task 2 running
Task 1 done
Task 2 done
All tasks complete!
```

## Error Messages

| Code | Meaning | Fix |
|------|---------|-----|
| `DSY0001` | TaskGroup without `using` | Wrap with `using tg = sync.TaskGroup():` |

## Import Styles

Both import styles work:

```desi
# Module import
import sync
using tg = sync.TaskGroup():
    ...

# Direct import
from sync import TaskGroup
using tg = TaskGroup():
    ...
```

## See Also

- [Mutex](./mutex.md) - For protecting shared data
- [Channel](./channel.md) - For message passing between tasks

## Current Limitations

- Worker functions cannot capture outer scope variables (closures coming soon)
- Functions must take no arguments
