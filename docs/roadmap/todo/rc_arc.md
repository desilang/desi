# Rc/Arc Smart Pointers - Design Notes (Deferred)

## Status: DEFERRED

**Decision:** Rc and Arc are deferred until stdlib requires shared ownership patterns.

## Rationale

1. **Arena + move semantics cover 90%+ of use cases**
   - Arena for batch processing, request handling, parsing
   - Move semantics for single ownership patterns

2. **Most stdlib modules don't need shared ownership**
   - `string` - operates on owned data
   - `time` - stateless functions
   - `fs` - file handles are single-owner
   - `task` - futures use move semantics

3. **Complexity trade-off**
   - Rc requires runtime counter management
   - Arc adds atomic operations overhead
   - Cycle detection adds more complexity

## When to Implement

Implement Rc when:
- Graph data structures are needed in stdlib
- Observer/callback patterns are common
- Cache/memoization with shared references

Implement Arc when:
- Threading primitives are added
- Shared state across async tasks

## Design Notes for Future

### Rc[T] - Reference Counted Pointer

```desi
class Rc[T]:
    """Single-threaded reference counted pointer."""
    
    pub def new(value: T) -> Rc[T]
    pub def clone(self) -> Rc[T]
    pub def strong_count(self) -> int
    pub def get(self) -> ref T
```

**Runtime:**
- `__rc_new(size: int) -> ptr` - allocate with counter
- `__rc_inc(ptr)` - increment counter
- `__rc_dec(ptr)` - decrement and free if zero

### Arc[T] - Atomic Reference Counted

Same API as Rc but uses atomic operations for thread safety.

### Weak[T] - Weak References

```desi
class Weak[T]:
    pub def upgrade(self) -> Option[Rc[T]]
```

Prevents cycles in self-referential structures.

## References

- Rust: `std::rc::Rc`, `std::sync::Arc`
- Swift: ARC (Automatic Reference Counting)
- C++: `std::shared_ptr`, `std::weak_ptr`
