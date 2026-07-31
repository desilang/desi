# Math Module

The `math` module provides 50+ mathematical functions covering constants, trigonometry, power/log, rounding, utility, float checks, random numbers, interpolation, number theory, and more.

## Import

```desi
import math
```

## Constants

```desi
math.PI()   # 3.14159265358979
math.E()    # 2.71828182845905
math.TAU()  # 6.28318530717959 (2π)
math.INF()  # Positive infinity
math.NAN()  # Not a Number
```

## Trigonometry

### Standard Trig

| Function | Description |
|----------|-------------|
| `sin(x: float) -> float` | Sine |
| `cos(x: float) -> float` | Cosine |
| `tan(x: float) -> float` | Tangent |
| `asin(x: float) -> float` | Arc sine |
| `acos(x: float) -> float` | Arc cosine |
| `atan(x: float) -> float` | Arc tangent |
| `atan2(y: float, x: float) -> float` | Two-argument arc tangent |

### Hyperbolic

| Function | Description |
|----------|-------------|
| `sinh(x: float) -> float` | Hyperbolic sine |
| `cosh(x: float) -> float` | Hyperbolic cosine |
| `tanh(x: float) -> float` | Hyperbolic tangent |

### Example

```desi
import math

let angle = math.atan2(1.0, 1.0)   # 0.785398... (π/4)
let deg = math.degrees(angle)       # 45.0
let rad = math.radians(180.0)       # 3.14159... (π)
```

## Power & Logarithms

| Function | Description |
|----------|-------------|
| `sqrt(x: float) -> float` | Square root |
| `cbrt(x: float) -> float` | Cube root |
| `pow(x: float, y: float) -> float` | x^y |
| `exp(x: float) -> float` | e^x |
| `log(x: float) -> float` | Natural log (ln) |
| `log2(x: float) -> float` | Log base 2 |
| `log10(x: float) -> float` | Log base 10 |
| `hypot(x: float, y: float) -> float` | √(x² + y²) |

### Example

```desi
import math

print(math.sqrt(144.0))    # 12.0
print(math.pow(2.0, 10.0)) # 1024.0
print(math.log2(1024.0))   # 10.0
print(math.hypot(3.0, 4.0)) # 5.0
```

## Rounding

Desi follows **Python's rounding convention** by default (banker's rounding / IEEE 754):

| Function | Description |
|----------|-------------|
| `floor(x: float) -> float` | Round down |
| `ceil(x: float) -> float` | Round up |
| `round(x: float) -> float` | Banker's rounding (half to even) |
| `round_away(x: float) -> float` | C-style (half away from zero) |
| `round_to(x: float, places: int) -> float` | Round to N decimal places |
| `trunc(x: float) -> float` | Truncate toward zero |
| `fmod(x: float, y: float) -> float` | Floating-point modulo |

### Rounding Behavior

| Expression | Result | Notes |
|-----------|--------|-------|
| `math.round(2.5)` | `2.0` | Banker's rounding — rounds to even |
| `math.round(3.5)` | `4.0` | Rounds to even (4) |
| `math.round(-2.5)` | `-2.0` | Same rule, half to even |
| `math.round_away(2.5)` | `3.0` | C-style, half away from zero |
| `math.round_to(3.14159, 2)` | `3.14` | Round to 2 decimal places |
| `math.round_to(1234.5, -2)` | `1200.0` | Negative places (like Python) |

!!! tip "Why Banker's Rounding?"
    Banker's rounding avoids systematic upward bias when rounding `.5` values repeatedly. This matters in financial calculations, scientific computing, and statistics. Use `round_away()` when you need the traditional behavior.

## Utility Functions

| Function | Description |
|----------|-------------|
| `abs(x: float) -> float` | Absolute value (float) |
| `abs_int(x: int) -> int` | Absolute value (int) |
| `fmin(x: float, y: float) -> float` | Minimum of two floats |
| `fmax(x: float, y: float) -> float` | Maximum of two floats |
| `clamp(x: float, lo: float, hi: float) -> float` | Clamp to range |
| `clamp_int(x: int, lo: int, hi: int) -> int` | Clamp (int) |
| `sign(x: float) -> float` | `-1.0`, `0.0`, or `1.0` |

### Example

```desi
import math

print(math.clamp(150.0, 0.0, 100.0))  # 100.0
print(math.clamp(-5.0, 0.0, 100.0))   # 0.0
print(math.sign(-7.0))                 # -1.0
```

## Float Checks

| Function | Description |
|----------|-------------|
| `is_nan(x: float) -> bool` | True if NaN |
| `is_inf(x: float) -> bool` | True if ±infinity |
| `is_finite(x: float) -> bool` | True if normal number |

## Random Numbers

| Function | Description |
|----------|-------------|
| `random() -> float` | Random float in `[0, 1)` |
| `randint(lo: int, hi: int) -> int` | Random int in `[lo, hi]` |
| `seed(n: int) -> none` | Seed the RNG |

### Example

```desi
import math

math.seed(42)
let roll = math.randint(1, 6)     # Dice roll
let val = math.random()           # 0.0 to 1.0
```

## Conversion

| Function | Description |
|----------|-------------|
| `radians(degrees: float) -> float` | Degrees → radians |
| `degrees(radians: float) -> float` | Radians → degrees |

## Interpolation & Animation

These functions are commonly found in game engines and shader languages (GLSL), but rarely in standard libraries.

| Function | Description |
|----------|-------------|
| `lerp(a: float, b: float, t: float) -> float` | Linear interpolation |
| `inverse_lerp(a: float, b: float, x: float) -> float` | Inverse lerp |
| `map_range(x: float, in_lo: float, in_hi: float, out_lo: float, out_hi: float) -> float` | Remap value from one range to another |
| `smoothstep(edge0: float, edge1: float, x: float) -> float` | Smooth transition |
| `step(edge: float, x: float) -> float` | Step function (0 or 1) |

### Example

```desi
import math

# Smooth animation from 0 to 1
let t = math.smoothstep(0.0, 1.0, 0.5)  # 0.5

# Map temperature sensor (0-1023) to Celsius (-40 to 125)
let celsius = math.map_range(512.0, 0.0, 1023.0, -40.0, 125.0)

# Wrap angle to [0, 360)
let angle = math.wrap(370.0, 0.0, 360.0)  # 10.0
```

## Float Comparison

| Function | Description |
|----------|-------------|
| `approx_eq(a: float, b: float, epsilon: float) -> bool` | Almost equal, within an explicit epsilon |
| `is_close(a: float, b: float, rel_tol: float, abs_tol: float) -> float` | Almost equal, with separate relative and absolute tolerances |

### Example: The Classic Float Trap

```desi
import math

# 0.1 + 0.2 == 0.3 is FALSE due to IEEE 754
let naive = (0.1 + 0.2 == 0.3)                    # false!
let correct = math.approx_eq(0.1 + 0.2, 0.3, 0.000000001)  # true
```

## Wrap Functions

| Function | Description |
|----------|-------------|
| `wrap(x: float, lo: float, hi: float) -> float` | Wrap to range |
| `wrap_int(x: int, lo: int, hi: int) -> int` | Wrap (int) |

### Example

```desi
import math

# Wrap angle to [0, 360)
print(math.wrap(370.0, 0.0, 360.0))   # 10.0
print(math.wrap(-30.0, 0.0, 360.0))   # 330.0

# Wrap array index
print(math.wrap_int(25, 0, 24))        # 1
```

## Number Theory

| Function | Description |
|----------|-------------|
| `gcd(a: int, b: int) -> int` | Greatest common divisor |
| `lcm(a: int, b: int) -> int` | Least common multiple |
| `factorial(n: int) -> int` | n! |
| `fib(n: int) -> int` | Fibonacci number |

### Example

```desi
import math

print(math.gcd(12, 8))       # 4
print(math.lcm(4, 6))        # 12
print(math.factorial(10))    # 3628800
print(math.fib(20))          # 6765
```

## Legacy Functions

These exist for backward compatibility with earlier versions of the module:

| Function | Description |
|----------|-------------|
| `add(x: int, y: int) -> int` | Integer addition |
| `sub(x: int, y: int) -> int` | Integer subtraction |

## See Also

- [Division by Zero](../language/division.md) — Related safety behavior
- [Builtins](../language/builtins.md) — Built-in functions like `sum()`, `min()`, `max()`
