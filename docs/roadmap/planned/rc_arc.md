# Rc/Arc Smart Pointers - Design Notes (Shipped)

## Status: SHIPPED in v0.1.0

**Decision:** Rc and Arc type checking, LLVM lowering, and runtime management have been fully implemented in the compiler as first-class types (`rc[T]` and `arc[T]`).

## Details

1. **First-class syntax & constructors**
   - Use `rc(value)` to construct an `rc[T]` pointer
   - Use `arc(value)` to construct an `arc[T]` pointer

2. **Access & Cloning**
   - Call `.get()` to access the inner value
   - Call `.clone()` to clone the reference and increment reference counts

3. **Runtime & Cleanup**
   - Lowered to `__rc_new`, `__rc_clone`, `__rc_get` calls
   - Scope end triggers automatic decref (`__rc_dec`), avoiding memory leaks

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
