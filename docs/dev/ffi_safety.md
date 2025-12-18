# FFI Safety Design Document

**Status**: DRAFT - Research & Design Phase
**Priority**: Medium (implement after concurrency primitives)

---

## Motivation: The Rust Binder Bug

In December 2025, a race condition was discovered in the Linux kernel's Rust binder implementation ([kernel commit](https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?id=3e0ae02ba831da2b707905f4e602e43f8507b8cc)).

### The Bug

```rust
// OLD CODE (BUGGY):
let death_list = core::mem::take(&mut guard.death_list);  // Move to local
drop(guard);  // Release lock
for death in death_list {  // Iterate local copy
    // Other threads can still call unsafe { list.remove(self) }
    // on the ORIGINAL list, causing data race on prev/next pointers
}
```

### The Fix

```rust
// NEW CODE (FIXED):
while let Some(death) = guard.death_list.pop_front() {  // Pop while locked
    drop(guard);  // Now safe to release
    death.into_arc().set_dead();
    guard = self.owner.inner.lock();  // Re-acquire for next iteration
}
```

### Root Cause Analysis

1. **Intrusive linked lists**: Node prev/next pointers are inside the node, not managed externally
2. **Lock scope mismatch**: Data moved out before lock released, but other threads still reference original
3. **`unsafe` bypassing Rust's borrow checker**: The `remove()` was marked unsafe but assumptions were wrong

---

## Implications for Desi

Desi supports FFI/ABI for calling C/C++ libraries. Similar bugs can occur when:

### Vulnerable Scenarios

| Scenario | Risk | Example |
|----------|------|---------|
| **C library with internal pointers** | High | C code manages linked structures; Desi can't track |
| **Concurrent access via FFI** | High | C callback modifies shared state while Desi iterates |
| **Arena freed with live references** | Medium | Arena allocator destroyed; dangling refs remain |
| **Callback from C to Desi** | Medium | C holds stale function pointer after Desi closure freed |
| **Move-then-access patterns** | Medium | Data ownership transferred but not synchronization |

### Current Desi Safety Model

| Feature | Status | Protection Level |
|---------|--------|------------------|
| No raw pointers for users | ✅ | High |
| Reference counting | ✅ | Medium |
| No intrusive data structures | ✅ | High (for now) |
| FFI calls "trusted" | ⚠️ | Low - no verification |
| No thread safety primitives | ⚠️ | High risk if threading added |

---

## Proposed Safety Enhancements

### 1. `unsafe` Blocks for FFI

Mark FFI calls explicitly dangerous:

```desi
# Current (implicit trust):
let ptr = c_get_pointer()

# Proposed (explicit danger zone):
unsafe:
    let ptr = c_get_pointer()
    c_process(ptr)  # Only allowed in unsafe block
```

**Implementation**:
- Add `KW_unsafe` token
- Parse `unsafe:` block statement
- Type checker flags FFI calls outside unsafe blocks (warning? error?)
- Lowering unchanged (safety is for human awareness)

### 2. FFI Safety Annotations

Document C function safety contracts:

```desi
@ffi(safe=false, reason="caller must hold lock")
extern def c_list_remove(list: ptr, node: ptr) -> none

@ffi(thread_safe=true)
extern def c_atomic_add(ptr: ptr, val: int) -> int
```

**Implementation**:
- Add FFI-specific annotations
- Type checker can warn about unsafe FFI in certain contexts
- Documentation generation includes safety notes

### 3. Lock-Guarded References (Future)

Prevent lock scope vs. data access mismatch:

```desi
with lock(mutex) as guard:
    let ref = guard.get_ref()  # ref lifetime tied to guard
    process(ref)
# ref cannot escape this block
```

**Implementation**:
- Requires lifetime tracking (significant complexity)
- Consider after basic concurrency primitives exist

### 4. Send/Sync Traits (Future)

For thread safety when concurrency is added:

```desi
class SafeData[T: Send + Sync]:
    value: T
    
# Compile error if T is not thread-safe
let bad: SafeData[UnsafePtr] = ...  # Error: UnsafePtr is not Send
```

**Implementation**:
- Requires trait bounds system
- Consider after generics phase 2

---

## Guidelines for Avoiding the Rust Binder Bug Pattern

### Pattern to Avoid ❌

```
1. Acquire lock
2. Move data out of shared structure
3. Release lock
4. Process moved data (other threads still reference original!)
```

### Safe Pattern ✅

```
1. Acquire lock
2. Pop/process ONE item
3. Release lock
4. Do work on item
5. Re-acquire if more items
```

### For Desi Implementation

If we add intrusive data structures:
- **Document mutation requirements** (must hold lock)
- **Use pop instead of take+iterate**
- **Consider copy-out instead of move** for small data
- **Add runtime assertions** in debug mode

---

## Implementation Roadmap

### Phase 1: Awareness (Immediate)
- [ ] Document FFI safety in contributor guide
- [ ] Add warnings in docs about C library assumptions
- [ ] Create example showing proper FFI usage

### Phase 2: Annotations (Short-term)
- [ ] Add `@ffi` annotation support
- [ ] Implement `safe`, `thread_safe` flags
- [ ] Generate safety documentation

### Phase 3: `unsafe` Blocks (Medium-term)
- [ ] Add `unsafe` keyword
- [ ] Parse unsafe blocks
- [ ] Flag FFI outside unsafe (configurable)

### Phase 4: Lock Guards (Long-term)
- [ ] Implement after concurrency primitives
- [ ] Requires lifetime tracking
- [ ] Consider Rust-style approach

---

## References

- [Rust Binder Bug Commit](https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?id=3e0ae02ba831da2b707905f4e602e43f8507b8cc)
- Rust [`unsafe` Guidelines](https://doc.rust-lang.org/nomicon/meet-safe-and-unsafe.html)
- [Send and Sync Traits](https://doc.rust-lang.org/book/ch16-04-extensible-concurrency-sync-and-send.html)
