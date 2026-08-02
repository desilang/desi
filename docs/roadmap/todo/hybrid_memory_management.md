# Hybrid Memory Management

**Status**: Phase 1 shipped. Phase 2 shipped but under-firing. Phase 3 not started.
**Related**: Generics implementation, Move semantics, Borrow checker

> **Overview:** Phase 1 (collection drops) is complete. Phase 2 (escape analysis
> and function-local arenas) is built and correct — non-escaping local
> collections are promoted to bump arenas, and types with custom destructors are
> excluded so cleanup still runs — but it **does not fire on the commonest shape
> in real code**, so the benefit is much narrower than this document previously
> claimed.

## Known gap: escape analysis over-approximates (v0.1.x item A)

These two loops differ only in the last line:

```desi
let t = [i, i, i]
print(str(len(t)))          # promoted to the arena
```
```desi
let t = [i, i, i]
total := total + len(t)     # falls back to malloc, every iteration
```

`escape.go` marks **every symbol referenced on the right-hand side** as escaping
whenever the assignment target is not itself an arena candidate. Candidates are
locals of heap type, so an `int` accumulator is not one — and `t` is marked
escaping by association, though `len(t)` yields an integer and the list itself
goes nowhere.

The over-approximation is safe: it only ever pushes values onto the heap, never
frees something still live. But it costs the `alloc_churn` benchmark most of its
gap against C (13 ms vs 1 ms on Linux), and it defeats the arena for any loop
that accumulates a number while touching a collection.

**Fix:** consult the type of the assignment target. A target that cannot hold a
reference — `int`, `float`, `bool` — cannot let anything escape through it.

**This fix was written, measured, and reverted.** It works: `alloc_churn`'s
loop-local list does start emitting `list_new_in` instead of `list_new`. But on
its own it makes the benchmark *worse* — 30 ms / 55.1 MB, against 13 ms / 1.8 MB
before and 1 ms / 1.5 MB for C. Output stayed correct; this is a performance
regression, not a miscompile.

The reason is the second gap below, and it is not incidental. Arenas are
function-scoped and freed only at function exit, so promoting an allocation that
happens 500,000 times in a loop turns 500,000 short-lived `malloc`/`free` pairs
— which a modern allocator serves from a hot free list — into 500,000 arena
slots all live at once. Growing the arena costs more than the allocator it
replaced.

**So the escape fix cannot ship without per-iteration reset.** They are one
change, not two, and the dependency runs both ways: reset needs the escape fix to
be safe, and the escape fix needs reset to be worth doing.

**Per-iteration reset** is Phase 2 below and is not built. It is also larger than
Phase 2 describes: a plain reset would free allocations made *before* the loop,
so it needs mark/rewind; and deciding where a rewind is safe needs
*iteration-scoped* escape information, which `escape.go` cannot currently express
— it answers "does this outlive the function?", not "does this outlive the
iteration?". A value stored into a container declared outside the loop clears the
first test and fails the second. See roadmap item B for the worked example and
the revised estimate.

See [roadmap.md](../roadmap.md#v01x--move-checks-from-run-time-to-compile-time),
items A and B.

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

**Not built.** This is roadmap item B, and the sketch below understates it —
see the "Known gap" section above before starting.

```desi
def process_items(items: List<T>) -> int:
    for item in items:
        # New arena scope per iteration
        let processed = expensive_generic_op(item)
        # processed freed at end of iteration
    # Loop completed with constant memory usage!
```

Two things this sketch hides. It is not a reset — the function's arena also holds
allocations made before the loop, so it needs a mark at loop entry and a rewind
to that mark at the latch. And "freed at end of iteration" is a claim the current
analysis cannot check: it must be shown that nothing declared outside the loop
holds a pointer into the rewound region, which is a finer question than the one
`escape.go` answers today.

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

Done:

- [x] Arena allocator in the runtime (`compiler/runtime/arena.c`)
- [x] Arena init/destroy in function lowering (`emitAlloc`, `currentAllocArena`)
- [x] Escape analysis to detect arena-safe values (`check/escape.go`)
- [x] Types with custom destructors excluded from promotion
- [x] Memory model documented for users (`docs/memory-model.md`)

Outstanding — see [roadmap.md](../roadmap.md#v01x--move-checks-from-run-time-to-compile-time):

- [ ] **(A)** Type-aware escape rule, so an `int` assignment target stops
      marking heap values on the right-hand side as escaping. Fixes the gap
      described at the top of this file. 1–2 days.
- [ ] **(B)** Per-iteration arena reset, so a loop body's allocations cost a
      bump and one reset. Depends on A. 3–5 days.
- [ ] Re-measure `alloc_churn` against C after A and B; it is the benchmark
      this whole design exists for, and it is the one that will show whether it
      worked.
- [ ] Generic boxing through `arena.alloc` rather than `malloc` — largely moot
      if generics are monomorphised instead (roadmap item G), which removes the
      boxing rather than relocating it.

## References

- Rust: Ownership model (but requires lifetimes)
- Swift: ARC (but has GC overhead)
- Zig: Arena allocators (but manual)
- **Desi**: Best of all - automatic arenas + borrow checker!
