# Edge Cases & Missing Features: Enum + Match + Struct

> **2026-07 update:** items 1–3 below were fixed by hybrid memory
> management phase 2 — enum instances (wrapper + payload box) and struct
> instances are freed at scope exit with recursive heap-field cleanup.
> See `docs/contributing/runtime/memory-management.md` for the design.
> String payloads themselves are still not freed (str drops are phase 3).

## Critical Memory Management Issues

### 1. Enum Payload Memory Leaks
**Status: FIXED (hybrid MM phase 2)** — enum locals are dropped at scope
exit; the payload box and wrapper are freed, heap-typed payload fields
recursively.
```desi
enum Result:
  Ok: int
  Err: str

def test():
  let r = Result.Ok(42)  # freed automatically at scope exit
```

### 2. Struct Field Memory Leaks
**Status: PARTIALLY FIXED** — struct instances are dropped at scope exit
and heap-typed fields (lists, dicts, nested structs/enums) are freed
recursively. `str` fields are not freed yet (strings may alias literals;
str ownership is MM phase 3).
```desi
struct Container:
  data: str

def test():
  let c = Container(data: "hello")
  # Container is freed at scope exit; the str is not (phase 3)
```

### 3. Nested Allocations
**Status: FIXED for enum/struct nesting** — recursive drops walk
heap-typed payloads. `str` payloads remain phase 3.
```desi
enum Nested:
  Inner: str

let n = Nested.Inner("test")  # enum + payload box freed; str pending
```

## Match Expression Edge Cases

### 4. N-Arm Matches (N > 2)
**Status: NOT IMPLEMENTED**
```desi
enum Color:
  Red: none
  Green: none  
  Blue: none
  Custom: int  # RGB value

match color:
  Color.Red(): print("Red")
  Color.Green(): print("Green")
  Color.Blue(): print("Blue")
  Color.Custom(rgb): print("Custom")  # 4 arms!
```

### 5. Pattern Variable Bindings
**Status: NOT IMPLEMENTED**
```desi
enum Option:
  Some: int
  None: none

match opt:
  Option.Some(value): print(value)  # Extract 'value'
  Option.None(): print("nothing")
```

### 6. Nested Pattern Matching
**Status: NOT IMPLEMENTED**
```desi
enum Result:
  Ok: Option  # Nested enum!
  Err: str

match result:
  Result.Ok(Option.Some(x)): print(x)  # Nested!
  Result.Ok(Option.None()): print("ok but empty")
  Result.Err(msg): print(msg)
```

### 7. Guard Expressions
**Status: NOT IMPLEMENTED**
```desi
match x:
  Option.Some(n) and n > 0: print("positive")
  Option.Some(n) and n < 0: print("negative")
  Option.Some(0): print("zero")
  _: print("none")
```

### 8. Literal Patterns
**Status: NOT IMPLEMENTED**
```desi
match status_code:
  200: print("OK")
  404: print("Not Found")
  500: print("Server Error")
  _: print("Other")
```

### 9. Exhaustiveness Checking
**Status: NOT IMPLEMENTED**
```desi
enum Bool:
  True: none
  False: none

# Should warn: missing False case!
match b:
  Bool.True(): print("yes")
  # Missing Bool.False() - NOT CAUGHT
```

### 10. Multiple Scrutinee Values (Tuples)
**Status: NOT IMPLEMENTED**
```desi
match (x, y):
  (0, 0): print("origin")
  (0, _): print("on y-axis")
  (_, 0): print("on x-axis")
  _: print("elsewhere")
```

### 11. Or-Patterns
**Status: NOT IMPLEMENTED**
```desi
match color:
  Color.Red() | Color.Green() | Color.Blue(): print("primary")
  _: print("other")
```

### 12. Range Patterns
**Status: NOT IMPLEMENTED**
```desi
match age:
  0..13: print("child")
  13..18: print("teen")
  18..: print("adult")
```

## Enum Edge Cases

### 13. Zero-Variant Enums
**Status: UNKNOWN**
```desi
enum Never:
  # Empty - uninhabited type
  
# Should this compile?
```

### 14. Enums with Generic Payloads
**Status: NOT IMPLEMENTED (no generics yet)**
```desi
enum Option[T]:
  Some: T
  None: none
```

### 15. Recursive Enums
**Status: UNKNOWN - Probably broken**
```desi
enum List:
  Cons: (int, List)  # Self-referential!
  Nil: none
  
# This requires indirection (pointer)
```

### 16. Large Payload Enums
**Status: UNKNOWN - Performance concern**
```desi
struct BigData:
  a: int
  b: int
  c: int
  # ... 100 fields
  
enum Container:
  Small: int
  Large: BigData  # Big payload!
  
# Current: Always malloc() payload
# Better: Inline small payloads, box large ones
```

### 17. Enum Discriminant Values
**Status: NOT IMPLEMENTED**
```desi
# Rust allows:
# enum Status {
#   Ok = 0,
#   Err = -1
# }

enum Status:
  Ok: none = 0      # Custom discriminant
  Err: none = -1
```

### 18. Enum Methods
**Status: NOT IMPLEMENTED**
```desi
enum Option:
  Some: int
  None: none
  
impl Option:
  def is_some(self) -> bool:
    match self:
      Option.Some(_): true
      Option.None(): false
```

## Struct Edge Cases

### 19. Struct with Enum Fields
**Status: Should work but untested**
```desi
struct Config:
  mode: Status
  value: int
```

### 20. Struct Methods
**Status: NOT IMPLEMENTED**
```desi
struct Point:
  x: int
  y: int
  
impl Point:
  def distance(self) -> float:
    sqrt(self.x * self.x + self.y * self.y)
```

### 21. Struct Copy vs Move
**Status: NOT IMPLEMENTED**
```desi
struct Resource:
  handle: int
  
let r1 = Resource(handle: 42)
let r2 = r1  # Copy or move? Should r1 be invalidated?
```

### 22. Struct with Pointer Fields
**Status: LEAKS**
```desi
struct Container:
  data: ptr  # Raw pointer to heap data
  
# No cleanup! Memory leak when struct is dropped
```

## Type System Edge Cases

### 23. Unit Type Enums
**Status: Works**
```desi
enum Signal:
  Start: none
  Stop: none
  Pause: none
```

### 24. Single-Variant Enums (Newtype Pattern)
**Status: Should work**
```desi
enum Age:
  Value: int  # Wrapper around int
```

### 25. Enum Equality
**Status: NOT IMPLEMENTED**
```desi
let a = Status.Ok()
let b = Status.Ok()
if a == b:  # Should this work? Need operator overload
  print("equal")
```

## Concurrency & Safety (Future)

### 26. Thread Safety
**Status: NOT CONSIDERED**
```desi
# If Desi adds threads:
# - Are enums thread-safe?
# - Need atomic tag reads?
# - Data races on payload?
```

### 27. Null Pointer Safety
**Status: UNSAFE**
```desi
enum Option:
  Some: int
  None: none
  
# Current: payload ptr can be NULL for unit variants
# But no checks! Dereferencing = segfault
```

## Performance Edge Cases

### 28. Match Optimization
**Status: Basic If-chain, not optimized**
```desi
# For dense discriminants, LLVM switch is faster
match n:
  0: print("0")
  1: print("1")
  2: print("2")
  # ... 100 cases
  
# Should emit: switch statement
# Currently emits: 100 if-else chains
```

### 29. Inline Small Enums
**Status: NOT IMPLEMENTED**
```desi
# Small enums could be passed by value
enum Small:
  A: none
  B: bool
  
# Current: Always use pointer
# Better: Pass struct { i32, bool } by value
```

### 30. Pattern Compilation Order
**Status: NOT IMPLEMENTED**
```desi
# Smart compilers reorder patterns for efficiency
match (x, y):
  (0, _): ...     # Check x==0 first (fastest)
  (_, 0): ...     # Then y==0
  (1, 1): ...     # Then both
```

## Testing Priority

### High Priority (Blockers)
1. ✅ Memory leaks in enum payloads
2. ✅ N-arm match support
3. ✅ Pattern variable bindings
4. ✅ Exhaustiveness checking

### Medium Priority
5. Struct/enum memory cleanup
6. Guard expressions
7. Literal patterns
8. Nested patterns

### Low Priority (Future)
9. Or-patterns, range patterns
10. Generic enums
11. Enum methods
12. Custom discriminants

## Recommended Next Steps

1. **URGENT: Fix memory leaks**
   - Add destructor support
   - Generate `free()` calls for enum payloads
   - Implement drop semantics

2. **Complete match implementation**
   - Support N arms
   - Pattern variable bindings
   - Exhaustiveness warnings

3. **Add comprehensive tests**
   - Create test suite for all edge cases
   - Memory leak detection (valgrind)
   - Fuzzing for pattern matching

4. **Documentation**
   - Memory model documentation
   - Ownership rules (if any)
   - Pattern syntax guide
