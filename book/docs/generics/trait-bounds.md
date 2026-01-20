# Trait Bounds

Trait bounds let you constrain generic type parameters to types that implement specific traits.

## Basic Syntax

```desi
# Require T to implement Display
struct Box<T: Display>:
    value: T

# Require T to implement multiple traits  
struct SortableBox<T: Numeric + Ord>:
    value: T
```

## Why Use Trait Bounds?

Without bounds, generic code can't assume anything about the type:

```desi
# This won't work - T might not support arithmetic
def double<T>(x: T) -> T:
    return x + x  # Error: T might not have +
```

With bounds, you tell the compiler what operations are available:

```desi
# This works - Numeric types support arithmetic
def double<T: Numeric>(x: T) -> T:
    return x + x  # OK: Numeric has +
```

## Built-in Traits

| Trait | What it means |
|-------|---------------|
| `Numeric` | Supports `+`, `-`, `*`, `/` operations |
| `Display` | Can be converted to string for printing |
| `Eq` | Supports `==` and `!=` comparison |
| `Ord` | Supports `<`, `>`, `<=`, `>=` comparison |
| `Copy` | Can be copied instead of moved |
| `Send` | Safe to send to another thread |
| `Sync` | Safe to share between threads |

## Error Messages

If you use a type that doesn't satisfy the bounds, you'll get a clear error:

```
error[DSY0010]: type does not satisfy trait bound
  let b: Box<MyClass> = Box(value=obj)
         ^~~~~~~~~~~~
  = help: The type argument does not implement the required trait.
```

## Multiple Bounds

Use `+` to require multiple traits:

```desi
struct Container<T: Display + Eq>:
    item: T
```

This means `T` must implement **both** `Display` and `Eq`.
