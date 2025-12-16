# Lambda Functions

Lambda functions in Desi provide a concise way to create anonymous functions inline. Unlike many languages, Desi requires explicit type annotations for both parameters and return values to maintain type safety.

## Syntax

```desi
lambda<ReturnType> param1: Type1, param2: Type2: expression
```

### Key Components

1. **`lambda` keyword**: Marks the start of a lambda expression
2. **`<ReturnType>`**: Mandatory return type annotation in angle brackets
3. **Parameters**: Comma-separated list of `name: Type` pairs
4. **`:` separator**: Separates parameter list from body
5. **Body**: Single expression that is returned (or executed for `none` return type)

## Basic Examples

### Single Parameter

```desi
let double: int = lambda<int> x: int: x * 2
let result: int = double(21)  # 42
```

### Multiple Parameters

```desi
let add: int = lambda<int> a: int, b: int: a + b
let sum: int = add(10, 20)  # 30
```

### No Parameters

```desi
let greet: str = lambda<str>: "Hello, World!"
print(greet())  # Hello, World!
```

### Void Return (Side Effects Only)

```desi
let log: none = lambda<none> msg: str: print(msg)
log("Debug message")
```

## Type System Integration

### Stored in Variables

Lambdas are first-class values and can be stored in variables:

```desi
let transform: int = lambda<int> x: int: x * 3
let value: int = transform(5)  # 15
```

**Important**: The variable type annotation should match the lambda's **return type**, not its function type. The compiler automatically infers that the variable holds a callable function.

### Passed as Arguments

```desi
def apply_twice(f: int, x: int) -> int:
    f(f(x))

let square: int = lambda<int> n: int: n * n
let result: int = apply_twice(square, 3)  # 81
```

## Advanced Usage

### String Operations

```desi
let exclaim: str = lambda<str> s: str: s + "!"
print(exclaim("Hello"))  # Hello!
```

### Nested Calls

```desi
let double: int = lambda<int> x: int: x * 2
let triple: int = lambda<int> x: int: x * 3
let result: int = triple(double(5))  # 30
```

### Closures (Capturing Outer Variables)

Lambdas can capture variables from their enclosing scope:

```desi
let x: int = 10
let add_x: int = lambda<int> y: int: x + y
print(add_x(5))  # 15
```

## Do's and Don'ts

### ✅ Do

- **Always specify return type**: `lambda<int>` even if it seems obvious
- **Annotate all parameters**: `lambda<int> x: int: ...`
- **Use for simple transformations**: Perfect for `map`, `filter`, `reduce`
- **Store in variables for reuse**: `let double: int = lambda<int> ...`
- **Match variable type to return type**: `let result: int = lambda<int> ...`

### ❌ Don't

- **Don't use complex logic in lambda bodies**: Extract to a named function instead
- **Don't omit type annotations**: `lambda x: x * 2` ❌ (will not compile)
- **Don't use statements in lambda body**: Only single expressions allowed
- **Don't try to reassign captured variables**: Closures are read-only

## Common Patterns

### With Higher-Order Functions

```desi
# Map
let numbers: list[int] = [1, 2, 3, 4, 5]
let doubled: list[int] = numbers.map(lambda<int> x: int: x * 2)

# Filter
let evens: list[int] = numbers.filter(lambda<bool> x: int: x % 2 == 0)

# Reduce
let sum: int = reduce(numbers, lambda<int> acc: int, x: int: acc + x, 0)
```

### Void Lambdas for Side Effects

```desi
let items: list[str] = ["a", "b", "c"]
items.map(lambda<none> x: str: print(x))  # Prints each item
```

## Implementation Details

### Compiler Transformation

Lambdas are **not** function pointers at runtime. Instead, the compiler performs an AST transformation:

1. **Desugaring Phase**: Each lambda is converted to a hidden top-level function
2. **Naming**: Hidden functions are named `__lam$0`, `__lam$1`, etc.
3. **Aliasing**: Variable bindings track which hidden function they reference

Example transformation:

```desi
# Source code
let double: int = lambda<int> x: int: x * 2
let result: int = double(21)
```

```desi
# After desugaring (conceptual)
def __lam$0(x: int) -> int:
    return x * 2

let double: int = __lam$0  # Alias tracked
let result: int = __lam$0(21)  # Call resolved via alias
```

### Why This Design?

1. **Simplicity**: No function pointer runtime needed
2. **Performance**: Direct function calls, no indirection
3. **Type Safety**: All types known at compile time
4. **Debugging**: Hidden functions appear in stack traces with clear names

### Void Return Special Case

For `lambda<none>`, the body is executed as a statement followed by an empty return:

```desi
# Source
lambda<none> x: int: print(x)

# Desugared
def __lam$N(x: int) -> none:
    print(x)
    return
```

This ensures void functions don't accidentally return values.

## Comparison with Other Languages

| Language | Syntax | Type Inference |
|----------|--------|----------------|
| **Desi** | `lambda<int> x: int: x + 1` | Explicit types required |
| Python | `lambda x: x + 1` | Dynamic typing |
| JavaScript | `x => x + 1` | No types |
| Rust | `\|x\| x + 1` | Type inference |
| TypeScript | `(x: number) => x + 1` | Return type inferred |

Desi's explicit typing prevents ambiguity and catches errors at compile time.

## Common Errors

### Missing Return Type

```desi
# ❌ Error: lambda requires explicit return type
let f = lambda x: int: x * 2
```

```desi
# ✅ Correct
let f: int = lambda<int> x: int: x * 2
```

### Missing Parameter Type

```desi
# ❌ Error: lambda parameters must be typed
let f: int = lambda<int> x: x * 2
```

```desi
# ✅ Correct
let f: int = lambda<int> x: int: x * 2
```

### Type Mismatch

```desi
# ❌ Error: lambda body type mismatch
let f: str = lambda<str> x: int: x * 2  # Returns int, not str
```

```desi
# ✅ Correct
let f: int = lambda<int> x: int: x * 2
```

### Wrong Variable Type

```desi
# ❌ Error: variable type should match return type
let f: (int) -> int = lambda<int> x: int: x * 2
```

```desi
# ✅ Correct: variable type is the return type
let f: int = lambda<int> x: int: x * 2
```

## Future Enhancements

Potential future features (not yet implemented):

- Generic lambdas: `lambda<T> x: T: x`
- Multi-line lambda bodies with `{` `}` blocks
- Partial application and currying
- Implicit return type inference from body

## Contributing

If you're working on lambda-related compiler features:

1. **Lexer**: Token definitions in `compiler/internal/token/`
2. **Parser**: `compiler/internal/parse/expr_lambda_keyword.go`
3. **Type Checker**: `compiler/internal/check/expr.go` (LambdaExpr case)
4. **Desugaring**: `compiler/internal/lower/async_lambda.go`
5. **Aliasing**: `compiler/internal/lower/module_lower.go` and `hir_lower.go`

Tests are in `examples/165_lambda_basic.desi` with comprehensive edge cases.
