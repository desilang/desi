# Variadic Functions in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [Syntax](#syntax)
3. [Type Safety](#type-safety)
4. [Working with Varargs](#working-with-varargs)
5. [Named Arguments with Varargs](#named-arguments-with-varargs)
6. [Common Patterns](#common-patterns)
7. [Performance](#performance)

---

## Quick Start

```desi
def sum(first: int, *rest: int) -> int:
    # rest is a list[int] containing all extra arguments
    let total: int = first
    for n: int in rest:
        total = total + n
    return total

def main():
    sum(1)              # first=1, rest=[]
    sum(1, 2, 3)        # first=1, rest=[2, 3]
    sum(10, 20, 30, 40) # first=10, rest=[20, 30, 40]
```

---

## Syntax

### Basic Varargs

Use `*` before the parameter name to collect remaining arguments:

```desi
def function_name(fixed_params, *vararg: T) -> ReturnType:
    # vararg is list[T]
```

**Rules:**
1. Only ONE vararg parameter allowed
2. Vararg must be the LAST positional parameter
3. Vararg collects zero or more arguments
4. Type annotation is required: `*name: ElementType`

### Examples

```desi
# Collect strings
def log(*messages: str) -> none:
    for msg: str in messages:
        print(msg)

# At least one required
def min(first: int, *rest: int) -> int:
    let result: int = first
    for n: int in rest:
        if n < result:
            result = n
    return result

# Mixed fixed and variadic
def format(template: str, *args: str) -> str:
    # template is fixed, args is variadic
    # ...
```

---

## Type Safety

Varargs are fully type-checked at compile time:

```desi
def sum(*numbers: int) -> int:
    # ...

def main():
    sum(1, 2, 3)        # ✅ OK - all ints
    sum(1, "two", 3)    # ❌ Error: expected int, got str
    sum()               # ✅ OK - empty list
```

The vararg parameter becomes `list[T]` internally:

```desi
def process(*items: str) -> none:
    # items has type list[str]
    let count: int = items.len()
    for item: str in items:
        print(item)
```

---

## Working with Varargs

### Iterating

```desi
def print_all(*items: str) -> none:
    for item: str in items:
        print(item)
```

### Getting Length

```desi
def count(*args: int) -> int:
    return args.len()
```

### Indexing

```desi
def first_or_default(*args: int, default: int = 0) -> int:
    if args.len() > 0:
        return args[0]
    return default
```

---

## Named Arguments with Varargs

Named arguments can follow varargs:

```desi
def print(*args: str, sep: str = " ", end: str = "\n") -> none:
    # args collected positionally
    # sep and end must be passed by name
```

**Usage:**

```desi
print("a", "b", "c")                    # Uses defaults
print("a", "b", sep=", ")               # Custom separator
print("a", "b", sep=", ", end="!\n")    # Custom both
```

---

## Common Patterns

### Printf-style Formatting

```desi
def printf(fmt: str, *args: str) -> none:
    # Format string with placeholder replacement
```

### Builder Methods

```desi
def query(*conditions: str) -> Query:
    let q: Query = Query()
    for cond: str in conditions:
        q.add_condition(cond)
    return q

# Usage
query("age > 18", "status = 'active'", "role = 'admin'")
```

### Aggregation

```desi
def max(first: int, *rest: int) -> int:
    let result: int = first
    for n: int in rest:
        if n > result:
            result = n
    return result
```

---

## Performance

**Zero overhead design:**

- Varargs are collected into a `list[T]` at call site
- No boxing for primitive types beyond normal list storage
- Iteration uses standard list iteration (index-based loop)
- Compiler can optimize small fixed-size calls

**Equivalent to:**

```desi
# This call:
sum(1, 2, 3)

# Is equivalent to:
let __vararg: list[int] = [1, 2, 3]
sum_impl(1, __vararg)
```

---

## Examples

All varargs examples are provided inline throughout this document:

- **Quick Start**: See opening section for basic usage
- **Named Arguments**: See "Named Arguments with Varargs" section
- **Common Patterns**: See "Common Patterns" section

## See Also

- [Keyword Arguments (**kwargs)](kwargs.md) — collect named arguments as a `dict[str, T]`
