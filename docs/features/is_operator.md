# Is/Is Not Operator Implementation

This document describes the implementation of the `is` and `is not` operators for contributors.

## Overview

The `is` operator provides:
1. **Identity comparison**: `a is b` compares values/pointers
2. **Pattern matching**: `opt is Some(val)` checks variant and extracts payload

## Architecture

```
Source: x is Some(val)
    ↓
Lexer: KW_is token
    ↓
Parser: IsExpr AST node
    ↓
Type Checker: Creates IsBindings for pattern
    ↓
Lowering: Tag check + payload extraction
    ↓
LLVM IR: icmp + GEP + load
```

## Key Files

| File | Purpose |
|------|---------|
| `token/token.go` | `KW_is` token definition |
| `ast/nodes.go` | `IsExpr` struct |
| `parse/expr.go` | `parseRel()` - parses `is`/`is not` |
| `check/expr.go` | Type checking, creates `IsBindings` |
| `check/stmt.go` | Pushes bindings into scope for if-then |
| `check/info.go` | `IsBindings` map definition |
| `types/option_result.go` | `IsOption`, `IsResult`, payload type helpers |
| `lower/lower_expr.go` | Tag comparison lowering |
| `lower/lower_stmt.go` | Payload extraction in IfStmt |

## Token: KW_is

Added to `token/token.go`:
```go
KW_is // 'is' operator for pattern matching
```

Registered in `keywords.go`:
```go
"is": KW_is,
```

## AST: IsExpr

```go
type IsExpr struct {
    X       Expr      // Left-hand side value
    Pattern Expr      // Pattern (Ident, CallExpr, NoneLit)
    Negated bool      // true for 'is not'
    Span    diag.Span
}
```

## Parsing

In `parseRel()`, after `KW_in` case:

```go
case token.KW_is:
    opStart := p.cur
    p.next()
    
    // Check for 'is not'
    negated := false
    if p.cur.Tok == token.KW_not {
        negated = true
        p.next()
    }
    
    pattern := p.parseBitOr()
    e = &ast.IsExpr{X: e, Pattern: pattern, Negated: negated, ...}
```

## Type Checking

### Pattern Detection

In `check/expr.go`, the `IsExpr` case detects patterns:

```go
case *ast.IsExpr:
    lhsType := c.typ(x.X)
    
    // Check for Some(val), Ok(v), Err(e) patterns
    if call, ok := x.Pattern.(*ast.CallExpr); ok && !x.Negated {
        if callee, ok := call.Callee.(*ast.Ident); ok {
            variantName := callee.Name
            // Create bindings for pattern variables
        }
    }
```

### Binding Creation

For each pattern argument, create a `MatchBinding`:

```go
binding := MatchBinding{
    Name:       ident.Name,   // e.g., "val"
    Type:       payloadType,  // e.g., int
    FieldIndex: i,
    Node:       ident,
}
c.info.IsBindings[x] = bindings
```

### Scope Push

In `check/stmt.go`, `IfStmt` case:

```go
if isExpr, ok := st.Cond.(*ast.IsExpr); ok && !isExpr.Negated {
    if bindings := c.info.IsBindings[isExpr]; len(bindings) > 0 {
        c.scope = NewScope(c.scope)
        for _, b := range bindings {
            c.scope.Define(&Symbol{Name: b.Name, Type: b.Type, ...})
        }
        c.checkBlock(st.Then)
        c.scope = c.scope.parent
    }
}
```

## Lowering

### Tag Comparison

In `lower_expr.go`, `IsExpr` case:

```go
// None/Nothing check for Option
if _, ok := x.Pattern.(*ast.NoneLit); ok && types.IsOption(lhsType) {
    // tag == 1 for Nothing
    ls.b.Emit(&hir.GetElementPtr{...offset 0...})
    ls.b.Emit(&hir.Load{Type: "i32"...})
    ls.b.Emit(&hir.BinaryOp{Op: "==", RHS: ConstInt{"1"}...})
}

// Some/Ok/Err patterns
if call, ok := x.Pattern.(*ast.CallExpr); ok {
    switch calleeName {
    case "Some": // tag == 0
    case "Ok":   // tag == 0
    case "Err":  // tag == 1
    }
}
```

### Payload Extraction

In `lower_stmt.go`, `IfStmt` case:

```go
if isExpr, ok := s.Cond.(*ast.IsExpr); ok && !isExpr.Negated {
    if bindings := ls.info.IsBindings[isExpr]; len(bindings) > 0 {
        lhsVal := ls.lowerExpr(isExpr.X)
        
        // GEP to offset 4 (after i32 tag)
        ls.b.Emit(&hir.GetElementPtr{...offset 4...})
        payloadPtr := ls.b.Emit(&hir.Load{Type: "ptr"...})
        
        for _, binding := range bindings {
            val := ls.b.Emit(&hir.Load{Type: lowerType(binding.Type), Src: payloadPtr})
            ls.matchLocals[binding.Name] = val  // Make available in then block
        }
    }
}
```

## Option/Result Layout

```
Offset 0: i32 tag (0 = Some/Ok, 1 = Nothing/Err)
Offset 4: ptr payload (pointer to boxed value)
```

## Type Helpers

`types/option_result.go` provides:

- `IsOption(t T) bool` - Checks for Option type
- `IsResult(t T) bool` - Checks for Result type  
- `OptionSomeType(t T) T` - Returns T from Option<T>
- `ResultOkType(t T) T` - Returns T from Result<T, E>
- `ResultErrType(t T) T` - Returns E from Result<T, E>

**Important**: These functions handle both `*types.Generic` (wrapper) and `*types.Enum` (direct) representations.

## Test Examples

### Basic Is/Is Not

```desi
def main() -> int:
    # Identity comparison
    let a = 10
    let b = 10
    if a is b:
        print("a is b: PASS")
    
    # Negated identity
    let x = 5
    let y = 10
    if x is not y:
        print("x is not y: PASS")
    
    # Option is none
    let opt_none: Option[int] = Option.Nothing()
    if opt_none is none:
        print("opt_none is none: PASS")
    
    # Boolean identity
    let flag = true
    if flag is true:
        print("flag is true: PASS")
    
    return 0
```

### Pattern Matching with Bindings

```desi
def main() -> int:
    # Option Some(val) binding
    let opt_some: Option[int] = Option.Some(42)
    if opt_some is Some(val):
        print("Got Some value:")
        print(val)  # Output: 42
    
    # Result Ok(v) binding
    let res_ok: Result[int, str] = Result.Ok(100)
    if res_ok is Ok(v):
        print("Got Ok value:")
        print(v)  # Output: 100
    
    # Result Err(e) binding
    let res_err: Result[int, str] = Result.Err("error message")
    if res_err is Err(e):
        print("Got Err value:")
        print(e)  # Output: error message
    
    return 0
```

### Wildcard and Negated Patterns

```desi
def main() -> int:
    let opt_some: Option[int] = Option.Some(42)
    let opt_none: Option[int] = Option.Nothing()
    
    # Wildcard pattern - check without binding
    if opt_some is Some(_):
        print("opt_some is Some (wildcard): PASS")
    
    # Negated wildcard pattern
    if opt_none is not Some(_):
        print("opt_none is not Some: PASS")
    
    # Result wildcards
    let res_ok: Result[int, str] = Result.Ok(100)
    if res_ok is Ok(_):
        print("res_ok is Ok (wildcard): PASS")
    
    return 0
```

