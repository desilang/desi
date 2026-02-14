# Math Module Implementation

This document covers the internal implementation details of the `math` module for contributors.

## Architecture

```
compiler/lib/math/__mod.desi  ← Desi public API (52 functions)
         ↓ (extern "C" calls)
compiler/runtime/math.c       ← C runtime (40+ functions)
         ↓ (links with)
libc (math.h)                 ← System math library (-lm)
```

## Design Rules

The math module follows the project's [stdlib design rules](file:///Users/desiprogrammer/.gemini/antigravity/brain/c5a65b54-83f2-4552-bb8f-aaec8ae18b86/stdlib_roadmap.md):

1. **Behavior → Python** (e.g., `round()` uses banker's rounding, not C's half-away-from-zero)
2. **Safety → Rust** (all C calls wrapped in `unsafe` blocks via safe public wrappers)
3. **Performance → C** only when semantically identical to Python

## C Runtime Functions

All C functions use `__math_` prefix to avoid collisions with libc symbols.

| Category | C Functions |
|----------|-----------|
| Trig | `__math_sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2`, `sinh`, `cosh`, `tanh` |
| Power | `__math_sqrt`, `cbrt`, `pow`, `exp`, `log`, `log2`, `log10`, `hypot` |
| Rounding | `__math_floor`, `ceil`, `round` (banker's), `round_away`, `round_to`, `trunc`, `fmod` |
| Utility | `__math_abs`, `abs_int`, `fmin`, `fmax`, `clamp`, `clamp_int`, `sign` |
| Float | `__math_is_nan`, `is_inf`, `is_finite` (in `builtins.c`) |
| Random | `__math_random`, `randint`, `seed` |
| Convert | `__math_radians`, `degrees` |
| Unique | `__math_lerp`, `inverse_lerp`, `map_range`, `approx_eq`, `is_close`, `smoothstep`, `step`, `wrap`, `wrap_int`, `gcd`, `lcm`, `factorial`, `fib` |

## Key Design Decisions

### Banker's Rounding

`__math_round()` implements IEEE 754 banker's rounding (half to even), matching Python:

```c
double __math_round(double x) {
    double r = round(x);
    double diff = x - floor(x);
    if (fabs(diff - 0.5) < 1e-15) {
        double down = floor(x);
        double up = ceil(x);
        if (fmod(fabs(down), 2.0) < 1e-15) return down;
        return up;
    }
    return r;
}
```

`round_away()` provides C's original behavior as an opt-in.

### round_to(x, places)

Uses multiply → banker's round → divide approach:

```c
double __math_round_to(double x, int places) {
    double multiplier = pow(10.0, (double)places);
    return __math_round(x * multiplier) / multiplier;
}
```

Supports negative places like Python: `round_to(1234.5, -2)` → `1200.0`.

### Float Display

Float-to-string conversion in `string.c` uses `%.15g` with:
- Trailing zero trimming (`3.140000` → `3.14`)
- `.0` preservation for whole numbers (`42.0` not `42`)
- Matches Python's `str(float)` behavior

## Symbol Mangling

Many Desi function names (`sin`, `cos`, `log2`, `round`, etc.) collide with libc symbols. The compiler's `mangleDesiName()` in [hir_lower.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/hir_lower.go) prefixes these with `__desi$` in LLVM IR.

13 math-specific names added to the mangle list: `log`, `log2`, `log10`, `exp`, `cbrt`, `hypot`, `fmod`, `fmin`, `fmax`, `round`, `trunc`, `ceil`, `floor`.

## Adding New Functions

1. Add C function in `compiler/runtime/math.c` with `__math_` prefix
2. Add private `@extern("C")` binding in `compiler/lib/math/__mod.desi`
3. Add `pub def` wrapper with `unsafe:` block and documentation comment
4. Check if the function name needs mangling (add to `mathNames` in `hir_lower.go` if it collides with libc)
5. `make runtime && go build -o bin/desic ./compiler/cmd/desic`
6. Add tests to `examples/342_math_module.desi`
7. Run `./test_examples.sh` — all tests must pass

## Testing

```bash
./test_examples.sh 342,342    # Math module only
./test_examples.sh             # Full suite (382 tests)
```

## Related Files

- [math.c](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/runtime/math.c) — C runtime
- [__mod.desi](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/lib/math/__mod.desi) — Desi bindings
- [hir_lower.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/hir_lower.go) — Symbol mangling
- [string.c](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/runtime/string.c) — `float_to_str()` formatting
- [lower_expr.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/lower_expr.go) — F-string float handling
