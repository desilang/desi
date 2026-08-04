# Safe FFI

Desi provides safe wrappers around C foreign function interface (FFI) calls, keeping `unsafe` code confined to generated wrappers.

## Basic Safe Extern

Use `@extern("C", safe=true, c_name="...")` to create a safe wrapper for a C function:

```desi
@extern("C", safe=true, c_name="abs")
pub def safe_abs(x: int) -> int
```

This generates:
- A hidden raw `@extern("C")` declaration for the actual C function
- A public wrapper that calls it inside an `unsafe` block

## Out Parameters

Some C functions write results through pointer parameters (out params). Use `out=["param"]` to wrap those params with `&`:

```desi
# C: double modf(double x, double *iptr)
@extern("C", safe=true, c_name="modf", out=["iptr"])
pub def safe_modf(x: float, iptr: float) -> float
```

The generated wrapper:
- Passes `out` params as `&param` (address-of) to the raw C function
- The raw extern receives `cptr[T]` for out param types
- The C function's direct return value is returned

## Diagnostics

| Code | Description |
|------|-------------|
| DFI0006 | Out parameter name not found in function signature |

## Examples

- [336_safe_extern.desi](https://github.com/desilang/desi/blob/main/examples/336_safe_extern.desi) — basic safe extern
- [339_safe_extern_bad_out.desi](https://github.com/desilang/desi/blob/main/examples/339_safe_extern_bad_out.desi) — invalid out param (compile error)
- [341_safe_extern_out_param.desi](https://github.com/desilang/desi/blob/main/examples/341_safe_extern_out_param.desi) — out param with `&value`
