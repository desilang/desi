# Global Constants - Internals & Implementation Guide

> **For Contributors**: Technical implementation details of the global constants system.

---

## Table of Contents

1. [Design Philosophy](#design-philosophy)
2. [Implementation Overview](#implementation-overview)
3. [Type Inference](#type-inference)
4. [Visibility Model](#visibility-model)
5. [LLVM Emission](#llvm-emission)
6. [Implementation Files](#implementation-files)

---

## Design Philosophy

### Core Design Decisions

| Decision | Rationale |
|----------|-----------|
| Immutable only | Prevents shared mutable state bugs |
| UPPER_CASE warning | Clear visual distinction from local variables |
| `pub` required for export | Explicit visibility control |
| Underscore prefix allowed | Convention for internal constants |

---

## Implementation Overview

### Compilation Pipeline

```
Source → Parser → Type Checker → HIR Lowering → LLVM Emission
           ↓            ↓               ↓              ↓
        __top__     injectGlobals   module_lower   DefineGlobal
```

### Parser Behavior

The parser wraps top-level `let` statements in synthetic `__top__` functions:
- `pub let X = 1` → Creates a separate `__top__` with `Pub=true` on the LetStmt
- `let Y = 2` → Hoisted into the main `__top__` function

**Important**: Multiple `pub let` statements create multiple `__top__` functions.

### Type Checker (`check/imports.go`)

The `injectGlobals()` function:
1. Finds ALL `__top__` functions (not just the first)
2. Scans for `LetStmt` nodes
3. Infers types from literals
4. Registers symbols in the top scope

```go
// Must iterate ALL __top__ functions
for _, topFn := range topFns {
    for _, st := range topFn.Body.Stmts {
        // ... type inference and scope binding
    }
}
```

---

## Type Inference

### Supported Literal Types

| Literal | Inferred Type | LLVM Type |
|---------|---------------|-----------|
| `42` | `int` | `i64` |
| `-100` | `int` | `i64` |
| `3.14` | `float` | `double` |
| `-2.5` | `float` | `double` |
| `true`/`false` | `bool` | `i1` |
| `"hello"` | `str` | `ptr` |

### Negative Numbers

Negative literals are parsed as `UnaryExpr` with `-` operator:
```go
case *ast.UnaryExpr:
    if v.Op == "-" {
        switch v.X.(type) {
        case *ast.IntLit:  t = types.Int
        case *ast.FloatLit: t = types.Float
        }
    }
```

---

## Visibility Model

| Syntax | Same File | Other Modules |
|--------|-----------|---------------|
| `let X = 1` | ✓ | ✗ |
| `pub let X = 1` | ✓ | ✓ |
| `let _X = 1` | ✓ | ✗ |

**Export Logic** (`resolve/exports.go`):
- Only `pub let` statements are added to `out.Globals`
- Non-pub globals are file-private

---

## LLVM Emission

### String Constants

```llvm
@.str.0 = private unnamed_addr constant [6 x i8] c"hello\00"
@MY_STR = global ptr getelementptr inbounds ([6 x i8], [6 x i8]* @.str.0, i64 0, i64 0)
```

### Numeric Constants

```llvm
@MY_INT = global i64 42, align 4
@MY_FLOAT = global double 3.14, align 4
@MY_BOOL = global i1 1, align 4
```

---

## Implementation Files

| File | Purpose |
|------|---------|
| `parse/parser.go` | Wraps top-level `let` in `__top__` |
| `check/imports.go` | `injectGlobals()` - type inference |
| `lower/module_lower.go` | HIR global definitions |
| `backend/llvm/module.go` | `DefineGlobal()`, `writeGlobals()` |
| `resolve/exports.go` | Export filtering (pub only) |
| `diag/codes.json` | `DW0008`, `DTE0051` |

---

## Test Coverage

| Test File | Coverage |
|-----------|----------|
| `230_global_consts.desi` | Basic primitives |
| `230_b_global_types.desi` | Type variants |
| `230_c_global_privacy.desi` | Import failure |
| `230_d_nonpub_fstring.desi` | Non-pub in f-strings |
| `230_e_all_globals.desi` | Comprehensive: pub, non-pub, underscore |
