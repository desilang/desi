# Arena Memory Management

Arena allocation lets you allocate multiple objects and free them all at once when a scope ends.

## Basic Usage

```desi
def main() -> int:
    using arena:
        let buffer = arena.alloc(1024)  # Allocate 1KB
        let data = arena.alloc(256)     # Allocate 256 bytes
        # Use buffer and data...
    # All arena memory freed automatically here
    return 0
```

## Benefits

- **Fast allocation**: Simple bump pointer (constant time)
- **Bulk deallocation**: All memory freed at once
- **No memory leaks**: Scope-based automatic cleanup
- **No use-after-free**: Memory valid for entire scope

## Multiple Allocations

```desi
def process_batch() -> none:
    using arena:
        let header = arena.alloc(64)
        let body = arena.alloc(4096)
        let footer = arena.alloc(32)
        
        # All three are valid throughout the scope
        process(header, body, footer)
    # Entire arena freed here
    return
```

## Nested Arenas

Use nested arenas for different lifetimes:

```desi
def complex_work() -> none:
    using outer:
        let persistent = outer.alloc(1024)
        
        for i: int in range(100):
            using inner:
                let temp = inner.alloc(512)
                process(temp, persistent)
            # inner freed each iteration
        
        finalize(persistent)
    # outer freed here
    return
```

## Use Cases

### Request Handling

```desi
def handle_request(req: Request) -> Response:
    using arena:
        let parsed = parse_json(req.body, arena)
        let result = process(parsed, arena)
        return result.clone()  # Clone to escape arena
```

### Game Loops

```desi
def game_loop():
    while running:
        using frame:
            let entities = frame.alloc(1024)
            update_physics(entities)
            render(entities)
        # Frame data freed - no GC pause!
```

## Rules

| Do | Don't |
|----|-------|
| Allocate temporary data | Store arena pointers in objects |
| Nest arenas for lifetimes | Use for long-lived data |
| Clone data that escapes | Expect individual frees |

## Quick Reference

```desi
using arena:              # Create arena
    let p = arena.alloc(size)  # Allocate bytes
    # Use p...
# Automatic cleanup
```
