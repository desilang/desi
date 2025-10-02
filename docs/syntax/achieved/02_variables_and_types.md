
# 02 · Variables, Mutability, Assignment

Variables are immutable by default. Use `mut` for mutability. `=` initializes; `:=` reassigns.

```desi
def main() -> int:
  let x = 1          # immutable
  let mut y = 0      # mutable
  y := y + x         # reassignment
  0
```

Type annotations are optional for locals (type inference):

```desi
let mut a: int = 1
a += 41
```

Parallel assignment and swapping:

```desi
let mut a, b, c = 1, 2, 3
a, b := b, a
```

