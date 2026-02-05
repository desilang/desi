# FFI Safety Design Document

**Status**: PARTIALLY IMPLEMENTED - Phase 3 Complete (unsafe blocks, @ffi_struct, safe wrappers)
**Priority**: Medium

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

### Phase 1: Awareness (Immediate) ✅
- [x] Document FFI safety in contributor guide
- [x] Add warnings in docs about C library assumptions
- [x] Create example showing proper FFI usage

### Phase 2: Annotations (Short-term)
- [ ] Add `@ffi` annotation support
- [ ] Implement `safe`, `thread_safe` flags
- [ ] Generate safety documentation

### Phase 3: `unsafe` Blocks (Medium-term) ✅
- [x] Add `unsafe` keyword and parse unsafe blocks
- [x] Flag FFI outside unsafe (error by default)
- [x] Add `@ffi_struct` for C-compatible struct layouts
- [x] Add safe wrapper generation with `@extern(safe=true, c_name="...")`
- [x] Add `&` address-of operator for FFI pointer passing

---

## The `&` Address-Of Operator

The `&` operator takes the address of a value, returning a pointer type (`CPtr[T]`). This is essential for C FFI functions that require pointer arguments.

### Syntax

```desi
unsafe:
    let x: int = 42
    let ptr_x = &x   # ptr_x has type CPtr[int]
```

### Semantics

| Operation | Result Type | Description |
|-----------|-------------|-------------|
| `&x` (variable) | `CPtr[T]` | Takes address of stack variable |
| `&expr` (expression) | `CPtr[T]` | Allocates temp on stack, stores expr, returns pointer |

### Restrictions

1. **Unsafe blocks only**: The `&` operator is only valid inside `unsafe:` blocks
2. **No pointer arithmetic**: Desi does not support pointer arithmetic (`ptr + 1`)
3. **Pointer lifetimes**: Pointers are valid only while the referenced value is in scope

### LLVM Implementation

For variable references:
```llvm
%addrof = bitcast ptr %x to ptr   ; Just take address of stack slot
```

For expression results:
```llvm
%addrof.addr = alloca i64         ; Allocate temp
store i64 %expr_result, ptr %addrof.addr  ; Store value
%addrof = bitcast ptr %addrof.addr to ptr ; Return pointer
```

### Use Cases

1. **C functions with out-params**: Pass address for C to write results
2. **C functions reading values by reference**: Avoid copying large structs
3. **Safe FFI wrappers**: Internal use in generated wrapper functions

---

### Phase 4: Lock Guards (Long-term)
- [ ] Implement after concurrency primitives
- [ ] Requires lifetime tracking
- [ ] Consider Rust-style approach

---

## Appendix: Historical Critical Bugs in Mature Languages

### Lessons for Desi's Safety Design

| Bug/CVE | Language | Root Cause | Desi Mitigation |
|---------|----------|------------|-----------------|
| **Heartbleed** (CVE-2014-0160) | C/OpenSSL | Buffer over-read; no bounds checking | Desi strings/lists have length; no raw buffers |
| **Null Pointer (Billion Dollar Mistake)** | Java/C#/C++ | Uninhabited `null` value | `Option[T]` forces explicit handling |
| **Morris Worm** (1988) | C | Stack buffer overflow | No stack-allocated arrays in user code |
| **Use-After-Free** | C/C++ | Dangling pointer access | Reference counting; arena scoping |
| **ARC Retain Cycles** | Swift/ObjC | Circular strong references | *(Risk exists)* - May need `weak` refs |
| **GIL Contention** | Python | Global lock limits threading | *(Future design)* - No GIL planned |
| **Prototype Pollution** | JavaScript | Object prototype modification | No prototype chain; static types |
| **Pickle Deserialization RCE** | Python/Ruby | Arbitrary code in serialized data | *(Future)* - Safe serialization only |
| **Data Races** | Go/Rust | Concurrent mutation | *(Future)* - Need Send/Sync traits |
| **Rust Binder Race** (2025) | Rust | Lock release before data access done | See main document |

### Detailed Analysis of Relevant Bugs

#### 1. C Buffer Overflows (Heartbleed, Morris Worm)
```c
// Heartbleed: Read beyond buffer bounds
memcpy(response, payload, payload_length);  // length not validated!
```
**Desi Status**: ✅ Safe - Strings/lists track length; no raw `char*` access.

#### 2. Null Pointer Exceptions
```java
// Java: Runtime NPE
String s = null;
s.length();  // NullPointerException at runtime
```
**Desi Status**: ✅ Safe - `Option[T]` with `Some`/`None` forces handling.

#### 3. Use-After-Free
```c
// C++: Dangling pointer
int* ptr = new int(42);
delete ptr;
*ptr = 10;  // Undefined behavior!
```
**Desi Status**: ⚠️ Mostly safe - RC prevents this in pure Desi, but FFI can return dangling ptrs.

#### 4. ARC Retain Cycles (Swift)
```swift
// Swift: Cycle prevents deallocation
class A { var b: B? }
class B { var a: A? }
let a = A(); let b = B()
a.b = b; b.a = a  // Memory leak - neither freed
```
**Desi Status**: ⚠️ Risk exists - Currently no `weak` references. Add to roadmap.

#### 5. Data Races (Go)
```go
// Go: Data race on shared variable
var count int
go func() { count++ }()  // Race!
go func() { count++ }()  // Race!
```
**Desi Status**: ⚠️ Future risk - When threading added, need Send/Sync traits.

#### 6. Lock Scope Mismatch (Rust Binder)
See main document above - the direct inspiration for this design document.

### Potential Desi-Specific Vulnerabilities

| Scenario | Current Status | Recommended Action |
|----------|---------------|-------------------|
| **Arena use-after-destroy** | ⚠️ Possible | Add runtime checks in debug mode |
| **Closure captures freed variable** | ⚠️ Possible with FFI | Document closure lifetime rules |
| **Generic boxing type confusion** | ⚠️ Possible | Add type tags to boxed values |
| **RC cycle in user classes** | ⚠️ Possible | Add `weak[T]` reference type |
| **FFI callback to freed Desi closure** | ⚠️ Possible | Track closure lifetimes |

---

## References

- [Rust Binder Bug Commit](https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?id=3e0ae02ba831da2b707905f4e602e43f8507b8cc)
- [Heartbleed Explained](https://heartbleed.com/)
- [Null References: The Billion Dollar Mistake](https://www.infoq.com/presentations/Null-References-The-Billion-Dollar-Mistake-Tony-Hoare/)
- Rust [`unsafe` Guidelines](https://doc.rust-lang.org/nomicon/meet-safe-and-unsafe.html)
- [Send and Sync Traits](https://doc.rust-lang.org/book/ch16-04-extensible-concurrency-sync-and-send.html)
