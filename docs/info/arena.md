# Arena Memory Management in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [What is Arena Allocation?](#what-is-arena-allocation)
3. [Why Arena Memory Management?](#why-arena-memory-management)
4. [Usage Guide](#usage-guide)
5. [How It Works](#how-it-works)
6. [Performance Characteristics](#performance-characteristics)
7. [Limitations and Restrictions](#limitations-and-restrictions)
8. [Use Cases](#use-cases)
9. [Comparison with Other Allocation Strategies](#comparison-with-other-allocation-strategies)
10. [Implementation Details](#implementation-details)
11. [Design Philosophy](#design-philosophy)
12. [Best Practices](#best-practices)

---

## Quick Start

```desi
def main() -> int:
    using arena:
        let buffer = arena.alloc(1024)    # Allocate 1KB
        let data = arena.alloc(256)       # Allocate 256 bytes
        # Use buffer and data...
    # All arena memory freed automatically here
    return 0
```

Key features:
- **Bulk deallocation**: All memory freed at once when scope exits
- **Fast allocation**: Simple bump pointer allocation (O(1))
- **No individual frees**: Eliminates use-after-free and double-free bugs
- **Scope-based lifetime**: Memory automatically managed by `using` block

---

## What is Arena Allocation?

Arena allocation (also called region-based memory management or bump allocation) is a memory management strategy where:

1. **Allocations are fast**: Memory is allocated by simply advancing a pointer
2. **No individual frees**: Objects cannot be freed individually
3. **Bulk deallocation**: All memory is freed at once when the arena is destroyed

```
Arena Memory Layout:
┌─────────────────────────────────────────────────────────┐
│  alloc(100)  │  alloc(256)  │  alloc(50)  │  [unused]  │
│    100B      │     256B     │     50B     │            │
└─────────────────────────────────────────────────────────┘
                                              ↑
                                         bump pointer
```

When the arena scope ends, everything is freed instantly:

```desi
using arena:
    let a = arena.alloc(100)
    let b = arena.alloc(256)
    let c = arena.alloc(50)
# Entire arena freed here - one operation!
```

---

## Why Arena Memory Management?

### Problem: Traditional Memory Management

Traditional heap allocation (`malloc`/`free`) has several challenges:

1. **Fragmentation**: Frequent alloc/free creates memory holes
2. **Overhead**: Each allocation has metadata (~16-24 bytes)
3. **Complexity**: Manual tracking of lifetimes leads to bugs
4. **Performance**: Allocator must search for suitable blocks

### Solution: Arena Allocation

Arena allocation addresses these issues:

| Issue | Traditional Heap | Arena |
|-------|-----------------|-------|
| Allocation speed | O(log n) or worse | O(1) - bump pointer |
| Memory overhead | 16-24 bytes per allocation | 0-8 bytes (alignment only) |
| Fragmentation | Progressive | None within arena |
| Deallocation | Per-object | Bulk (instant) |
| Use-after-free | Possible bug | Impossible |
| Double-free | Possible bug | Impossible |

### Benefits for Desi

1. **Simpler memory model**: No need to track individual allocations
2. **Predictable performance**: Consistent allocation times
3. **Batch processing**: Allocate many objects, free all at once
4. **Compiler-friendly**: Scope-based lifetimes integrate with RAII

---

## Usage Guide

### Basic Usage

```desi
def main() -> int:
    using arena:
        # Allocate raw bytes
        let buffer = arena.alloc(1024)
        
        # All allocations from this arena are valid until scope ends
        process_data(buffer)
        
    # Arena is destroyed here - all memory freed
    return 0
```

### Multiple Allocations

```desi
def process_batch() -> none:
    using arena:
        # Allocate multiple buffers from same arena
        let header = arena.alloc(64)
        let body = arena.alloc(4096)
        let footer = arena.alloc(32)
        
        # All three are valid throughout the scope
        write_message(header, body, footer)
    return
```

### Nested Arenas

```desi
def complex_processing() -> none:
    using outer_arena:
        let persistent_data = outer_arena.alloc(1024)
        
        for i in range(100):
            using inner_arena:
                # Temporary allocations for each iteration
                let temp = inner_arena.alloc(512)
                process_iteration(temp, persistent_data)
            # inner_arena freed here - every iteration!
        
        # persistent_data still valid
        finalize(persistent_data)
    # outer_arena freed here
    return
```

---

## How It Works

### Allocation Mechanism

Arena uses a **bump allocator** - the simplest and fastest allocation strategy:

```
Initial state:
┌────────────────────────────────────────┐
│                 [unused]               │
└────────────────────────────────────────┘
↑
offset = 0

After alloc(100):
┌────────────────────────────────────────┐
│  [100 bytes]  │      [unused]          │
└────────────────────────────────────────┘
                 ↑
            offset = 100

After alloc(256):
┌────────────────────────────────────────┐
│  [100 bytes]  │  [256 bytes]  │[unused]│
└────────────────────────────────────────┘
                                 ↑
                            offset = 356
```

**Algorithm:**
1. Check if `offset + size <= capacity`
2. Return pointer at current offset
3. Advance offset by aligned size

### Chunk Growth

When the current chunk is full, a new larger chunk is allocated:

```desi
# If alloc(size) exceeds current chunk:
# 1. Allocate new chunk (2x current size, minimum = size)
# 2. Link to previous chunk
# 3. Continue allocation in new chunk
```

### Scope Management

The `using arena:` syntax is lowered by the compiler to:

```desi
# What you write:
using arena:
    let x = arena.alloc(100)
    # ... use x ...

# What the compiler generates:
let arena = __arena_new(0)  # 0 = default 64KB capacity
let x = __arena_alloc(arena, 100)
# ... use x ...
__arena_destroy(arena)  # Called automatically at scope exit
```

---

## Performance Characteristics

### Allocation Speed

| Operation | Time Complexity | Typical Time |
|-----------|-----------------|--------------|
| `arena.alloc()` | O(1) | ~5-10 nanoseconds |
| `malloc()` | O(log n) | ~50-500 nanoseconds |
| `new` (GC'd) | O(1) amortized | ~10-100 nanoseconds |

### Memory Overhead

| Allocator | Per-Object Overhead |
|-----------|---------------------|
| Arena | 0-7 bytes (alignment) |
| `malloc` | 16-24 bytes (metadata) |
| GC | 8-16 bytes (header) |

### When Arena Excels

- **Many small allocations**: Minimal overhead per object
- **Batch processing**: Allocate many, free all at once
- **Known lifetimes**: All objects in scope have same lifetime
- **Performance-critical paths**: Predictable, fast allocation

### Memory Usage

```desi
# Default capacity: 64KB initial chunk
# Growth: 2x previous chunk when full
# Peak memory = sum of all allocated chunks
```

---

## Limitations and Restrictions

### What You CANNOT Do

**❌ Free Individual Objects**
```desi
using arena:
    let x = arena.alloc(100)
    let y = arena.alloc(100)
    # free(x)  # NOT POSSIBLE - no individual free
```

**❌ Extend Lifetime Beyond Scope**
```desi
let escaped_ptr: ptr
using arena:
    escaped_ptr = arena.alloc(100)  # ❌ DANGER!
# escaped_ptr is now invalid - arena destroyed
```

**❌ Reset Partially**
```desi
using arena:
    let a = arena.alloc(100)
    # arena.reset_to(a)  # NOT SUPPORTED (yet)
```

### Current Limitations

1. **Raw bytes only**: Currently returns untyped `ptr`, no typed allocation
2. **No object constructors**: Just raw memory, not initialized objects
3. **Single-threaded**: Not thread-safe (no atomic operations)
4. **No custom capacity**: Uses default 64KB (future: `arena.alloc_with_capacity`)

### Planned Features

- [ ] Typed allocations: `arena.alloc[T]()` returning `T*`
- [ ] Arena reset: `arena.reset()` to reuse without deallocation
- [ ] Custom initial capacity: `using arena(16384):`
- [ ] Thread-local arenas for concurrent code

---

## Use Cases

### Parsing and Compilers

Perfect for AST nodes that live for a parse phase:

```desi
def parse(source: str) -> AST:
    using arena:
        let tokens = tokenize(source, arena)
        let ast = parse_tokens(tokens, arena)
        return ast.clone()  # Clone result to escape arena
    # Parse temporaries freed automatically
```

### Game Development

Frame allocators for per-frame temporary data:

```desi
def game_loop():
    while running:
        using frame_arena:
            let entities = frame_arena.alloc(entity_count * 64)
            let physics = frame_arena.alloc(physics_data_size)
            update_physics(entities, physics)
            render(entities)
        # All frame data freed - no GC pause!
```

### Request Handling

Per-request allocations in web servers:

```desi
def handle_request(req: Request) -> Response:
    using arena:
        let parsed = parse_json(req.body, arena)
        let result = process_request(parsed, arena)
        let response = serialize(result, arena)
        return response.clone()
    # Request temporaries freed
```

### Data Processing Pipelines

Batch processing with intermediate buffers:

```desi
def process_batch(items: list[Item]) -> list[Result]:
    using arena:
        let intermediate = arena.alloc(items.len() * 256)
        let transformed = arena.alloc(items.len() * 128)
        
        transform(items, intermediate)
        aggregate(intermediate, transformed)
        return finalize(transformed)
```

---

## Comparison with Other Allocation Strategies

### vs. Traditional Heap (`malloc`/`free`)

| Aspect | Arena | Heap |
|--------|-------|------|
| Speed | O(1) bump | O(log n) search |
| Flexibility | Low - no individual free | High - free anything |
| Fragmentation | None within arena | Can be severe |
| Memory leaks | Impossible in scope | Easy to miss |
| Best for | Batch/phase-based | Long-lived, dynamic |

### vs. Garbage Collection

| Aspect | Arena | GC |
|--------|-------|-----|
| Latency | Predictable | GC pauses |
| Throughput | Excellent | Good |
| Memory overhead | Minimal | GC metadata |
| Programmer effort | Manual scope design | Automatic |
| Best for | Performance-critical | General apps |

### vs. Reference Counting

| Aspect | Arena | Ref Counting |
|--------|-------|--------------|
| Cycle handling | N/A | Can leak |
| Overhead | None | Counter updates |
| Speed | Fastest | Counter overhead |
| Flexibility | Scope-limited | Anywhere |
| Best for | Batch work | Shared ownership |

---

## Implementation Details

### Runtime Structure

```c
// C runtime (compiler/runtime/arena.c)
typedef struct ArenaChunk {
    char* buffer;           // Actual memory
    size_t capacity;        // Chunk size
    size_t offset;          // Current allocation offset
    struct ArenaChunk* next; // Link to next chunk
} ArenaChunk;

typedef struct {
    ArenaChunk* head;       // First chunk
    ArenaChunk* current;    // Current chunk for allocations
} DesiArena;
```

### Runtime Functions

| Function | Purpose |
|----------|---------|
| `__arena_new(capacity)` | Create new arena |
| `__arena_alloc(arena, size)` | Allocate from arena |
| `__arena_destroy(arena)` | Free all arena memory |
| `__arena_reset(arena)` | Reset for reuse (planned) |

### Compiler Integration

The compiler handles arena through several phases:

1. **Parsing**: `using arena:` recognized as UsingStmt with implicit Arena type
2. **Type Checking**: Arena type added, `arena.alloc()` method resolved
3. **Lowering**: `using` lowered to init + defer destroy pattern
4. **LLVM Backend**: Arena operations mapped to runtime calls

---

## Design Philosophy

### RAII Integration

Arena follows Desi's RAII pattern - resources are acquired and released with scope:

```desi
using arena:  # Arena created
    # ... work with arena ...
# Arena destroyed - all memory freed
```

### Explicit Memory Control

Arena gives programmers **explicit control** over allocation lifetime while preventing common bugs:

- ✅ No use-after-free (scope enforced)
- ✅ No double-free (only arena destroys)
- ✅ Predictable deallocation point
- ❌ No individual object free (by design)

### Zero-Cost Abstraction

Arena is designed as a zero-cost abstraction:

- Allocation is just pointer arithmetic
- No garbage collector overhead
- No reference counting overhead
- Compile-time scope enforcement

---

## Best Practices

### ✅ DO

```desi
# DO: Use for batch processing
using arena:
    for item in items:
        let temp = arena.alloc(item.size)
        process(item, temp)
```

```desi
# DO: Nest arenas for different lifetimes
using long_lived:
    let config = long_lived.alloc(1024)
    
    for request in requests:
        using short_lived:
            handle_request(request, short_lived, config)
```

```desi
# DO: Clone data that needs to escape
def parse() -> Result:
    using arena:
        let ast = parse_impl(arena)
        return ast.clone()  # Clone to outlive arena
```

### ❌ DON'T

```desi
# DON'T: Store arena pointers in long-lived structures
class BadClass:
    pub data: ptr  # ❌ Will be invalid when arena destroyed
    
    pub def init(arena: Arena):
        self.data = arena.alloc(64)  # Escapes scope!
```

```desi
# DON'T: Use arena for long-lived objects
using arena:
    let config = arena.alloc(1024)  # ❌ Lives for program lifetime?
    run_forever(config)  # Arena stuck until program ends
```

```desi
# DON'T: Over-allocate in tight loops
for i in range(1000000):
    using arena:  # ❌ Creating/destroying arena each iteration
        let x = arena.alloc(8)
```

### Summary

| Do | Don't |
|----|-------|
| Batch allocate, bulk free | Store arena pointers in objects |
| Nest arenas for different lifetimes | Use for singleton/global data |
| Clone data that escapes | Create arena in hot loops |
| Use for request/frame scopes | Expect individual deallocation |

---

## Implementation Status

### ✅ Fully Implemented

- Basic `using arena:` syntax
- `arena.alloc(size)` for raw byte allocation
- Automatic destruction at scope exit
- Chunk growth for large allocations
- LLVM IR generation

### 🚧 Planned

- [ ] Typed allocations: `arena.alloc[T]()`
- [ ] Arena reset: `arena.reset()`
- [ ] Custom capacity: `using arena(capacity):`
- [ ] Arena statistics: `arena.used()`, `arena.capacity()`
- [ ] Compile-time escape analysis warnings

---

## Examples Repository

**See working examples:**
- `examples/143_arena.desi` - Basic arena usage

---

**This document is the definitive reference for Arena memory management in Desi.** For compiler internals, see `compiler/runtime/arena.c` and `compiler/internal/lower/lower_call.go`.
