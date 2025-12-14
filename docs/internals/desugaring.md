# Desugaring in Desi

Desugaring is a compile-time transformation that rewrites high-level syntax into simpler, more fundamental forms before type-checking and lowering.

## Overview

| Phase | What Happens |
|-------|--------------|
| Parse | Source code → AST |
| **Desugar** | AST rewriting (this phase) |
| Type Check | Semantic analysis |
| Lower | AST → HIR |
| Codegen | HIR → LLVM IR |

## Current Desugars

### map/filter → List Comprehension

```python
# Before desugaring
map(double, nums)
filter(is_even, nums)

# After desugaring
[double(__x) for __x in nums]
[__x for __x in nums if is_even(__x)]
```

**Location**: `compiler/internal/check/desugar_map_filter.go`

## Performance

Desugaring has **zero runtime overhead** - the generated code is identical to hand-written equivalents.

Compile-time cost is O(n) where n = number of AST nodes, achieved by doing all transformations in a single AST walk.

## Adding New Desugars

To maintain O(n) complexity:

### Option 1: Extend `desugarExpr()` (Preferred)

Add a new case to the switch statement:

```go
func desugarExpr(e ast.Expr) ast.Expr {
    switch x := e.(type) {
    case *ast.CallExpr:
        // ... existing map/filter handling ...
        
        // NEW: Add your desugar here
        if id.Name == "reduce" {
            return buildReduceLoop(args)
        }
    }
}
```

### Option 2: Extend `desugarBlock()`

For statement-level transformations, add a case to `desugarBlock()`:

```go
func desugarBlock(b *ast.Block) {
    for i := range b.Stmts {
        switch s := b.Stmts[i].(type) {
        // ... existing cases ...
        
        // NEW: Handle new statement patterns
        case *ast.YourNewStmt:
            b.Stmts[i] = desugarYourNewStmt(s)
        }
    }
}
```

### What to Avoid

❌ **Don't create separate AST walks** for each desugar type:

```go
// BAD: O(k*n) complexity
func DesugarAll(mod *ast.Module) {
    desugarMapFilter(mod)  // walk 1
    desugarReduce(mod)     // walk 2
    desugarFlatMap(mod)    // walk 3
}
```

✅ **Do combine patterns** in a single walk:

```go
// GOOD: O(n) complexity
func desugarExpr(e ast.Expr) ast.Expr {
    switch x := e.(type) {
    case *ast.CallExpr:
        switch id.Name {
        case "map": ...
        case "filter": ...
        case "reduce": ...
        case "flatmap": ...
        }
    }
}
```

## Design Rationale

1. **Simplicity**: Fewer AST node types to handle in type-checker and lowering
2. **Optimization**: Comprehensions can be optimized uniformly
3. **Extensibility**: Easy to add new higher-order functions without backend changes
4. **Debugging**: Users see familiar comprehension syntax in error messages

## Future Desugars

Potential candidates for desugaring:

- `reduce(f, xs, init)` → explicit loop with accumulator
- `flatmap(f, xs)` → nested comprehension
- `zip(xs, ys)` → already handled specially in type-checker, could desugar
- Pattern matching in assignments → explicit match expressions
