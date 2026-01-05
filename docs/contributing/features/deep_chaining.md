# Deep Chaining Implementation

Deep chaining allows method calls to be chained indefinitely, like `obj.method1().method2().method3()`.

## Problem Context

In previous versions, the compiler often struggled with:
1. **Factory Methods:** Methods returning the class type (`-> Node`) were sometimes confused with constructors or treated as returning opaque pointers without proper type information for subsequent calls.
2. **Value vs Pointer:** Ensuring that `method1()` returns an instance/pointer that `method2()` can correctly accept as `self`.

## Implementation

### Lowering (`lower_call.go`)

The key fix involved distinguishing between **Constructor Calls** and **Method/Factory Calls**.

- **Constructor:** `Node()` -> Allocates memory, calls `__init__`, returns pointer.
- **Factory/Chain:** `n.set_next()` -> Calls function, returns the result directly.

The `lowerCall` function now correctly checks `c.info.Types[expr]` to accept `*types.Class` (instance type) as a valid return type for method calls, preserving the type metadata needed for the next link in the chain.

### String Chaining via `StrBox`

For primitive types like `str`, chaining is achieved via wrapper objects (e.g., `StrBox` in `examples/229_p`).
- `str` is immutable.
- `StrBox` holds a mutable `str` or creates new strings.
- Methods return `StrBox` (self) to allow chaining.
