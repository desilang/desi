# Mutability and Assignment Operators

This document describes Desi's mutability system and assignment operator semantics.

## Assignment Operators

Desi distinguishes between initial binding and mutation:

| Operator | Token | Usage |
|----------|-------|-------|
| `=` | `ASSIGN` | Initial binding in `let` statements |
| `:=` | `DECLARE` | Mutation of existing values |

### Examples

```desi
# Initial binding (=)
let x = 10           # Binds x to 10
let data = [1, 2, 3] # Binds data to list

# Mutation (:=)
x := 20              # Mutates x from 10 to 20
data[0] := 99        # Mutates list element
self.field := value  # Mutates class field
```

## Implementation

### AST Change

**Location:** `compiler/internal/ast/stmt_assign_node.go`

```go
type AssignStmt struct {
    LHS        []Expr
    RHS        []Expr
    IsReassign bool      // true for ':=', false for '='
    Span       diag.Span
}
```

### Parser

**Location:** `compiler/internal/parse/stmt_assign.go`

```go
case token.ASSIGN: // '='
    return &ast.AssignStmt{IsReassign: false, ...}

case token.DECLARE: // ':='
    return &ast.AssignStmt{IsReassign: true, ...}
```

### Type Checker Enforcement

**Location:** `compiler/internal/check/stmt.go` (IndexExpr case)

```go
case *ast.IndexExpr:
    if !st.IsReassign {
        c.add(diagAt("DTE0012", st.Span, 
            "index mutation requires ':=' not '=' (e.g., list[0] := value)"))
    }
```

## Mutability (`let` vs `let mut`)

Variables declared with `let` are immutable; `let mut` enables mutation.

```desi
let data = [1, 2]     # Immutable list
data[0] := 99         # ERROR: cannot assign to element of immutable list

let mut data = [1, 2] # Mutable list
data[0] := 99         # OK
```

### Enforcement

**Location:** `compiler/internal/check/stmt.go` (lines 385-404)

```go
if _, ok := objType.(*types.List); ok {
    if ident, ok := lhs.X.(*ast.Ident); ok {
        sym := c.scope.Lookup(ident.Name)
        if sym != nil && !sym.IsMutable {
            c.add(diagAt("DCL0004", ident.Span, 
                "cannot assign to element of immutable list '"+ident.Name+"'"))
        }
    }
}
```

## Tests

- `examples/206_custom_dunders.desi` - Uses `:=` for all mutations
- `examples/208_immutable_error.desi` - Immutable list error
- `examples/209_mutation_syntax.desi` - Testing `:=` syntax

## Commits

- `8a29f0e` - Mutability enforcement for list/dict
- `2d0b74c` - `:=` enforcement for mutations
