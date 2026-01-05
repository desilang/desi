# Iteration in Desi

Desi provides `for` loops for iterating over collections and `while` loops for conditional iteration.

---

## For Loops

### Basic Syntax
```desi
for variable: Type in collection:
    # body
```

### List Iteration
```desi
let items: list[int] = [1, 2, 3, 4, 5]
for x: int in items:
    print(x)
```

### Dict Iteration
```desi
let prices: dict[str, int] = {"apple": 100, "banana": 50}
for key: str, value: int in prices.items():
    print(key)
    print(value)
```

### Range Iteration
```desi
for i: int in range(10):
    print(i)  # 0, 1, 2, ..., 9
```

---

## Mutable Iteration

By default, loop variables are **read-only**. To modify collection elements, use `mut`:

### Mutable List Elements
```desi
let mut items: list[int] = [1, 2, 3]
for mut x: int in items:
    x *= 2
# items is now [2, 4, 6]
```

### Mutable Dict Values
```desi
let mut scores: dict[str, int] = {"alice": 100, "bob": 85}
for key: str, mut value: int in scores.items():
    value += 10
# scores is now {"alice": 110, "bob": 95}
```

> **Note**: Dict keys cannot be mutable. Only values can be modified.

---

## Rules

| Rule | Description |
|------|-------------|
| Read-only by default | Loop variables cannot be assigned without `mut` |
| Source must be mutable | `for mut x in coll` requires `let mut coll` |
| Keys always immutable | Dict keys cannot be marked `mut` |

---

## ✅ DO

```desi
# DO: Use mut when you need to modify elements
let mut numbers: list[int] = [1, 2, 3]
for mut n: int in numbers:
    n = n * n

# DO: Iterate immutably when only reading
let items: list[str] = ["a", "b", "c"]
for item: str in items:
    print(item)

# DO: Modify dict values, not keys
let mut config: dict[str, int] = {"timeout": 30}
for key: str, mut val: int in config.items():
    val *= 2
```

---

## ❌ DON'T

```desi
# DON'T: Try to mutate without mut keyword
let mut items: list[int] = [1, 2, 3]
for x: int in items:
    x = 0  # ❌ Error: 'x' is read-only

# DON'T: Mutate elements of immutable collection
let items: list[int] = [1, 2, 3]
for mut x: int in items:  # ❌ Error: 'items' is immutable
    x = 0

# DON'T: Try to mutate dict keys
let mut d: dict[str, int] = {"a": 1}
for mut k: str, v: int in d.items():  # ❌ Error: keys cannot be mut
    k = "b"
```

---

## While Loops

While loops follow standard mutability rules:

```desi
let mut i: int = 0
while i < 10:
    print(i)
    i += 1  # ✅ Works - i is declared mut

let j: int = 0
while j < 10:
    j += 1  # ❌ Error: 'j' is immutable
```

---

## Examples

All iteration examples are provided inline throughout this document:

- **List Iteration**: See "For Loops" section
- **Dict Iteration**: See "Dict Iteration" section
- **Mutable Iteration**: See "Mutable Iteration" section
