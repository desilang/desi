# Select Implementation

This document explains the implementation of `select` statement in the Desi compiler.

## Syntax

```desi
select:
    case msg = rx.try_recv():
        handle(msg)
    case tx.try_send(value):
        pass
    default:
        print("nothing ready")
```

## AST Node

**File**: `ast/stmt_select_node.go`
```go
type SelectStmt struct {
    Cases   []SelectCase
    Default []Stmt
    Span    diag.Span
}

type SelectCase struct {
    Binding *Ident  // For recv: variable to bind
    Op      Expr    // Channel operation
    Body    []Stmt  // Case body
}
```

## Parser

**File**: `parse/stmt_select.go`
- Recognizes `select:` keyword
- Parses `case binding = expr:` and `default:` branches
- Uses IDENT comparison for 'case'/'default' (not keywords)

## Type Checker

**File**: `check/check_select.go`
- Validates channel operations
- Introduces bindings in case body scope

## Lowering

**File**: `lower/select_lower.go`
- Currently simplified: runs all case operations and bodies sequentially
- TODO: Proper if-else branching based on operation result

## Related Files

- `compiler/internal/ast/stmt_select_node.go`
- `compiler/internal/parse/stmt_select.go`
- `compiler/internal/check/check_select.go`
- `compiler/internal/lower/select_lower.go`
