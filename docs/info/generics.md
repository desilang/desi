# Generics in Desi

This document covers Desi's generics system, including generic classes, functions, and the monomorphization implementation.

## Table of Contents
1. [Generic Classes](#generic-classes)
2. [Generic Functions](#generic-functions)
3. [Constructor Type Inference](#constructor-type-inference)
4. [Implementation Details](#implementation-details)
5. [Limitations](#limitations)

---

## Generic Classes

Generic classes allow you to define type-safe containers and data structures that work with any type.

### Basic Syntax

```python
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

```python
# Explicit type annotation
let b: Box<int> = Box()
b.val = 42

# Or with constructor arguments (type is inferred)
let b = Box(42)  # Inferred as Box<int>
```

### Multiple Type Parameters

Classes can have multiple type parameters:

```python
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

```python
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

```python
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

```python
let b: Box<int> = Box()  # Must specify Box<int>
b.val = 42
```

### With Inference

```python
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
- Support for trait/interface bounds on type parameters

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

