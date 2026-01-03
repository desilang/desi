# Enum Implementation Guide

This document covers the internal implementation of enums in the Desi compiler.

## Type Representation

Enums are represented as tagged unions in LLVM IR:

```llvm
%Status = type { i32, ptr }  ; { tag, payload_ptr }
```

- **tag** (`i32`): Discriminator indicating which variant is active (0-indexed)
- **payload_ptr** (`ptr`): Pointer to payload data (null for unit variants)

## Type Checker (`check/expr_field.go`)

### Variant Type Resolution

All enum variants (unit and payload) are typed as functions returning the enum type:

```go
// Lines 122-132
funcType := types.FuncOf(variantParams, resultType, false)
```

- Unit variant `Status.Pending` → `func() -> Status`
- Payload variant `Status.Error` → `func(str) -> Status`

This ensures consistent CallExpr handling: `Status.Pending()` works like `Status.Error("msg")`.

## Lowering (`lower/lower_expr.go`)

### Enum Variant Detection (lines 1167-1205)

The lowering phase must distinguish:
1. **Enum variant access**: `Status.Error` (base is type name)
2. **Struct field access**: `worker.status` (base is variable, field has enum type)

Detection uses two methods:
1. Check `info.Idents[id]` for `SymType` with `Enum` type
2. Fallback: Check `info.Types[x]` for `func() -> Enum`

### IsExpr for User Enums (lines 1047-1114)

The `is` operator with FieldExpr patterns (e.g., `value is Status.Pending()`) extracts the scrutinee's tag and compares to the variant's known tag value:

```go
// Extract tag from LHS enum struct
tagPtr := ls.b.FreshTemp("tag_ptr")
ls.b.Emit(&hir.GetElementPtr{Type: "i8", Base: lhs, Indices: []hir.Value{hir.ConstInt{Text: "0"}}, Dst: tagPtr})
tag := ls.b.FreshTemp("tag")
ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

// Compare to variant's tag value
ls.b.Emit(&hir.BinaryOp{Op: "==", LHS: tag, RHS: hir.ConstInt{Text: fmt.Sprintf("%d", variantTag)}, Dst: dst, Type: "i1"})
```

## Common Pitfalls

1. **Unit variants need parentheses**: `let v = Status.Pending()` not `Status.Pending`
2. **Struct field vs variant**: Always check if base is a type symbol before treating as variant
3. **Tag extraction**: Use `i8` GEP type for opaque byte access to enum struct
4. **ConstNull vs string "null"**: Use `hir.ConstNull{}` for null pointers, not `hir.ConstStr{Text: "null"}`. The LLVM backend correctly emits "null" for ConstNull but a string constant for ConstStr.

## Match Expression Return Values

Match expressions return values by storing to a result alloca and loading at merge point:

```go
// In match_lower.go
res := ls.lowerExpr(arm.Result)
if llvmResType != "void" {
    ls.b.Emit(&hir.Store{Dst: resPtr, Val: res})
}
```

To return the match value from a function, use explicit `return`:
```desi
def stringify(val: Status) -> str:
    let result = match val:
        Status.Pending(): "pending"
        _: "other"
    return result
```
