# Dunder Methods Implementation

This document describes the implementation of Python-style dunder (double underscore) methods in Desi.

## Supported Dunders

| Dunder | Syntax | Description |
|--------|--------|-------------|
| `__new__` | `Class()` | Constructor |
| `__getitem__` | `obj[idx]` | Index access |
| `__setitem__` | `obj[idx] := val` | Index assignment |
| `__len__` | `len(obj)` | Custom length |
| `__contains__` | `x in obj` | Membership test |
| `__format__` | `f"{obj:spec}"` | F-string formatting with spec |
| `__repr__` | `f"{obj}"`, `print(obj)` | Fallback string representation |
| `__eq__`, `__ne__`, etc. | `obj == other` | Comparison operators |
| `__add__`, `__sub__`, etc. | `obj + other` | Arithmetic operators |

## Implementation Pattern

All dunders follow the same pattern:

1. **Type Checker** registers dunder in `cls.Dunders` map
2. **Lowering** desugars syntax to function call: `ClassName___dunder__(obj, args...)`
3. **Code Generation** emits LLVM call to mangled function name

### `__getitem__` Example

**Desi Code:**
```desi
class MyList:
    pub def __getitem__(self, index: int) -> int:
        return self.data[index]

let x = arr[5]  # Desugars to MyList___getitem__(arr, 5)
```

**Lowering:** `compiler/internal/lower/lower_expr.go`
```go
if _, found := cls.Dunders["__getitem__"]; found {
    mangledName := fmt.Sprintf("%s___getitem__", cls.Name)
    ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: []hir.Value{obj, idx}})
}
```

### `__setitem__` Example

**Important:** Uses `:=` for mutation (not `=`)

```desi
class MyList:
    pub def __setitem__(self, index: int, value: int):
        self.data[index] := value

arr[0] := 999  # Desugars to MyList___setitem__(arr, 0, 999)
```

**Lowering:** `compiler/internal/lower/lower_stmt.go` (lines 326-351)

### `__contains__` Example

```desi
class MySet:
    pub def __contains__(self, item: int) -> bool:
        for x in self.data:
            if x == item:
                return true
        return false

if 20 in myset:  # Desugars to MySet___contains__(myset, 20)
    print("found")
```

**Lowering:** `compiler/internal/lower/lower_expr.go` (lines 1618-1626)

## List Mutation

**Issue Fixed:** `list[index] := value` wasn't calling `list_set`

**Location:** `compiler/internal/lower/lower_stmt.go` (lines 353-377)

```go
if _, ok := objType.(*types.List); ok {
    // Sign-extend index to i64
    ls.b.Emit(&hir.Cast{Dst: idx64, Src: idx, Type: "i64"})
    // Cast value to ptr
    ls.b.Emit(&hir.Cast{Dst: valPtr, Src: val, Type: "ptr"})
    // Call list_set
    ls.b.Emit(&hir.Call{Fn: "list_set", Args: []hir.Value{list, idx64, valPtr}})
}
```

### `__format__` and `__repr__` Fallback Chain

**F-string formatting follows Python-like fallback:**
1. If `__format__(self, spec: str) -> str` exists, call it
2. Else if `__repr__(self) -> str` exists, call it  
3. Else output `<?>`

**Key Design Decision:** The format specifier is passed as a string parameter, allowing custom formatting:
- `f"{obj:short}"` → calls `__format__(obj, "short")`
- `f"{obj}"` → calls `__format__(obj, "")` with empty spec

**Implementation:** `compiler/internal/lower/lower_expr.go`

```go
// FStringExpr case (with format spec)
if _, hasFormat := cls.Dunders["__format__"]; hasFormat {
    formatFn := cls.Name + "___format__"
    ls.b.Emit(&hir.Call{Dst: res, Fn: formatFn, Args: []hir.Value{val, specVal}})
}

// Default case (no spec) - fallback chain
if _, hasFormat := cls.Dunders["__format__"]; hasFormat {
    // Call __format__ with empty spec
} else if _, hasRepr := cls.Dunders["__repr__"]; hasRepr {
    reprFn := cls.Name + "___repr__"
    ls.b.Emit(&hir.Call{Dst: res, Fn: reprFn, Args: []hir.Value{val}})
}
```

**Type Checker Fix:** Added `*ast.FStringExpr` case in `check/expr.go` to populate type info:
```go
case *ast.FStringExpr:
    innerType := c.typ(x.X)
    c.info.Types[e] = innerType
```

## Tests

- `examples/206_custom_dunders.desi` - `__getitem__`, `__setitem__`, `__len__`
- `examples/210_custom_contains.desi` - `__contains__` dunder
- `examples/293_format_dunder.desi` - `__format__` and `__repr__` fallback

## Commits

- `6864dfe` - List mutation fix (list_set call)
- `220b33e` - Dunder method tests
- `62b6278` - `__contains__` dunder support
