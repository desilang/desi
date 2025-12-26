# Generic Method Monomorphization - Implementation Guide

## Overview

When a generic class `Base<T>` is inherited by a concrete class `Child(Base<int>)`, all methods from `Base<T>` must have their type parameter `T` replaced with the concrete type `int` in the method bodies.

## Problem Statement

Before this fix, inherited methods from generic base classes were re-lowered without type substitution in their bodies. This caused LLVM IR type mismatches (e.g., returning `ptr` when `i32` was expected).

## Implementation (`lower/class_lower.go`)

### 1. Build Substitution Map

When lowering inherited methods, build a map from type parameters to concrete types:

```go
subst := make(map[string]types.T)
if cls.Base != nil && len(cls.Base.TypeParams) > 0 && cd.Bases[0].Params != nil {
    for i, tp := range cls.Base.TypeParams {
        if i < len(cd.Bases[0].Params) {
            concreteType := resolveASTTypeNameToT(cd.Bases[0].Params[i], info)
            if concreteType != nil {
                subst[tp.Name] = concreteType
            }
        }
    }
}
```

### 2. Apply Body Substitution

Use the existing `substituteHIRFuncBody` function to replace types:

```go
if len(subst) > 0 {
    substituteHIRFuncBody(hirFn, subst, cls.Base)
}
```

### 3. Update Return/Param Types

The method signature must also use concrete types:

```go
methodType := cls.Methods[name]
if methodType != nil && methodType.Ret != nil {
    hirFn.RetType = lowerType(methodType.Ret)
}
```

### Helper: `resolveASTTypeNameToT`

Converts `*ast.TypeName` (from inheritance declaration) to `types.T`:

```go
func resolveASTTypeNameToT(tn *ast.TypeName, info *check.Info) types.T {
    switch tn.Name {
    case "int": return types.Int
    case "str": return types.Str
    // ... other builtins
    case "list": return types.ListOf(resolveASTTypeNameToT(tn.Params[0], info))
    // ... other containers
    }
    // Look up user-defined classes in info.Types
}
```

## Example

```python
class Container<T>:
    pub val: T
    pub def get(self) -> T:
        return self.val  # T here

class IntBox(Container<int>):
    pass

# After monomorphization:
# IntBox.get() returns i32, body uses i32 not T
```

## Files Modified

- `compiler/internal/lower/class_lower.go` - Main implementation
- Uses `substituteHIRFuncBody` from `class_monomorph.go`

## Testing

Key test files:
- `examples/135_generic_inheritance_simple.desi`
- `examples/136_generic_inheritance.desi`
- `examples/185_generic_method_body_test.desi`
