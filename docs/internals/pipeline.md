# Pipeline Operator Implementation

The pipeline operator (`|>`) allows chaining function and method calls by injecting the left-hand side (LHS) expression as the first argument of the right-hand side (RHS) call.

## Syntax

```python
value |> function()
value |> obj.method()
```

## Lowering Strategy

The lowering phase (`compiler/internal/lower/lower_expr.go`) performs a syntactic desugaring of the pipeline operator.

Transformation:
```python
LHS |> RHS(args...)
```
becomes:
```python
RHS(LHS, args...)
```

The LHS expression is injected as the **first positional argument** to the call.

## Type Checking

The type checker (`compiler/internal/check/expr_binary.go`) validates the pipeline expression by resolving the RHS target and checking argument compatibility.

### 1. Function Targets (`Ident`)
If the RHS is an identifier (e.g., `val |> func`), the checker:
1. Resolves the function overload set.
2. Synthesizes a new argument list: `[LHS] + ExplicitArgs`.
3. Performs overload resolution using this synthesized list.

### 2. Method Targets (`FieldExpr`)
If the RHS is a method call (e.g., `val |> obj.method()`), the checker:
1. Resolves the method on the receiver type (e.g., `obj`'s type).
2. Matches arguments against the method's signature.
   - **Note:** Instance methods have an implicit `self` parameter at index 0. The explicit parameters start at index 1.
   - The checker matches `[LHS] + ExplicitArgs` against `MethodParams[1:]`.

## Edge Cases

- **Argument Ordering:** The LHS is always the *first* argument.
  - `10 |> sub(5)` becomes `sub(10, 5)`.
- **Chaining:** The operator is left-associative.
  - `a |> b() |> c()` parses as `(a |> b()) |> c()`, which becomes `c(b(a))`.
