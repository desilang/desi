# Lambda Internals

This document explains the implementation of lambdas and closures for contributors.

## Lambda Desugaring

Lambdas are desugared to top-level functions in `async_lambda.go`:

```go
// DesugarAsyncLambdas(mod, info) transforms:
lambda<int> x: int: x + 1

// Into hidden function:
def __lam$0(x: int) -> int:
    return x + 1
```

## Capture Handling

### How Captures Work

1. **Free variable analysis** - `CollectFreeVars()` finds variables used in lambda body but not defined as parameters
2. **Type lookup** - Captured variable types are looked up from `info.Types`
3. **Extra parameters** - Captures become extra parameters on the synthesized function
4. **Call-site resolution** - At call site, captured values are appended as extra arguments

### LambdaCaptures Map

```go
// In check/info.go
LambdaCaptures map[string][]string // "__lam$0" -> ["offset", "scale"]
```

Populated during desugaring, used at call-site lowering.

### Example Flow

```desi
let offset = 10
let add_offset = lambda<int> x: int: x + offset
add_offset(5)
```

**Step 1: Desugaring** (async_lambda.go)
```go
// Synthesized function with captured param
def __lam$0(x: int, offset: int) -> int:
    return x + offset

// Store in info
info.LambdaCaptures["__lam$0"] = ["offset"]
info.LambdaAliases["add_offset"] = "__lam$0"
```

**Step 2: Call-site lowering** (lower_call.go:2092)
```go
// When lowering add_offset(5):
callee := "__lam$0"  // via LambdaAliases
args := [5]          // explicit arg

// Check LambdaCaptures
captures := info.LambdaCaptures["__lam$0"]  // ["offset"]
for _, capName := range captures {
    args = append(args, hir.Var{Name: capName})
}
// args = [5, offset]
```

**Step 3: Generated IR**
```llvm
define i32 @__lam$0(i32 %x, i32 %offset) {
    %result = add i32 %x, %offset
    ret i32 %result
}

; Call site
%result = call i32 @__lam$0(i32 5, i32 %offset)
```

## TaskGroup Integration

For `tg.run(lambda<none>: ...)`, a different approach is needed because:
1. TaskGroup spawns functions with signature `fn(void* ctx)`
2. Captured values must be packed into a context struct

### Context Struct Building

```go
// lower_call.go lines 618-666
if numCaptures > 0 {
    ctxPtr := malloc(numCaptures * 8)
    for i, capName := range captures {
        offset := GEP(ctxPtr, i * 8)
        if isPrimitive(type) {
            boxPtr := malloc(8)  // Box primitives
            store(value, boxPtr)
            store(boxPtr, offset)
        } else {
            store(value, offset)
        }
    }
}
```

### Wrapper Generation

`emitTaskGroupWrapper()` generates a wrapper that:
1. Receives `void* ctx`
2. Unpacks captured values from context
3. Calls the actual lambda function

## Primitive vs Pointer Captures

| Type | Regular Lambda | TaskGroup |
|------|---------------|-----------|
| int/float/bool | Passed directly | Boxed in context |
| str/class/ptr | Passed directly | Stored directly |

Primitives need boxing in TaskGroup because the context is an array of pointers.

## Key Files

| File | Purpose |
|------|---------|
| `async_lambda.go` | Desugaring, free var analysis, type lookup |
| `capture_analysis.go` | `CollectFreeVars()` implementation |
| `lower_call.go:2092` | Call-site capture argument appending |
| `lower_call.go:608` | TaskGroup context struct building |
| `check/info.go` | `LambdaCaptures` and `LambdaAliases` maps |

## Test Coverage

| Test File | Coverage |
|-----------|----------|
| 165_lambda_basic.desi | Basic lambda syntax |
| 266_closure_capture.desi | TaskGroup with closure |
| 267_concurrent_edge_cases.desi | Multiple captures, primitives |
| 286_primitive_captures.desi | int/float captures in regular lambdas |
