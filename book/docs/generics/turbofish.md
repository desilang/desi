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

---

## Generic Classes

Turbofish works when constructing a generic class too. This matters when the
constructor's arguments don't mention every type parameter — inference reads
them off the arguments, so a `__new__` that takes none leaves nothing to infer
from:

```python
class Stack<T>:
    pub mut items: list<T>

    pub def __new__(self):      # no argument mentions T
        self.items = []

    pub def push(self, item: T):
        self.items.append(item)

def main() -> int:
    let s = Stack::<int>()      # T stated explicitly
    s.push(1)
    return 0
```

Without the turbofish, `Stack()` reports `DTE0113 — cannot infer type
parameter`.

Each instantiation is independent, so one class can be used at several types in
the same program:

```python
let ints = Stack::<int>()
let names = Stack::<str>()
```

Where the arguments do carry the type, inference is enough and turbofish is
just noise:

```python
class Pair<A, B>:
    pub mut left: A
    pub mut right: B

    pub def __new__(self, l: A, r: B):
        self.left = l
        self.right = r

let p = Pair(7, "x")            # A = int, B = str, inferred
let q = Pair::<int, str>(7, "x")  # same thing, spelled out
```

Passing the wrong number of type arguments reports `DTE0114 — wrong number of
type arguments`.
