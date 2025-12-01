# TODO: Hybrid Memory Management for Generics

**Status**: Proposed  
**Priority**: High  
**Related**: Generics implementation, Move semantics, Borrow checker

## Current State

Desi already has excellent Rust-inspired memory management:
- ✅ Move semantics with ownership tracking
- ✅ Borrow checker (`inout`, `ref` parameters)
- ✅ Automatic memory management in most cases

**Problem**: Generic functions that box values currently use `malloc()` without `free()`, causing memory leaks.

## Proposed Solution: Arena/Region-Based Allocation

### Design

Automatically manage boxed generic values using **arena allocation** at function scope:

```desi
def main() -> int:
    # Arena created implicitly at function entry
    
    let x = identity(42)      # Boxed in arena
    let y = swap(1, 2)        # Tuple allocated in arena
    let z = process(data)     # Generic return in arena
    
    return 0
    # Arena automatically freed here - all boxed values cleaned up!
```

### Why Better Than Rust?

1. **No lifetime annotations needed** for temporary boxed values
2. **Automatic cleanup** at scope boundaries
3. **Zero GC overhead** (no tracing, no pauses)
4. **Memory safety** without manual `free()`
5. **Predictable performance** (no GC pauses like Java/Go)

### Implementation Strategy

#### Phase 1: Function-Scoped Arenas
```go
// In compiler/internal/lower/hir_lower.go

func LowerFuncFromDecl(fd *ast.FuncDecl, ...) *hir.Func {
    // Add arena.init() at function entry
    if needsArena(fd) {
        b.EmitFirst(&hir.Call{Fn: "arena.init"})
        
        // Add arena.free() before each return
        instrumentReturns(b, &hir.Call{Fn: "arena.free"})
    }
}

// Use arena.alloc instead of malloc for generic boxing
if isGenericContext {
    boxPtr := ls.b.FreshTemp("box")
    ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "arena.alloc", Args: size, Type: "ptr"})
}
```

#### Phase 2: Scope-Based Arenas
```desi
def process_items(items: List<T>) -> int:
    for item in items:
        # New arena scope per iteration
        let processed = expensive_generic_op(item)
        # processed freed at end of iteration
    # Loop completed with constant memory usage!
```

#### Phase 3: Manual Control (Advanced)
```desi
# Explicit arena for performance-critical code
with arena.scope():
    # All allocations in this arena
    let results = parallel_map(data, fn)
    # Explicitly freed at end of scope
```

### Runtime Support Needed

Add to `runtime/arena.c`:
```c
// Thread-local arena
__thread void* arena_ptr = NULL;
__thread size_t arena_used = 0;
__thread size_t arena_capacity = 0;

void* arena_alloc(size_t size) {
    // Bump allocator - extremely fast
    if (arena_used + size > arena_capacity) {
        arena_grow();
    }
    void* result = arena_ptr + arena_used;
    arena_used += size;
    return result;
}

void arena_free() {
    // Single operation - free entire arena
    if (arena_ptr) {
        free(arena_ptr);
        arena_ptr = NULL;
        arena_used = 0;
    }
}
```

### Benefits

1. **Performance**: Bump allocation is ~10x faster than malloc
2. **Safety**: No use-after-free, no double-free, no leaks
3. **Simplicity**: No lifetime annotations, no manual free
4. **Predictable**: Deterministic memory usage
5. **Cache-friendly**: Sequential allocation improves cache hits

### Fallback: Reference Counting

For values that escape function scope:
```desi
def create_box() -> Box<int>:
    return Box(42)  # Escapes - use RC instead of arena

def use_box():
    let b = create_box()  # RC incremented
    print(b.value)
    # RC decremented, freed when count reaches 0
```

## Task Breakdown

- [ ] Implement arena allocator in runtime
- [ ] Modify generic boxing to use `arena.alloc`
- [ ] Add arena init/free to function lowering
- [ ] Implement escape analysis to detect arena-safe values
- [ ] Add RC for escaping values
- [ ] Test with benchmark suite
- [ ] Document memory model for users

## References

- Rust: Ownership model (but requires lifetimes)
- Swift: ARC (but has GC overhead)
- Zig: Arena allocators (but manual)
- **Desi**: Best of all - automatic arenas + borrow checker!
