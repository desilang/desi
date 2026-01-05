# Struct Global Constants

This document describes the implementation of struct literals as global constants.

## Overview

Desi supports declaring struct instances at module level:

```desi
struct Point:
    x: int
    y: int

let ORIGIN = Point(x=0, y=0)  # Global constant
```

## Implementation Changes

### 1. Type Inference (check/imports.go)

The `injectGlobals` function was extended to handle `ast.CallExpr` initializers:

```go
case *ast.CallExpr:
    // Look up struct definition to build types.Struct
    if id, ok := v.Callee.(*ast.Ident); ok {
        for _, d := range mod.Decls {
            if sd, ok := d.(*ast.StructDecl); ok && sd.Name.Name == id.Name {
                // Build types.Struct from declaration
                t = &types.Struct{Name: id.Name, Fields: ...}
            }
        }
    }
```

### 2. Global Variable Store (lower/lower_stmt.go)

Assignment statements now handle globals:

```go
// Check if LHS is a global variable
if id, ok := s.LHS.(*ast.Ident); ok {
    if ls.globals[id.Name] {
        // Emit store to global
        ls.b.Emit(&hir.Store{Dst: hir.Global{Name: "@" + id.Name}, Val: rhs})
        return
    }
}
```

### 3. Global Load Type (lower/lower_expr.go)

Global variable loads default to `ptr` type for heap-allocated structs:

```go
loadType := "ptr"  // Default for globals (heap-allocated)
if sym := ls.info.Idents[x]; sym != nil && sym.Type != nil {
    loadType = lowerType(sym.Type)
}
// Fallback: if type is "void", use "ptr" for globals
if loadType == "void" {
    loadType = "ptr"
}
```

### 4. __top__ Initialization (backend/llvm/emit_func.go)

The `main` function now calls `__top__()` to initialize globals:

```go
if isMainFunc && firstBlock {
    wprintf(&m.funcs, "  call void @__desi_runtime_init()\n")
    // Call __top__ to initialize global variables (only if defined)
    if m.definedFunctions["__top__"] {
        wprintf(&m.funcs, "  call i32 @__top__()\n")
    }
}
```

## Initialization Order

1. `main()` starts
2. `__desi_runtime_init()` initializes runtime
3. `__top__()` runs all global `let` statements (if any exist)
4. `main()` body executes

## Module-Level Variables

The `__top__` function contains all module-level `let` statements:

```llvm
define i32 @__top__() {
entry:
  ; malloc struct
  %inst = call ptr @malloc(i32 8)
  ; call constructor
  call void @Point___new__(ptr %inst, i32 0, i32 0)
  ; store to global
  store ptr %inst, ptr @ORIGIN
  ret i32 0
}
```

## Key Files Modified

- `compiler/internal/check/imports.go` - Type inference for CallExpr
- `compiler/internal/lower/lower_stmt.go` - Global store handling
- `compiler/internal/lower/lower_expr.go` - Global load type resolution
- `compiler/internal/backend/llvm/emit_func.go` - Conditional __top__ call

## Test Coverage

- Global struct declarations work with field access
- All 285 existing tests pass
