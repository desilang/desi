# `__bool__` Dunder Method Implementation

## Overview

The `__bool__` dunder method allows custom classes to define their truthiness behavior when used in boolean contexts like `if` conditions.

## Implementation

### Type Checker (check/stmt.go)

When checking an `if` statement, we detect if the condition is a class type with a `__bool__` dunder:

```go
case *ast.IfStmt:
    condType := c.typ(st.Cond)
    if condType != nil {
        if cls, ok := condType.(*types.Class); ok {
            if _, hasBool := cls.Dunders["__bool__"]; hasBool {
                c.info.BoolConversions[st.Cond] = cls
            }
        }
    }
```

### Info Struct (check/info.go)

A new map tracks expressions needing `__bool__` conversion:

```go
BoolConversions map[ast.Expr]*types.Class
```

### Lowering (lower/lower_stmt.go)

When lowering the if statement, we check if the condition needs `__bool__` conversion:

```go
if cls, needsBool := ls.info.BoolConversions[s.Cond]; needsBool {
    boolResult := ls.b.FreshTemp("bool_result")
    mangledName := fmt.Sprintf("%s___bool__", cls.Name)
    ls.b.Emit(&hir.Call{
        Dst:  boolResult,
        Fn:   mangledName,
        Args: []hir.Value{cond},
        Type: "i1",
    })
    cond = boolResult
}
```

## Usage Example

```desi
class Container:
    pub mut count: int
    
    pub def __new__(self, n: int):
        self.count := n
    
    pub def __bool__(self) -> bool:
        return self.count > 0

let c = Container(5)
if c:
    print("has items")
```

## Files Modified

- `compiler/internal/check/info.go` - Added `BoolConversions` map
- `compiler/internal/check/stmt.go` - Added `__bool__` detection
- `compiler/internal/lower/lower_stmt.go` - Added `__bool__` call emission
- `examples/247_bool_dunder.desi` - Test file
