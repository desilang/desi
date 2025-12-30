# Global Constants - Internals & Implementation Guide

> **For Contributors**: This document covers the global constants implementation, design decisions, and known limitations.

---

## Table of Contents

1. [Design Philosophy](#design-philosophy)
2. [Implementation Overview](#implementation-overview)
3. [Type Inference](#type-inference)
4. [LLVM Emission](#llvm-emission)
5. [Known Limitations](#known-limitations)
6. [Implementation Files](#implementation-files)

---

## Design Philosophy

### Why Global Constants?

Desi supports top-level `let` declarations as global constants, similar to Python's module-level variables but with:

- **Immutability**: Global constants cannot use `mut`
- **Naming Convention**: `UPPER_CASE` naming is enforced via warnings
- **Visibility Control**: Only `pub let` globals can be imported by other modules

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| Immutable only | Prevents shared mutable state bugs |
| UPPER_CASE warning | Clear visual distinction from local variables |
| `pub` required for export | Explicit visibility control |

---

## Implementation Overview

### Compilation Pipeline

```
Source → Parser → Type Checker → HIR Lowering → LLVM Emission
           ↓            ↓               ↓              ↓
        __top__     injectGlobals   module_lower   DefineGlobal
```

### Key Components

1. **Parser** (`parser.go`): Wraps top-level `let` in synthetic `__top__` function
2. **Type Checker** (`check/imports.go`): `injectGlobals()` adds globals to scope
3. **HIR Lowering** (`lower/module_lower.go`): Creates `hir.Global` definitions
4. **LLVM Emission** (`backend/llvm/module.go`): Emits LLVM global definitions

---

## Type Inference

### Supported Literal Types

```go
// From module_lower.go
switch v := ls.Value.(type) {
case *ast.IntLit:    typ = "i64"
case *ast.FloatLit:  typ = "double"
case *ast.BoolLit:   typ = "i1"
case *ast.StrLit:    typ = "ptr" // Uses ensureCStringGlobal
case *ast.UnaryExpr: // Handles -100, -3.14
}
```

### Explicit Type Annotations

When annotations are present (e.g., `let X: u8 = 255`), `llvmTypeFromAST()` maps:

```go
i8, u8   → "i8"
i16, u16 → "i16"
i32, u32 → "i32"
i64, u64 → "i64"
f32      → "float"
f64      → "double"
```

---

## LLVM Emission

### String Constants

String globals use `ensureCStringGlobal` to create backing storage:

```llvm
@.str.0 = private unnamed_addr constant [14 x i8] c"Hello Globals\00"
@STRING_VAL = global ptr getelementptr inbounds ([14 x i8], [14 x i8]* @.str.0, i64 0, i64 0)
```

### Numeric Constants

```llvm
@INT_VAL = global i64 42, align 4
@FLOAT_VAL = global double 3.14159, align 4
@BOOL_TRUE = global i1 1, align 4
```

---

## Known Limitations

### 1. Non-Pub Globals in F-Strings

**Issue**: Non-`pub` globals show `<?>` in f-strings due to missing type info.

**Root Cause**: Type checker populates `info.Types` during identifier resolution, but non-pub globals may not be fully registered.

**Workaround**: Use `pub let` for globals that need f-string interpolation.

### 2. Struct/Enum/Class Globals

**Not Supported**: `let ORIGIN = Point { x: 0, y: 0 }`

**Reason**: Requires compile-time evaluation of struct literals. Planned for future.

### 3. Negative Number Edge Cases

**Partially Supported**: `-100` works via `UnaryExpr` handling, but deeply nested expressions may fail.

---

## Implementation Files

| File | Purpose |
|------|---------|
| `check/imports.go` | `injectGlobals()` - type inference and scope binding |
| `lower/module_lower.go` | HIR global definitions and value extraction |
| `backend/llvm/module.go` | `DefineGlobal()` and `writeGlobals()` |
| `diag/codes.json` | `DW0008` (naming), `DTE0051` (mutability) |

---

## Test Files

- `examples/230_global_consts.desi` - Basic primitives
- `examples/230_b_global_types.desi` - Type coverage
- `examples/230_c_global_privacy.desi` - Import failure test
- `examples/230_c_global_visibility_helper.desi` - Pub global helper
