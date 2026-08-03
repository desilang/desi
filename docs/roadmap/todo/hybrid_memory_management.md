# Hybrid Memory Management

**Status**: Phases 1 and 2 shipped. Phase 3 (manual `arena.scope()`) not started.
**Related**: Generics implementation, Move semantics, Borrow checker

> **Overview:** Phase 1 (collection drops) is complete. Phase 2 is complete as
> of `594e7d47`: non-escaping local collections are promoted to bump arenas,
> types with custom destructors are excluded so cleanup still runs, the escape
> analysis no longer loses a value because an `int` was assigned nearby, and a
> loop whose allocations cannot outlive one iteration rewinds the arena at its
> latch instead of growing it every pass.
>
> Read the results before reaching for this design again: it made the arena
> **safe to leave on**, and it did **not** make allocation fast. `alloc_churn`
> is 14 ms against C's 1 ms, essentially where it was before any of this. glibc
> already serves a repeated small alloc/free about as fast as a bump allocator
> can. The remaining gap is per-element call overhead, not allocation strategy.
> Full numbers in [roadmap.md](../roadmap.md#what-actually-happened-with-a--b).

## How arena promotion decides (v0.1.x items A and B, shipped)

A local of heap type goes in the arena when all three hold:

1. **It does not escape the function.** Returned, stored in a field or a global,
   or handed to a user-defined function that might retain it — any of those and
   it stays on the heap.
2. **It does not grow.** `realloc` releases or extends the block it replaces; a
   bump allocator can only hand out a new one and abandon the old, so a list
   appended to a million times would leave every intermediate backing array in
   the arena. Any method call on it, and any assignment through an index, keeps
   it on the heap. This one cost `list_ops` 8 MB before it was added.
3. **Its type has no custom destructor**, so nothing is skipped at cleanup.

Assigning to an `int` no longer counts as escaping. It used to: `escape.go`
marked every symbol on the right-hand side as escaping whenever the target was
not itself a candidate, so

```desi
let t = [i, i, i]
total := total + len(t)     # t was marked escaping by association
```

fell back to malloc while the same loop ending in `print(str(len(t)))` did not.
A value of scalar type cannot carry a pointer out of a function, so nothing
escapes through it.

### Loops rewind rather than grow

A function-scoped arena is wrong for a loop on its own: 500,000 iterations
allocating a small list each would grow it 500,000 times and release none of it
until the function returned. Fixing the escape analysis without fixing this was
measured at 30 ms / 55.1 MB on `alloc_churn` against 13 ms / 1.8 MB before it —
which is why the two shipped as one change.

So a loop marks the arena on the way in and rewinds to that mark at the end of
every pass, and the same bytes serve every iteration. `__arena_mark` /
`__arena_rewind` / `__arena_release` are in `arena.c`; rewind deliberately
leaves the mark in place, because the latch returns to it repeatedly, and
release retires it from the loop's exit block so that `break` retires it too.

A loop qualifies only if **nothing declared inside it flows to anything declared
outside it**, judged on the dependency edges the escape constraints already
build. The test is on the edge itself rather than on whether an arena pointer
travels along it: tracking that means following chains through heap containers,
and a missed link is a use-after-free rather than a missed optimisation.

The shape this exists to refuse:

```desi
let mut keep: list[list[int]] = []
for i in range(10):
    let t = [i]
    keep.append(t)   # t outlives its iteration, so this loop never rewinds
```

Changing any of this: build with `-DDESI_ARENA_POISON` and run the example
suite. See [memory-model.md](../../memory-model.md#loops-reuse-their-scratch-memory).

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

**Shipped.** See "How arena promotion decides" above for what was actually
built; the sketch below understated it in two ways worth keeping on record.

```desi
def process_items(items: List<T>) -> int:
    for item in items:
        # New arena scope per iteration
        let processed = expensive_generic_op(item)
        # processed freed at end of iteration
    # Loop completed with constant memory usage!
```

It is not a reset — the function's arena also holds allocations made before the
loop, so it needs a mark at loop entry and a rewind to that mark at the latch.
And "freed at end of iteration" is a claim that has to be checked, not assumed:
nothing declared outside the loop may hold a pointer into the rewound region,
which is a finer question than whether a value escapes the function.

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

1. **Performance**: bump allocation is a pointer increment — but this was
   measured after Phase 2 shipped and the ~10x this document used to claim is
   not there. Against glibc, a repeated small alloc/free is served from the
   tcache about as fast as a bump allocator can manage; `alloc_churn` came out
   at 14 ms with arenas against 13 ms without. Arenas buy predictable
   reclamation, not speed.
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
