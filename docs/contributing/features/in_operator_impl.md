# Membership Operator (`in`) Implementation

This document describes the implementation of the `in` membership operator in Desi.

## Syntax

```desi
x in collection      # Returns bool
x in list[T]         # True if x is in the list
x in set[T]          # True if x is in the set  
substr in str        # True if substr is in string
x in custom_obj      # Calls __contains__ dunder
```

## Implementation

### Type Checker

**Location:** `compiler/internal/check/expr_binary.go` (lines 479-557)

```go
case "in":
    // x in list[T] -> bool
    if listT, ok := rt.(*types.List); ok {
        if types.Assignable(listT.Elem, lt) {
            c.info.Types[x] = types.Bool
            return types.Bool
        }
    }
    
    // x in set[T] -> bool
    if setT, ok := rt.(*types.Set); ok {
        // Similar logic...
    }
    
    // Custom class with __contains__
    if cls, ok := rt.(*types.Class); ok {
        if ft, found := cls.Dunders["__contains__"]; found {
            c.info.Types[x] = types.Bool
            return types.Bool
        }
    }
```

### Lowering

**Location:** `compiler/internal/lower/lower_expr.go` (lines 1575-1628)

For built-in types, calls runtime functions:

| Type | Runtime Function |
|------|------------------|
| `list[T]` | `list_contains(list, elem_ptr)` |
| `set[T]` | `set_contains(set, elem_ptr)` |
| `str` | `string_contains(haystack, needle)` |

**Note:** Runtime returns `i32`, converted to `i1` for LLVM branch:

```go
foundI32 := ls.b.FreshTemp("found_i32")
ls.b.Emit(&hir.Call{Dst: foundI32, Fn: "list_contains", ...})

// Convert i32 to i1
dst := ls.b.FreshTemp("found")
ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: foundI32, RHS: hir.ConstInt{Text: "0"}, Dst: dst, Type: "i1"})
```

For custom classes:

```go
if cls, ok := rhsType.(*types.Class); ok {
    mangledName := fmt.Sprintf("%s___contains__", cls.Name)
    ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: []hir.Value{rhs, lhs}, Type: "i1"})
}
```

## Custom `__contains__` Dunder

Classes can implement custom membership logic:

```desi
class MySet:
    pub data: list[int]
    
    pub def __contains__(self, item: int) -> bool:
        for x in self.data:
            if x == item:
                return true
        return false
```

**Design Decision:** The `__contains__` method decides how to compare elements:
- Python convention: value equality via `__eq__` (not memory address)
- User can implement any logic

## Tests

- `examples/207_contains_test.desi` - `in` for lists
- `examples/210_custom_contains.desi` - `__contains__` dunder

## Commits

- `908969b` - `in` operator for lists/sets/strings
- `62b6278` - `__contains__` dunder for custom classes
