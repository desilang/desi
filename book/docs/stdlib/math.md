# Math Module

The `math` module provides mathematical functions for working with numbers.

## Quick Start

```desi
import math

let x = 1.0e308 * 10.0  # Creates infinity
if math.is_inf(x):
    print("x is infinity")
```

## Float Special Value Functions

| Function | Returns | Description |
|----------|---------|-------------|
| `math.is_nan(x)` | `bool` | True if x is NaN (Not a Number) |
| `math.is_inf(x)` | `bool` | True if x is ±infinity |
| `math.is_finite(x)` | `bool` | True if x is a normal number |

## Usage Examples

### Checking for Infinity

```desi
import math

let big = 1.0e308 * 10.0
if math.is_inf(big):
    print("Overflow to infinity!")
```

### Checking for Valid Numbers

```desi
import math

def safe_process(x: float):
    if not math.is_finite(x):
        print("Invalid number!")
        return
    # Process x...
```

## Note on NaN

NaN (Not a Number) typically results from:
- `0.0 / 0.0` - but this now panics in Desi
- `sqrt(-1)` - when implemented
- Invalid math operations

Since division by zero now panics, creating NaN requires special operations. A `math.nan` constant may be added in the future.

## See Also

- [Examples: Math Module](../../examples/245_math_module.desi)
- [Division by Zero](../language/division.md)
