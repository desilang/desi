# Explicit Generic Instantiation

Desi supports **generic functions and types** that work with any type. In most cases, Desi's type inference figures out the types automatically, but sometimes you need to specify them explicitly.

## The Turbofish Syntax `::<T>`

Desi uses the **turbofish** syntax to specify generic type arguments explicitly:

```python
# Type inference - Desi figures out the type
let x = identity(42)        # identity::<int> inferred

# Turbofish - explicit type specification
let y = identity::<str>("hello")
```

### Why `::<>` instead of `<>`?

You might wonder why we use `::<>` instead of just `<>`. The answer is **clarity**.

In expressions, `<` and `>` are comparison operators:
```python
if x < 10:    # Less than comparison
    pass
```

Using `::` before `<>` makes it unambiguous that we're specifying types, not comparing values:
```python
identity::<int>(42)   # Clearly a type argument
x < y > z             # Clearly a comparison chain
```

This design choice prioritizes **readable, unambiguous code** and follows the same approach used by Rust.

## Examples

### Generic Functions

```python
def identity<T>(x: T) -> T:
    return x

# With type inference (preferred when possible)
let a = identity(42)

# With explicit turbofish (when inference isn't enough)
let b = identity::<str>("hello")
```

### Generic Structs

```python
struct Box<T>:
    value: T

# Explicit type argument
let box = Box::<int>(value=42)
```

### Multiple Type Arguments

```python
struct Pair<A, B>:
    first: A
    second: B

let pair = Pair::<int, str>(first=10, second="hello")
```

## When to Use Turbofish

**Use type inference** (no turbofish) when Desi can figure out the types:
```python
let x = identity(42)  # int inferred from 42
```

**Use turbofish** when:
- The return type can't be inferred from context
- You want to be explicit about the types
- You're calling a function with no arguments that would hint at the type

> **Tip:** Let Desi infer types when possible — it's cleaner. Use turbofish only when needed.
