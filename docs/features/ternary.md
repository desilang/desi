# Ternary/Conditional Expression

Desi supports Python-style conditional expressions (ternary operator) for inline conditional logic.

## Syntax

```desi
value_if_true if condition else value_if_false
```

## Basic Usage

```desi
let x = 42 if is_valid else 0
let msg = "yes" if flag else "no"
```

## Common Patterns

### Max/Min

```desi
let max_val = a if a > b else b
let min_val = a if a < b else b
```

### Absolute Value

```desi
def abs(n: int) -> int:
    return n if n >= 0 else (0 - n)
```

### Clamp

```desi
def clamp(val: int, min_v: int, max_v: int) -> int:
    return min_v if val < min_v else max_v if val > max_v else val
```

### Default Values

```desi
let name = user_name if user_name != "" else "Anonymous"
```

## Nested (Chained) Ternary

Ternary expressions can be chained for multi-way selection:

```desi
let grade = "A" if score >= 90 else "B" if score >= 80 else "C" if score >= 70 else "F"
```

## Type Requirements

- **Condition** must be `bool`
- **Both branches** must have compatible types

```desi
let x = 42 if true else 0          # ✅ Both int
let y = "a" if true else "b"       # ✅ Both str
# let z = 42 if true else "str"    # ❌ Type mismatch
```

## In Expressions

Ternary works in any expression context:

```desi
# In function calls
print(f"value: {100 if flag else 0}")

# In arithmetic
let doubled = (10 if cond else 5) * 2

# In list literals
let items = [1, 2, 3 if cond else 0]
```

## See Also

- [Match Expressions](match.md) - For more complex pattern matching
- [Control Flow](../basics/control_flow.md) - For if statements
