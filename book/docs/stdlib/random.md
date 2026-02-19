# Random Module

The `random` module provides random number generation, list sampling, and cryptographically secure randomness.

## Import

```desi
import random
```

## API Reference

### Core

| Function | Returns | Description |
|----------|---------|-------------|
| `seed(n)` | `none` | Seed the PRNG |
| `randint(a, b)` | `int` | Random int in [a, b] inclusive |
| `random()` | `float` | Random float in [0.0, 1.0) |
| `uniform(a, b)` | `float` | Random float in [a, b] |

### Collections

| Function | Returns | Description |
|----------|---------|-------------|
| `choice(items)` | `str` | Random element from list |
| `shuffle(items)` | `list[str]` | Shuffled copy (not in-place) |
| `sample(items, k)` | `list[str]` | k random elements without replacement |

### Secure

| Function | Returns | Description |
|----------|---------|-------------|
| `hex(n)` | `str` | Random hex string of n bytes |
| `crypto_random()` | `int` | Cryptographically secure random int |

## Usage Example

```desi
import random

def main() -> int:
    random.seed(42)
    print(random.randint(1, 100))
    print(random.choice(["red", "green", "blue"]))

    let deck = ["A", "2", "3", "4", "5"]
    let hand = random.sample(deck, 3)
    print(hand)
    0
```

## See Also

- [UUID Module](uuid.md) — Random UUID generation
- [Hash Module](hash.md) — Cryptographic hashing
