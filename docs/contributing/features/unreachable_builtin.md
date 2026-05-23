# `unreachable()` Builtin Implementation

## Overview

`unreachable()` marks code paths that should never execute. If reached, it calls `__desi_unreachable()` in the C runtime which prints a message to stderr and exits with code 1.

```desi
def classify(n: int) -> str:
    if n > 0:
        return "positive"
    elif n < 0:
        return "negative"
    else:
        return "zero"

# Exhaustive guard — unreachable() documents the invariant
let kind = classify(x)
if kind == "positive" or kind == "negative" or kind == "zero":
    print(kind)
else:
    unreachable()
```

## Implementation

Follows the same pattern as `todo()`.

### C Runtime (`compiler/runtime/builtins.c`)

```c
void __desi_unreachable(void) {
    fprintf(stderr, "unreachable: entered unreachable code\n");
    exit(1);
}
```

### Type Checker (`compiler/internal/check/info.go`, `expr_call.go`)

Registered in the prelude with return type `never` (same as `todo()`):

```go
// info.go
UnreachableCalls map[*ast.CallExpr]bool

// expr_call.go
if id.Name == "unreachable" && len(call.Args) == 0 {
    c.info.UnreachableCalls[call] = true
    return types.Never
}
```

### Lowerer (`compiler/internal/lower/lower_call.go`)

```go
if calleeName == "unreachable" {
    ls.b.Emit(&hir.Call{
        Fn:   "__desi_unreachable",
        Args: []hir.Value{},
        Type: "void",
    })
    ls.b.Emit(&hir.Unreachable{})
    ls.terminated = true
    return hir.ConstInt{Text: "0"}
}
```

## Files Modified

- `compiler/runtime/builtins.c` — `__desi_unreachable()` C function
- `compiler/internal/check/info.go` — `UnreachableCalls` map
- `compiler/internal/check/expr_call.go` — type-checker registration
- `compiler/internal/lower/lower_call.go` — lowering dispatch
- `examples/517_unreachable.desi` — test case
