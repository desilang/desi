# Iteration

Desi provides `for` loops for iterating over collections and `while` loops for conditional iteration.

## For Loops

### Basic List Iteration

```desi
let items: list[int] = [1, 2, 3, 4, 5]
for x: int in items:
    print(x)
```

### Dict Iteration

```desi
let scores: dict[str, int] = {"alice": 100, "bob": 85}
for key: str, value: int in scores.items():
    print(f"{key}: {value}")
```

### Range Iteration

```desi
for i: int in range(5):
    print(i)  # 0, 1, 2, 3, 4

for i: int in range(2, 6):
    print(i)  # 2, 3, 4, 5

for i: int in range(0, 10, 2):
    print(i)  # 0, 2, 4, 6, 8
```

---

## Mutable Iteration

By default, loop variables are read-only. Use `mut` to modify elements:

### Modify List Elements

```desi
let mut items: list[int] = [1, 2, 3]
for mut x: int in items:
    x *= 2
# items is now [2, 4, 6]
```

### Modify Dict Values

```desi
let mut scores: dict[str, int] = {"alice": 100, "bob": 85}
for key: str, mut value: int in scores.items():
    value += 10
# scores is now {"alice": 110, "bob": 95}
```

> **Note**: Dict keys cannot be modified, only values.

---

## Iteration Helpers

### enumerate()

Get both index and value:

```desi
let items: list[str] = ["a", "b", "c"]
for i: int, item: str in enumerate(items):
    print(f"{i}: {item}")
```

### zip()

Iterate over two lists in parallel:

```desi
let names: list[str] = ["alice", "bob"]
let ages: list[int] = [30, 25]
for name: str, age: int in zip(names, ages):
    print(f"{name} is {age}")
```

### reversed()

Iterate in reverse order:

```desi
let items: list[int] = [1, 2, 3]
for x: int in reversed(items):
    print(x)  # 3, 2, 1
```

---

## List Comprehensions

Create lists with inline syntax:

```desi
# Basic comprehension
let squares: list[int] = [x * x for x: int in range(5)]
# [0, 1, 4, 9, 16]

# With filter
let evens: list[int] = [x for x: int in range(10) if x % 2 == 0]
# [0, 2, 4, 6, 8]
```

---

## While Loops

```desi
let mut i: int = 0
while i < 10:
    print(i)
    i += 1
```

---

## Quick Reference

| Pattern | Description |
|---------|-------------|
| `for x in list:` | Iterate over list |
| `for k, v in dict.items():` | Iterate over dict |
| `for i in range(n):` | 0 to n-1 |
| `for mut x in list:` | Modify elements |
| `for i, x in enumerate(list):` | Index + value |
