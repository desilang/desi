# Math Module - Contributor Guide

Implementation details for the `math` module in `compiler/lib/math/__mod.desi`.

## Architecture

The math module provides 52 mathematical functions through:

1. **C Runtime Functions** (`compiler/runtime/math.c`) — Low-level libc wrappers + custom implementations
2. **Desi Wrappers** (`compiler/lib/math/__mod.desi`) — Safe, ergonomic API

### Module Pattern

```desi
# Private extern binding (not visible to users)
@extern("C")
def __math_sin(x: float) -> float

# Public safe wrapper (the API users call)
pub def sin(x: float) -> float:
    unsafe:
        return __math_sin(x)
```

## Unique Functions

These 13 functions are not found in most stdlib math modules and were inspired by GLSL, game engines, Python, and Rust:

| Function | Inspiration | Use Case |
|----------|------------|----------|
| `lerp` | GLSL | Animation, color blending |
| `inverse_lerp` | Unity | Normalizing values |
| `map_range` | Arduino/Processing | Sensor data mapping |
| `approx_eq` | Rust `approx` crate | Float comparison |
| `is_close` | Python `math.isclose` | Default tolerance comparison |
| `smoothstep` | GLSL | Smooth transitions |
| `step` | GLSL | Threshold function |
| `wrap` / `wrap_int` | Game engines | Angle/index wrapping |
| `gcd` / `lcm` | Python | Number theory |
| `factorial` | Python | Combinatorics |
| `fib` | — | Fibonacci sequence |

## Rounding Implementation

### Why Banker's Rounding?

Python, IEEE 754, and scientific computing use banker's rounding (half-to-even) because it avoids systematic bias. C's `round()` uses half-away-from-zero which biases upward.

Desi provides both:
- `math.round(x)` — Banker's rounding (default, matches Python)
- `math.round_away(x)` — C-style (opt-in)
- `math.round_to(x, places)` — Decimal places with banker's rounding

### Float Display Changes

The float formatting change (`%f` → `float_to_str()`) affects `lower_expr.go` in two code paths:

1. **Default path** (line ~148): `{x}` — no format spec, uses `float_to_str()` + `%s`
2. **FormatExpr path** (line ~120): `{x:spec}` — empty spec uses `float_to_str()`, explicit spec (`.2f`) uses printf format

Both paths handle `float_to_str` with `Type: "ptr"` in the HIR Call node.

## Linker Collision Fix

Many math function names (`sin`, `cos`, `log2`, etc.) collide with libc. When the compiler generates LLVM IR, a Desi function `log2(x)` compiles to `define double @log2(double %x)` which gets resolved back to itself, causing infinite recursion.

**Fix**: `mangleDesiName()` in `hir_lower.go` adds these to the mangle list → `__desi$log2` in IR.

13 names added: `log`, `log2`, `log10`, `exp`, `cbrt`, `hypot`, `fmod`, `fmin`, `fmax`, `round`, `trunc`, `ceil`, `floor`.

## Testing

```bash
./test_examples.sh 342,342    # Math module only
```

## Related Files

- [math.c](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/runtime/math.c) — 40+ C functions
- [__mod.desi](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/lib/math/__mod.desi) — 550+ line Desi module
- [hir_lower.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/hir_lower.go) — Symbol mangling
- [string.c:float_to_str](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/runtime/string.c) — Python-style float formatting
- [lower_expr.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/lower_expr.go) — F-string float_to_str integration
