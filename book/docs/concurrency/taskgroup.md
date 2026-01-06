# TaskGroup in Desi

TaskGroup provides structured concurrency for managing spawned tasks. It ensures all spawned tasks complete before the group is destroyed.

## Quick Start

```desi
import sync

def main() -> int:
    let tg = sync.TaskGroup()
    
    # Tasks would be spawned here (coming soon)
    
    tg.wait()  # Wait for all tasks to complete
    return 0
```

## Creating Task Groups

```desi
import sync

# Create using module import
let tg1 = sync.TaskGroup()

# Or use from-import
from sync import TaskGroup
let tg2 = TaskGroup()
```

## Methods

### wait()

Blocks until all tasks in the group complete:

```desi
let tg = sync.TaskGroup()
# ... spawn tasks ...
tg.wait()  # Blocks until all done
print("All tasks complete!")
```

### cancel()

Marks the group as cancelled. Tasks should check `is_cancelled()` at their await points:

```desi
let tg = sync.TaskGroup()
tg.cancel()  # Mark as cancelled
```

### is_cancelled()

Check if the group has been cancelled:

```desi
let tg = sync.TaskGroup()

if tg.is_cancelled():
    print("Group was cancelled")
else:
    print("Group is running")
```

## Example: Cancel Pattern

```desi
import sync

def main() -> int:
    let tg = sync.TaskGroup()
    
    # Check initial state
    if !tg.is_cancelled():
        print("Group active")
    
    # Cancel the group
    tg.cancel()
    
    if tg.is_cancelled():
        print("Group cancelled")
    
    # Wait still works (returns immediately if no tasks)
    tg.wait()
    
    return 0
```

## Best Practices

1. **Always call wait()** before letting the TaskGroup go out of scope
2. **Check is_cancelled()** in long-running tasks
3. **Use cancel()** for cooperative cancellation

## Limitations

- `spawn()` method is not yet implemented (coming soon)
- Currently supports wait, cancel, and is_cancelled only

## Import Styles

Both import styles work:

```desi
# Module import
import sync
let tg = sync.TaskGroup()

# Direct import
from sync import TaskGroup
let tg = TaskGroup()
```

## See Also

- [Mutex](./mutex.md) - For protecting shared data
- [Channel](./channel.md) - For message passing
- [Spawn](./spawn.md) - For creating concurrent tasks
