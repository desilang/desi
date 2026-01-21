# Generics in Desi

This document covers Desi's generics system, including generic classes, functions, and the monomorphization implementation.

## Table of Contents
1. [Generic Classes](#generic-classes)
2. [Generic Functions](#generic-functions)
3. [Constructor Type Inference](#constructor-type-inference)
4. [Turbofish Syntax](#turbofish-syntax-t)
5. [Generic Type Aliases](#generic-type-aliases)
6. [Trait Bounds](#trait-bounds)
7. [Implementation Details](#implementation-details)
8. [Limitations](#limitations)

---

## Generic Classes

Generic classes allow you to define type-safe containers and data structures that work with any type.

### Basic Syntax

```desi
class Box<T>:
    pub mut val: T

    pub def __new__(self, v: T):
        self.val = v

    pub def get(self) -> T:
        return self.val

    pub def set(self, v: T):
        self.val = v
```

### Instantiation

Generic classes can be instantiated with explicit type annotations:

```desi
# Explicit type annotation
let b: Box<int> = Box()
b.val = 42

# Or with constructor arguments (type is inferred)
let b = Box(42)  # Inferred as Box<int>
```

### Multiple Type Parameters

Classes can have multiple type parameters:

```desi
class Pair<A, B>:
    pub mut first: A
    pub mut second: B

    pub def get_first(self) -> A:
        return self.first

    pub def get_second(self) -> B:
        return self.second

# Usage
let p: Pair<int, str> = Pair()
p.first = 42
p.second = "hello"
```

### Supported Types

Generic classes work with all Desi types:
- Primitive types: `int`, `float`, `bool`, `str`
- Sized integers: `i32`, `i64`, `u32`, `u64`
- Collections: `list<T>`, `dict<K, V>`, `set<T>`
- Custom classes and structs
- Nested generics: `Box<list<int>>`

### Example: Comprehensive Test

```desi
class Box<T>:
    pub mut val: T
    pub def __new__(self, v: T):
        self.val = v

def main() -> int:
    # Box with different types
    let b_int = Box(42)
    let b_str = Box("hello")
    let b_float = Box(3.14)
    let b_bool = Box(true)
    
    print(b_int.val)    # 42
    print(b_str.val)    # hello
    print(b_float.val)  # 3.14
    if b_bool.val:
        print("true")   # true
    
    0
```

---

## Generic Functions

Generic functions use type erasure with boxing/unboxing at call sites.

### Basic Syntax

```desi
def identity<T>(x: T) -> T:
    return x

# Usage
let a = identity(42)      # Returns int
let b = identity("hello") # Returns str
let c = identity(3.14)    # Returns float
```

### How It Works

Unlike generic classes (which use monomorphization), generic functions use type erasure:
1. Inside the function, all type parameters are represented as `ptr`
2. At call sites, primitive arguments are boxed (allocated + stored)
3. Return values are unboxed if needed

This approach is simpler and works well for most use cases.

---

## Constructor Type Inference

Desi can infer generic type parameters from constructor arguments.

### Without Inference (Explicit)

```desi
let b: Box<int> = Box()  # Must specify Box<int>
b.val = 42
```

### With Inference

```desi
let b = Box(42)  # Inferred as Box<int> from argument type
print(b.val)     # 42
```

### How It Works

When you call a generic class constructor with arguments:
1. The type checker uses `unify()` to match parameter types with argument types
2. Type parameters are inferred from this unification
3. The inferred instantiation is recorded for monomorphization

### Limitations

- Constructor inference requires arguments that can determine all type parameters
- Zero-argument constructors require explicit type annotations

---

## Turbofish Syntax `::<T>`

When the compiler cannot infer a generic type, you can use **turbofish syntax** to explicitly specify type arguments.

### Basic Usage

```desi
def identity<T>(x: T) -> T:
    return x

# Explicit type specification with turbofish
let x = identity::<int>(42)
let s = identity::<str>("hello")
```

### Nested Generic Calls

Turbofish works correctly with nested generic calls:

```desi
let result = identity::<int>(identity::<int>(42))
```

### When to Use

- When type inference fails due to ambiguity
- When you want to be explicit about types for documentation
- In complex generic contexts where inference might choose the wrong type

---

## Generic Type Aliases

Type aliases let you create shorter names for complex types, including generic types.

### Simple Type Alias

```desi
type IntList = list<int>
type StrToInt = dict<str, int>

let nums: IntList = [1, 2, 3]
let ages: StrToInt = {"alice": 30, "bob": 25}
```

### Generic Type Alias

You can create generic type aliases with type parameters:

```desi
type MyList<T> = list<T>
type MyDict<K, V> = dict<K, V>
type Pair<A, B> = (A, B)

let nums: MyList<int> = [1, 2, 3]
let p: Pair<str, int> = ("alice", 30)
```

### Type Alias with Trait Bounds

Constrained type aliases allow you to restrict which types can be used:

```desi
type NumericList<T: Numeric> = list<T>

let nums: NumericList<int> = [1, 2, 3]   # Works: int is Numeric
let floats: NumericList<float> = [1.0]  # Works: float is Numeric
# let strs: NumericList<str> = ["hi"]   # ERROR: str is not Numeric
```

---

## Trait Bounds

Trait bounds restrict which types can be used with a generic.

### Syntax

```desi
def sum<T: Numeric>(a: T, b: T) -> T:
    return a + b

class Container<T: Display>:
    pub mut val: T
```

### Built-in Traits

| Trait | Types | Description |
|-------|-------|-------------|
| `Numeric` | `int`, `float`, `i32`, etc. | Supports arithmetic |
| `Display` | Most types | Can be printed |
| `Eq` | Most types | Supports `==` |
| `Ord` | `int`, `float`, `str` | Supports `<`, `>` |
| `Send` | Thread-safe types | Safe to send across threads |
| `Sync` | Thread-safe types | Safe to share across threads |

### Multiple Bounds

```desi
def compare<T: Eq + Ord>(a: T, b: T) -> bool:
    return a < b
```

---

## Implementation Details

### Monomorphization (Generic Classes)

Generic classes use **monomorphization** - generating specialized versions for each concrete type instantiation.

#### Name Mangling

```
Box<int>      -> Box_int
Box<str>      -> Box_str
Pair<int,str> -> Pair_int_str
```

#### Specialized Functions

For `class Box<T>`:
- Constructor: `Box_int___new__(ptr self, i32 v)`
- Method: `Box_int_get(ptr self) -> i32`

#### Key Files

- `compiler/internal/lower/class_monomorph.go` - Monomorphization logic
- `compiler/internal/check/generics.go` - `unify()` and `substitute()` functions
- `compiler/internal/check/check_inst.go` - Constructor type inference

### Type Erasure (Generic Functions)

Generic functions use **type erasure** with runtime boxing.

#### At Definition

```
def identity<T>(x: T) -> T
           ↓
define ptr @identity(ptr %x)
```

#### At Call Site

```
identity(42)
    ↓
%box = alloca i32
store i32 42, ptr %box
%result = call ptr @identity(ptr %box)
%unboxed = load i32, ptr %result
```

---

## Limitations

### Known Issues

1. **Multi-param Constructor Inference**: May trigger "inconsistent default parameters" error in some cases.

2. **Nested Generics**: `Box<Box<int>>` works but requires careful handling.

### Future Work

- Full monomorphization for generic functions (if needed for performance)
- Better error messages for type inference failures
- ✅ ~~Support for trait/interface bounds on type parameters~~ (Implemented Jan 2025)

---

## Test Coverage

This document contains comprehensive inline examples for all generics features:

- **Generic Classes**: See "Basic Syntax" and "Example: Comprehensive Test" sections
- **Generic Functions**: See "Generic Functions" section
- **Constructor Inference**: See "Constructor Type Inference" section
- **Multi-param Classes**: See "Multiple Type Parameters" section

---

## Summary

| Feature | Approach | Status |
|---------|----------|--------|
| Generic Classes | Monomorphization | ✅ Complete |
| Generic Functions | Type Erasure | ✅ Complete |
| Constructor Inference | Unification | ✅ Complete |
| Generic Structs | Type Erasure | ✅ Complete |

