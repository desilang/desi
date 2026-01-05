# `dbg()` Builtin Implementation

## Overview

The `dbg()` builtin is a debugging helper that prints the source location, expression text, and value, then returns the value unchanged. This allows inspecting values without disrupting code structure.

```desi
let x = 42
let result = dbg(x + 10)  # prints: [file.desi:2] x + 10 = 52
# result is now 52
```

## Implementation

### Type Checker (check/info.go, check/expr_call.go)

1. **DbgCallInfo struct** stores metadata for each dbg() call:
```go
type DbgCallInfo struct {
    File     string
    Line     int
    ExprText string
}
```

2. **DbgCalls map** tracks all dbg() calls:
```go
DbgCalls map[*ast.CallExpr]*DbgCallInfo
```

3. **expr_call.go** intercepts dbg() calls:
```go
if id.Name == "dbg" {
    span := call.Args[0].SpanOf()
    exprText := c.getExprSourceText(call.Args[0])
    c.info.DbgCalls[call] = &DbgCallInfo{...}
    // Return same type as argument
    return argType
}
```

4. **getExprSourceText** in check.go generates expression text from AST:
   - Ident → name
   - BinaryExpr → "lhs op rhs"
   - CallExpr → "func(...)"
   - etc.

### Lowering (lower/lower_call.go)

In `lowerCall()`, dbg() is handled specially:

```go
if calleeName == "dbg" {
    if dbgInfo := ls.info.DbgCalls[x]; dbgInfo != nil {
        argVal := ls.lowerExpr(x.Args[0])
        prefix := fmt.Sprintf("[%s:%d] %s = ", ...)
        // print prefix, print value, print newline
        return argVal  // Return original value
    }
}
```

## Files Modified

- `compiler/internal/check/info.go` - DbgCallInfo struct, DbgCalls map
- `compiler/internal/check/check.go` - getExprSourceText helper
- `compiler/internal/check/expr_call.go` - dbg() type checking
- `compiler/internal/lower/lower_call.go` - dbg() lowering
- `examples/248_dbg_builtin.desi` - Test file

## Usage Example

```desi
def main() -> int:
    let x = 42
    dbg(x)                    # [file:3] x = 42
    dbg(x + 10)               # [file:4] x + 10 = 52
    let y = dbg(x * 2)        # [file:5] x * 2 = 84  (y = 84)
    dbg(dbg(10) + 5)          # [file:6] 10 = 10
                              # [file:6] dbg(...) + 5 = 15
    0
```
