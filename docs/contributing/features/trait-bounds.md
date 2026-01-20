# Trait Bounds for Generics

This document describes the trait bounds system for Desi's generics.

## Syntax

```desi
# Single bound
struct Container<T: Display>:
    value: T

# Multiple bounds
def sum<T: Numeric + Ord>(items: list[T]) -> T:
    ...
```

## Built-in Traits

| Trait | Types that implement |
|-------|---------------------|
| `Numeric` | `int`, `float`, `i8`-`i64`, `u8`-`u64`, `f32`, `f64` |
| `Display` | All primitives, auto-generated for structs/classes/enums |
| `Eq`, `PartialEq` | All primitives, structs, classes, enums |
| `Ord`, `PartialOrd` | Numeric types, `str` |
| `Copy` | All primitives |
| `Send` | All types except guards (`MutexGuard`, `ReadGuard`, `WriteGuard`) |
| `Sync` | All types except guards |

## Error Messages

When a bound is not satisfied, diagnostic **DSY0010** is emitted:

```
error[DSY0010] type: type does not satisfy trait bound
  let c: Container<MyClass> = ...
         ^~~~~~~~~~~~~~~~~~~
  = help: The type argument does not implement the required trait.
```

## Key Files

| File | Purpose |
|------|---------|
| [`ast/nodes.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/ast/nodes.go) | `TypeParamNode` struct with `Bounds` |
| [`check/check_trait.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/check/check_trait.go) | `implementsTrait()`, `validateGenericBounds()` |
| [`check/typename_resolver.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/check/typename_resolver.go) | Bound checking at `Generic` creation |
| [`parse/decl.go`](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/parse/decl.go) | Parsing `<T: Trait>` syntax |
