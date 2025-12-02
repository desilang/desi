# Class Implementation in Desi

This document provides detailed technical information about how classes are implemented in the Desi compiler. This is intended for compiler developers and advanced users who want to understand the performance characteristics and design decisions.

---

## Design Philosophy

Classes in Desi follow a **"pay only for what you use"** philosophy:
- **Static dispatch by default**: No vtables unless needed ✅ **IMPLEMENTED**
- **Zero-cost abstractions**: Methods inline like functions ✅ **IMPLEMENTED**
- **Explicit costs**: Async, copy, and dynamic features are opt-in 🚧 **PARTIAL**
- **Memory efficient**: C-compatible layout, no hidden overhead ✅ **IMPLEMENTED**

**Inspiration**: Rust's zero-cost abstractions, C's struct layout, Python's readable syntax

---

## 1. Method Dispatch Strategy

### Static Dispatch with Name Mangling

**Decision**: All method calls use static dispatch (compile-time resolution).

```desi
class Counter:
    pub value: int
    pub def increment(self) -> none:
        self.value = self.value + 1

counter.increment()  # compiles to: Counter_increment(&counter)
```

**Why static dispatch?**

1. **Zero overhead**: No runtime lookup, inlines like regular functions
2. **Cache-friendly**: Direct call, no pointer indirection
3. **Predictable performance**: Same cost as C function call
4. **LLVM optimizable**: Full visibility for inlining and optimization

**Name mangling scheme:**
```
ClassName.method_name  →  ClassName_method_name
```

### Comparison with Other Languages

| Language | Dispatch | Overhead | When resolved |
|----------|----------|----------|---------------|
| **Desi** | Static mangling | Zero | Compile time |
| C++ (`final`/non-virtual) | Static | Zero | Compile time |
| C++ (virtual) | vtable lookup | 1 indirection | Runtime |
| Rust (impl) | Static | Zero | Compile time |
| Rust (trait object) | vtable | 1 indirection | Runtime |
| Python | Dict lookup | Hash + lookup | Runtime |
| Java | Always virtual | 1 indirection | Runtime (JIT optimizes) |

**Future-proofing**: When we add traits/interfaces, trait objects will use vtables, but regular method calls remain static.

---

## 2. Memory Layout

### C-Compatible Struct Layout

**Decision**: Classes use the same memory layout as C structs.

```desi
class Point:
    pub x: int  # offset 0, size 4
    pub y: int  # offset 4, size 4
# Total size: 8 bytes
```

**Memory representation:**
```
┌──────────┬──────────┐
│ x (4B)   │ y (4B)   │
└──────────┴──────────┘
```

**Benefits:**

1. **FFI compatible**: Can pass to C/C++ code directly
2. **Predictable**: No hidden fields, padding is explicit
3. **Cache-friendly**: Sequential access, no pointer chasing
4. **SIMD-friendly**: Aligned data for vectorization

### Inheritance Layout

**Single inheritance**: Base class fields come first

```desi
class Animal:
    pub name: str  # 8 bytes (ptr)

class Dog(Animal):
    pub breed: str  # 8 bytes (ptr)
```

**Memory representation:**
```
┌────────────┬─────────────┐
│ name (8B)  │ breed (8B)  │
│ (from Base)│ (Dog field) │
└────────────┴─────────────┘
```

**Why base-first?**
- Type casting: Dog* can be cast to Animal* (pointer remains valid)
- Field access: Inherited field offsets are known at compile time

**Inspiration**: C++ standard layout, Rust's repr(C)

---

## 3. Constructor Implementation

### Zero-arg Default Constructor

**No `__new__` defined:**
```desi
class Note:
    pub title: str
    pub body: str

n = Note()  # Zero-arg only
```

**Generated LLVM IR:**
```llvm
define ptr @Note() alwaysinline {
entry:
    %ptr = call ptr @malloc(i32 16)  ; 2 pointers = 16 bytes
    ret ptr %ptr
}
```

**Optimizations:**
- **`alwaysinline`**: Disappears after inlining
- **Stack allocation**: LLVM can promote to stack if escape analysis proves safe
- **Zero-initialization**: Fields are uninitialized (caller responsible)

### User-defined `__new__` ✅ **IMPLEMENTED**

**With `__new__`:**
```desi
class Point:
    pub x: int
    pub y: int
    
    pub def __new__(px: int, py: int) -> Point:
        # Note: struct literal syntax not yet implemented
        # For now, allocate and return
        let p = Point()  # zero-arg call
        # Field initialization via methods will be added
        return p
```

**Generated IR:**
```llvm
define ptr @Point(i32 %x, i32 %y) {
entry:
    %ptr = call ptr @malloc(i32 8)
    %x_ptr = getelementptr inbounds i8, ptr %ptr, i32 0
    store i32 %x, ptr %x_ptr
    %y_ptr = getelementptr inbounds i8, ptr %ptr, i32 4
    store i32 %y, ptr %y_ptr
    ret ptr %ptr
}
```

**Constructor overloading 🚧 DESIGNED (not yet implemented):**
```desi
pub def __new__() -> Point:
    Point{ x: 0, y: 0 }

pub def __new__(x: int, y: int) -> Point:
    Point{ x: x, y: y }
```

Will compile to: `Point()` and `Point$2(i32, i32)` (arity-based mangling)

---

## 4. Field Access

### Direct Offset Access

**Decision**: Field access compiles to direct memory load/store at known offset.

```desi
p.x       # load i32, ptr %p, offset 0
p.x = 10  # store i32 10, ptr %p, offset 0
```

**Cost**: 1 memory access (same as C struct)

**No hidden getters/setters** unless explicitly defined:

```desi
class Counter:
    _value: int  # private
    
    pub def value(self) -> int:
        return self._value
    
    pub def set_value(self, v: int) -> none:
        self._value = v

c.value()        # Explicit method call
c.set_value(10)  # Explicit method call
```

**Why explicit?**
- **No surprises**: Field access is always fast
- **Performance visible**: Method calls are obvious in code
- **Opt-in encapsulation**: Use methods when needed

**Inspiration**: Rust's public fields, C structs, Zig's field access

---

## 5. Async Methods 🚧 **NOT YET IMPLEMENTED**

### State Machine Transformation (Design)

**Async methods will compile to state machines** (zero-cost async/await):

```desi
class DataLoader:
    pub url: str
    
    pub async def fetch(self) -> str:
        data = await http_get(self.url)
        return process(data)
```

**Compiled state machine:**
```
enum FetchState {
    Initial,
    AwaitingHttp(Future<str>),
    Processing(str),
    Done(str)
}

def DataLoader_fetch$poll(self, state: &mut FetchState) -> Poll<str>:
    match state:
        Initial:
            future = http_get(self.url)
            *state = AwaitingHttp(future)
            return Pending
        AwaitingHttp(future):
            data = ready!(future.poll())
            *state = Processing(data)
            continue
        Processing(data):
            result = process(data)
            *state = Done(result)
            return Ready(result)
```

**Benefits:**
- **Zero allocation**: State stored inline (if small)
- **Composable**: Can await other futures
- **Cancellation-safe**: State can be dropped
- **No green threads**: Native async/await

**`__new__` is NOT async**: Constructors allocate memory, must be synchronous.

**Inspiration**: Rust's async/await, Zig's async, C# async/await

---

## 6. RAII & Resource Management 🚧 **NOT YET IMPLEMENTED**

### Deterministic Destruction with `__close__` (Design)

**Planned**: Use scope-based cleanup (like C++ RAII)

```desi
class File:
    pub path: str
    
    pub def __close__(self) -> none:
        close_fd(self)  # Platform-specific cleanup

using f = File("data.txt"):
    content = f.read()
# f.__close__() called here automatically
```

**Compilation:**
```
1. Allocate f
2. Execute block
3. Call f.__close__() (even if error via ?)
4. Deallocate f
```

**Exception safety:**
```desi
using f = File("data.txt"):
    risky_operation()?  # If error, __close__ still called
```

**Benefits:**
- **Predictable**: Cleanup happens at scope exit
- **No GC pauses**: Deterministic, not collector-based
- **Exception-safe**: Works with `?` error propagation
- **Leak-free**: Compiler ensures cleanup

**Inspiration**: C++ RAII, Rust's Drop trait, Python's `with` statement

---

## 7. Generic Classes ✅ **TYPE INFERENCE IMPLEMENTED**

### Bidirectional Type Checking

**Current implementation**: Type inference from annotations:

```desi
class Box<T>:
    pub value: T

let b1: Box<int> = Box()  # ✅ Type inferred from annotation!
let b2: Box<str> = Box()  # ✅ Works!
```

**Generated code** (currently single constructor, monomorphization planned):
```llvm
define ptr @Box() {
    %ptr = call ptr @malloc(i32 8)
    ret ptr %ptr
}
```

### Monomorphization Strategy 🚧 **PLANNED OPTIMIZATION**

**Future**: Specialize generic classes at compile time (like C++ templates)

```desi
class Box<T>:
    pub value: T
    
    pub def get(self) -> T:
        return self.value

b1 = Box(42)      # Box<int>
b2 = Box("hello") # Box<str>
```

**Generated code:**
```llvm
; Box<int>
define ptr @Box_int(i32 %value) { ... }
define i32 @Box_int_get(ptr %self) { ... }

; Box<str>
define ptr @Box_str(ptr %value) { ... }
define ptr @Box_str_get(ptr %self) { ... }
```

**Trade-offs:**

✅ **Pros:**
- Fully optimized for each type
- No boxing for primitives
- Inlining opportunities
- No runtime type checking

❌ **Cons:**
- Code bloat (multiple copies)
- Longer compile times

**Why monomorphization over type erasure?**
- **Performance**: Critical for systems programming
- **Zero-cost**: No runtime overhead
- **Optimization**: LLVM can inline across instantiations

**Inspiration**: C++ templates, Rust generics, D's templates

---

## 8. Copy vs Move Semantics 🚧 **MOVE IMPLEMENTED, COPY PLANNED**

### Move-by-default ✅, Opt-in Copy 🚧

**Current**: Classes are moved by default (enforced by borrow checker).
**Planned**: Explicit `__copy__` for copying.

```desi
let c1 = Counter(0)
let c2 = c1  # MOVED, c1 is now invalid ✅

# Explicit copy (planned):
class Counter:
    pub def __copy__(self) -> Counter:
        Counter(self.value)

let c3 = c1.__copy__()  # Explicit copy 🚧
```

**Why move-by-default?**
- **Memory safety**: Prevents accidental double-frees
- **Performance**: No hidden expensive copies
- **Explicit ownership**: Clear who owns the resource

**When to implement `__copy__`:**
- Cheap copy (small data)
- Value semantics needed (e.g., Point, Color)
- Sharing state is safe (no unique resources)

**Inspiration**: Rust's Copy trait, C++ move semantics

---

## 9. Dunder Methods ✅ **PUB ENFORCEMENT IMPLEMENTED**

### Reserved Special Methods

**All dunders MUST be `pub`** ✅ (compiler enforces with error DCL0001):

```desi
class Account:
    pub def __new__(id: int) -> Account: ...  # ✅ Required
    pub def __repr__(self) -> str: ...        # ✅ Required
    pub def __eq__(self, other: Account) -> bool: ...  # ✅ Required
    pub def __hash__(self) -> u64: ...        # 🚧 Planned
    pub def __close__(self) -> none: ...      # 🚧 Planned
    pub def __copy__(self) -> Account: ...    # 🚧 Planned
```

**Recognized dunders:**
- `__new__(args...) -> Class` ✅ **IMPLEMENTED** - Constructor
- `__repr__(self) -> str` ✅ **IMPLEMENTED** - String representation  
- `__eq__(self, other: Self) -> bool` 🚧 **PLANNED** - Equality
- `__hash__(self) -> u64` 🚧 **PLANNED** - Hashing (required with `__eq__` for dict/set)
- `__close__(self) -> none` 🚧 **PLANNED** - RAII cleanup
- `__copy__(self) -> Self` 🚧 **PLANNED** - Deep copy
- (Future: `__iter__`, `__next__`, `__lt__`, etc.)

**Overloading:**
- `__new__`: YES (multiple constructors)
- Other dunders: NO (fixed protocol)

**Why no overloading for other dunders?**
- **Simpler semantics**: Each dunder has one role
- **Predictable**: Users know what to expect
- **No ambiguity**: Clear protocol for each operation

---

## 10. Visibility Model

### Public by Default (Top-level), Private by Default (Nested)

```desi
class Outer:       # Public (exported from module)
    pub x: int     # Public field
    y: int         # Private field (same-file only)
    
    class Inner:   # Private nested class
        pass
    
    pub class Info:  # Public nested class
        pass
```

**Cross-file access:**
- Requires `pub` on fields/methods
- Top-level classes always exported

**Same-file access:**
- All members visible (even private)

**Why top-level public?**
- Classes are module's public API
- Nested classes are implementation details

**Inspiration**: Rust's pub(crate), Java's package-private

---

## 11. Method Overloading

### Signature-based Overloading

**Allowed for regular methods and `__new__`:**

```desi
class Point:
    pub def __new__() -> Point:
        Point{ x: 0, y: 0 }
    
    pub def __new__(x: int, y: int) -> Point:
        Point{ x: x, y: y }
    
    pub def distance(self, other: Point) -> float:
        ...
    
    pub def distance(self) -> float:  # Distance from origin
        ...
```

**Overload resolution:**
1. Count arguments (arity)
2. Match parameter types
3. Pick most specific match

**Mangling:**
```
Point()           → Point
Point(int, int)   → Point$2
distance(Point)   → Point_distance$1
distance()        → Point_distance
```

---

---

## 2. Decorators & Metaprogramming

Desi supports a set of built-in decorators to modify method behavior. These are designed to be zero-overhead or low-overhead abstractions.

### @staticmethod
**Design**: Defines a method that does not receive an implicit `self` parameter.
- **Syntax**: Called as `ClassName.method()`
- **Implementation**: Lowered to a regular function with no `self` parameter.
- **Performance**: Zero overhead (identical to a regular function call).
- **Use Case**: Utility functions related to the class but not requiring instance state.

### @classmethod
**Design**: Defines a method that operates on the class rather than an instance.
- **Syntax**: Called as `ClassName.method()`.
- **Parameter Policy**: Unlike Python, Desi does **not** inject an implicit `cls` parameter.
  - **Reasoning**: Desi classes are static types without runtime class objects (meta-classes). There is no "class state" to pass.
  - **Usage**: Use the class name directly within the method body (e.g., `Counter()`).
- **Future Proofing**: If class-level state (static fields) is added in the future, we can introduce an implicit `cls` parameter then without breaking existing code.
- **Performance**: Zero overhead (identical to a regular function call).

### @property
**Design**: Defines a getter method that is accessed like a field.
- **Syntax**: Defined as `def prop(self) -> T`, accessed as `obj.prop` (no parentheses).
- **Implementation**: The compiler rewrites field access `obj.prop` into a method call `obj.prop()`.
- **Performance**: Cost of one function call.
  - **Optimization**: The LLVM backend can inline trivial property getters (e.g., returning a field), making them zero-cost in release builds.
  - **Recommendation**: For expensive computations in hot loops, cache the property value in a local variable.

---

## Performance Summary

| Operation | Cost | Compared to |
|-----------|------|-------------|
| Method call | 1 call | C function |
| Field access | 1 load/store | C struct |
| Constructor (default) | 1 malloc | C malloc |
| Constructor (inline) | 0 (stack) | C++ RAII |
| Async method | State machine | Rust async |
| Generic (mono) | 0 runtime | C++ template |
| RAII cleanup | Scope-exit | C++ destructor |

**Target: Zero overhead compared to hand-written C**

---

## Implementation Status Summary

### ✅ Production Ready (Implemented)
- Static dispatch with name mangling
- C-compatible memory layout
- Single inheritance
- Zero-arg and user-defined `__new__`
- Method lowering with implicit self
- Generic classes with type inference
- Dunder pub enforcement
- Error reporting with diagnostic codes
- Move semantics
- **Decorators**: `@staticmethod`, `@classmethod`, `@property`

### 🚧 Designed (Implementation Pending)
- RAII (`__close__` with `using`)
- Constructor overloading
- Monomorphization for generics (optimization)
- Copy semantics (`__copy__`)
- Visibility enforcement (cross-file pub)
- Async methods (state machines)
- Additional dunders (`__eq__`, `__hash__`, etc.)

### 🔮 Future Considerations

**Not yet implemented (but designed for):**
1. **Trait objects**: Virtual dispatch when needed
2. **Abstract methods**: `@abstract` for inheritance
4. **Reflection**: Limited runtime type information
5. **Custom allocators**: Per-class `__allocate__` hook

**Explicitly NOT supported:**
1. **Multiple inheritance**: Use composition + traits
2. **Operator overloading (general)**: Only specific dunders
3. **Implicit conversions**: Must be explicit
4. **GC/reference counting**: Use RAII instead

