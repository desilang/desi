# Keyword Arguments (**kwargs) in Desi

**Status**: ✅ Implemented  
**Since**: v0.10  
**Related**: [Variadic Functions](varargs.md)

---

## Overview

Desi supports Python-style `**kwargs` — keyword arguments that are collected into a `dict[str, T]` at the call site. This enables flexible function signatures like Django's `Model.objects.create(name="Alice", age=30)`.

## Syntax

### Declaration

```desi
def function_name(**kwargs_name: ValueType) -> ReturnType:
    # kwargs_name is dict[str, ValueType]
```

### With Positional Parameters

```desi
def function_name(pos1: T1, pos2: T2, **kwargs_name: ValueType) -> ReturnType:
    # kwargs_name collects all remaining named arguments
```

### Call Site

```desi
function_name(key1="value1", key2="value2")
function_name(pos1_val, pos2_val, key1="value1", key2="value2")
```

## Type Annotation

| Annotation | Dict Type | Use Case |
|------------|-----------|----------|
| `**fields: str` | `dict[str, str]` | All values are strings |
| `**fields: int` | `dict[str, int]` | All values are integers |
| `**fields: Any` | `dict[str, Any]` | Mixed types (Django-style) |

## Compiler Implementation

### Pipeline

```
Parser → AST → Type Checker → Overload Resolver → Lowerer
```

### 1. Parser (`parse/decl.go`)

- Token `**` is lexed as `token.POW` (power/exponent operator reused)
- `parseParams()` checks for `POW` before `STAR` (kwargs vs varargs)
- Sets `Param.Kwargs = true` on the AST node

### 2. AST (`ast/nodes.go`)

```go
type Param struct {
    Name     Ident
    Type     *TypeName
    Default  Expr
    Mode     ParamMode
    Variadic bool      // true if *args
    Kwargs   bool      // true if **kwargs  ← NEW
    Span     diag.Span
}
```

### 3. Type Checker (`check/check.go`)

- `collectFunc()`: wraps `**kwargs: T` as `dict[str, T]` in the function signature
- `checkFunc()`: binds kwargs param as `dict[str, T]` in the function body scope
- `types.Func` has `HasKwargs bool` field to propagate this info

### 4. Overload Resolution (`check/expr_overload.go`)

Two functions updated:

- **`resolveCallAgainstSet()`**: When a candidate has `HasKwargs`, unknown named args are accepted and type-checked against the dict's value type
- **`canonicalizeForCandidate()`**: Same logic for the named-args path

Key behavior: named args that don't match any explicit param name are collected as kwargs entries, rather than being rejected.

### 5. Lowerer (`lower/lower_call.go`)

`lowerKwargsCall()` builds the kwargs dict at the call site:

1. Lower positional args normally
2. `dict_new(key_type=str, ...)` — create empty dict
3. For each `key=value` named arg:
   - `dict_insert(dict, key_str, &value, type_tag)` 
4. Append dict pointer as last arg
5. Emit `hir.Call` with all args

For `**kwargs: Any`, per-entry type tags are used:
- `str` → `TYPE_TAG_STR = 1`
- `int` → `TYPE_TAG_INT = 0`
- `float` → `TYPE_TAG_FLOAT = 3` (BitCast to i64)
- `bool` → `TYPE_TAG_BOOL = 2`

## Rules & Constraints

1. Only **one** `**kwargs` parameter per function
2. Must be the **last** parameter
3. Cannot have both `*args` and `**kwargs` in the same function
4. No **default value** allowed for `**kwargs`
5. Kwargs keys are always `str` (the dict key type is always `str`)

## Examples

### Basic String Kwargs

```desi
def show(**opts: str) -> int:
    print("opts received")
    0

show(name="Alice", email="alice@test.com")
```

### Any-typed Kwargs (Mixed Types)

```desi
def create(**fields: Any) -> int:
    print("creating record")
    0

create(name="Alice", age=30, score=3.14, active=true)
```

### Positional + Kwargs

```desi
def insert(table: str, **fields: Any) -> int:
    print("table: " + table)
    0

insert("users", name="Alice", age=30)
```

## Future Work

- [ ] Dict iteration inside kwargs functions (`for key in kwargs`)
- [ ] LSP/IDE autocomplete for Model field names
- [ ] ORM `create()` using `**kwargs`
