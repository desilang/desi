# Generics

Generics enable writing code that works with multiple types while maintaining type safety. Desi uses a combination of type erasure and monomorphization.

## Generic Functions

### Basic Syntax

```desi
def function_name<T>(param: T) -> T:
    return param
```

### Single Type Parameter

```desi
def identity<T>(x: T) -> T:
    return x

def main():
    let a: int = identity(42)
    let b: str = identity("hello")
    let c: float = identity(3.14)
    print(a)  # 42
    print(b)  # hello
```

### Multiple Type Parameters

```desi
def swap<A, B>(a: A, b: B) -> tuple[B, A]:
    return (b, a)

def main():
    let (x, y) = swap(42, "hello")
    print(x)  # "hello"
    print(y)  # 42
```

### Type Inference

The compiler infers type parameters from arguments:

```desi
# No need to write: identity<int>(42)
let x = identity(42)      # T inferred as int
let y = identity("hi")    # T inferred as str
```

## Generic Classes

### Basic Syntax

```desi
class ClassName<T>:
    pub field: T
    
    pub def get(self) -> T:
        return self.field
```

### Single Type Parameter

```desi
class Box<T>:
    pub mut val: T
    
    pub def get(self) -> T:
        return self.val
    
    pub def set(self, v: T):
        self.val = v

def main():
    # Box<int>
    let b_int: Box<int> = Box()
    b_int.val = 42
    print(b_int.get())  # 42
    
    # Box<str>
    let b_str: Box<str> = Box()
    b_str.val = "hello"
    print(b_str.get())  # hello
```

### Multiple Type Parameters

```desi
class Pair<A, B>:
    pub mut first: A
    pub mut second: B
    
    pub def get_first(self) -> A:
        return self.first
    
    pub def get_second(self) -> B:
        return self.second
    
    pub def set_both(self, a: A, b: B):
        self.first = a
        self.second = b

def main():
    let pair: Pair<int, str> = Pair()
    pair.first = 42
    pair.second = "answer"
    print(pair.get_first())   # 42
    print(pair.get_second())  # "answer"
```

### Constructor Inference

Type parameters can be inferred from constructor arguments:

```desi
class Box<T>:
    pub mut val: T
    
    pub def __new__(self, v: T):
        self.val = v

def main():
    # Type inferred from constructor argument
    let b1 = Box(42)       # Box<int>
    let b2 = Box("hello")  # Box<str>
    let b3 = Box(3.14)     # Box<float>
    let b4 = Box(true)     # Box<bool>
    
    print(b1.val)  # 42
    print(b2.val)  # hello
```

## Generic Structs

```desi
struct Wrapper<T>:
    value: T

def main():
    let w: Wrapper<int> = Wrapper(value=42)
    print(w.value)
```

## Type Aliases with Generics

### Generic Type Aliases

```desi
# Simple aliases
type IntList = list<int>
type StrList = list<str>

# Generic aliases with type parameters
type Box<T> = Option<T>
type Pair<A, B> = tuple[A, B]
type Triple<X, Y, Z> = tuple[X, Y, Z]

def main():
    let nums: IntList = [1, 2, 3]
    let boxed: Box<int> = Option.Some(42)
    let pair: Pair<str, int> = ("answer", 42)
```

## Built-in Generic Types

### Option\<T\>

```desi
let some: Option<int> = Option.Some(42)

# `none` is a keyword, so it cannot be a variable name
let empty: Option<str> = Option.Nothing
```

### Result\<T, E\>

```desi
let ok: Result<int, str> = Result.Ok(100)
let err: Result<int, str> = Result.Err("failed")
```

### Collections

```desi
let ints: list<int> = [1, 2, 3]
let strs: list<str> = ["a", "b", "c"]
let map: dict<str, int> = {"a": 1, "b": 2}
let tags: set<str> = #{"x", "y", "z"}
```

## Supported Type Arguments

Generic classes and functions work with all types:

| Type | Example |
|------|---------|
| `int` | `Box<int>` |
| `float` | `Box<float>` |
| `bool` | `Box<bool>` |
| `str` | `Box<str>` |
| Sized types | `Box<i64>`, `Box<f32>` |
| Collections | `Box<list<int>>` |
| Custom classes | `Box<Point>` |

```desi
# All supported types
let b_int: Box<int> = Box()
let b_float: Box<float> = Box()
let b_bool: Box<bool> = Box()
let b_str: Box<str> = Box()
let b_i64: Box<i64> = Box()
let b_list: Box<list<int>> = Box()
```

## Implementation Details

### Monomorphization

Desi uses **monomorphization** for generic classes: the compiler generates specialized versions for each type used.

```desi
# When you write:
let b1: Box<int> = Box()
let b2: Box<str> = Box()

# Compiler generates:
# - Box_int class with int-specific methods
# - Box_str class with str-specific methods
```

!!! info "What this means"
    - **Performance**: No runtime overhead - specialized code for each type
    - **Binary size**: Larger binaries with many generic instantiations
    - **Stack traces**: Functions show specialized names like `Box_int_get`

### Type Erasure for Functions

Generic functions use **type erasure** with boxing:

```desi
# identity<T>(x: T) -> T
# All types are boxed/unboxed at call sites
```

This means generic functions compile once but work with all types through pointer indirection.

## Constraints (Future)

!!! info "Not Yet Implemented"
    Type constraints (bounds) like `T: Display` are planned for a future release.

Planned syntax:

```desi
# Future: constrained generics
def print_it<T: Display>(x: T) -> none:
    print(x.to_str())
```

## Nested Generics

```desi
# Nested generic types
let nested: list<Option<int>> = [
    Option.Some(1), 
    Option.Some(2), 
    Option.Nothing
]

let deep: dict<str, list<int>> = {
    "a": [1, 2, 3],
    "b": [4, 5, 6]
}
```

## Best Practices

### ✅ Do

- **Use descriptive type parameter names**: `T` for single, `K/V` for key/value, `A/B` for pairs
- **Let the compiler infer when possible**: `identity(42)` vs `identity<int>(42)`
- **Document what types are expected**: Comments help readers
- **Use generic types for reusable containers**: Box, Wrapper, etc.

### ❌ Don't

- **Don't over-generalize**: Simple functions don't need generics
- **Avoid deeply nested generics**: Hard to read `Box<Option<list<int>>>`
- **Don't assume constraints exist**: All types accepted (for now)

## Common Patterns

### Container Pattern

```desi
class Container<T>:
    _items: list<T>
    
    pub def add(self, item: T):
        self._items.append(item)
    
    pub def get(self, idx: int) -> T:
        return self._items[idx]
    
    pub def len(self) -> int:
        return len(self._items)
```

### Optional Value Pattern

```desi
def first_or_default<T>(items: list<T>, default: T) -> T:
    if len(items) > 0:
        return items[0]
    return default
```

### Transform Pattern

A function type cannot be written in a signature yet, so a callable parameter is
typed `Any` — see [Known Limitations](../reference/known-limitations.md):

```desi
def map_value<T, U>(opt: Option<T>, f: Any) -> Option<U>:
    return match opt:
        Option.Some(v): Option.Some(f(v))
        Option.Nothing: Option.Nothing
```

## Comparison with Other Languages

| Language | Approach | When Resolved |
|----------|----------|---------------|
| **Desi** | Monomorphization (classes) + Erasure (functions) | Compile time |
| Rust | Monomorphization | Compile time |
| Java | Type erasure | Runtime |
| Go | Monomorphization (Go 1.18+) | Compile time |
| Python | Duck typing | Runtime |
| TypeScript | Type erasure | Compile time (types only) |

Desi's hybrid approach balances performance (monomorphization for classes) with simplicity (erasure for functions).

## See Also

- [Types](types.md) - Type system overview
- [Classes](classes.md) - Generic classes
- [Functions](functions.md) - Generic functions
- [Error Handling](error-handling.md) - Option and Result generics
