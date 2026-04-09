# sys Module Implementation

The `sys` module provides access to system internals: version info, memory layout, recursion limits, and runtime introspection.

## Architecture

```
import sys
    ↓
StdlibImports["sys"] = true  (check/stmt.go)
    ↓
compiler/lib/sys/__mod.desi  →  @extern("C") bindings
    ↓
compiler/runtime/sys.c  →  C functions (__sys_*)
```

## Files

| File | Purpose |
|------|---------|
| `compiler/lib/sys/__mod.desi` | Desi bindings (pub def wrappers around C) |
| `compiler/runtime/sys.c` | C runtime implementation |
| `compiler/internal/check/stmt.go` | Registers sys in StdlibImports |

## C Runtime Functions

| C Function | Desi Binding | Return Type | Description |
|-----------|-------------|-------------|-------------|
| `__sys_version()` | `version()` | `str` | Returns `"0.1.0"` |
| `__sys_maxsize()` | `maxsize()` | `int` | `INT32_MAX` (2147483647) |
| `__sys_byteorder()` | `byteorder()` | `str` | Runtime endianness check |
| `__sys_sizeof_ptr()` | `sizeof_ptr()` | `int` | `sizeof(void*)` |
| `__sys_recursion_limit()` | `recursion_limit()` | `int` | Current max depth |
| `__sys_call_depth()` | `call_depth()` | `int` | Current stack depth |

## Design Decisions

- **No `pub extern stdout/stderr`**: The old sys module exposed `sys.stdout` and `sys.stderr` as `pub extern` variables, but Desi's compiler doesn't support `pub extern` for variables (only functions). These symbols are now handled by `print(..., file=sys.stderr)` syntax and the `io.eprint()` function.
- **Version is hardcoded**: `__sys_version()` returns a compile-time constant. Will be generated from build system in the future.
- **`call_depth()`** reads from the `__desi_call_depth` global that the call enter/exit instrumentation maintains.

## Adding New sys Functions

1. Add C function in `compiler/runtime/sys.c` with `__sys_` prefix
2. Add `@extern("C")` binding in `compiler/lib/sys/__mod.desi`
3. Add `pub def` wrapper in the same file
4. Add test in `examples/451_sys_module.desi`
5. Update book docs in `book/docs/stdlib/sys.md`
