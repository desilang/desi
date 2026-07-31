# Lambda Functions

Lambda functions provide a concise way to create anonymous functions inline. Desi requires explicit type annotations for both parameters and return values to maintain type safety.

## Syntax

```desi
lambda<ReturnType> param1: Type1, param2: Type2: expression
```

### Components

- **`lambda` keyword** - Marks the start of a lambda expression
- **`<ReturnType>`** - Mandatory return type annotation in angle brackets  
- **Parameters** - Comma-separated `name: Type` pairs
- **`:` separator** - Separates parameter list from body
- **Body** - Single expression (returned or executed for `none` type)

## Basic Examples

=== "Single Parameter"
    ```desi
    let double: int = lambda<int> x: int: x * 2
    let result: int = double(21)  # 42
    ```

=== "Multiple Parameters"
    ```desi
    let add: int = lambda<int> a: int, b: int: a + b
    let sum: int = add(10, 20)  # 30
    ```

=== "No Parameters"
    ```desi
    let greet: str = lambda<str>: "Hello, World!"
    print(greet())  # Hello, World!
    ```

=== "Void Return"
    ```desi
    let log: none = lambda<none> msg: str: print(msg)
    log("Debug message")
    ```

## Type Annotations

!!! warning "Both types required"
    Desi requires **explicit type annotations** for both parameters and return type. This catches errors at compile time.

### Variable Binding

The variable type annotation should match the lambda's **return type**:

```desi
let transform: int = lambda<int> x: int: x * 3
let value: int = transform(5)  # 15
```

!!! tip "Type matching"
    Write `let varname: ReturnType = lambda<ReturnType>...`  
    The compiler automatically tracks that the variable holds a callable function.

## Higher-Order Functions

Lambdas work seamlessly with map, filter, and reduce:

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]

# Map - transform each element
let doubled: list[int] = numbers.map(lambda<int> x: int: x * 2)
# [2, 4, 6, 8, 10]

# Filter - keep only matching elements
let evens: list[int] = numbers.filter(lambda<bool> x: int: x % 2 == 0)
# [2, 4]

# Reduce - accumulate a result
let sum: int = reduce(numbers, lambda<int> acc: int, x: int: acc + x, 0)
# 15
```

## Closures

Lambdas can capture variables from their enclosing scope:

```desi
let x: int = 10
let add_x: int = lambda<int> y: int: x + y
print(add_x(5))  # 15
```

### Capturing Different Types

All types can be captured - primitives, strings, and objects:

```desi
# Primitive capture
let offset: int = 100
let scale: float = 2.5
let calc = lambda<float> n: int: (n + offset) as float * scale
print(calc(10))  # 275.0

# Multiple captures
let prefix = "Hello, "
let suffix = "!"
let greet = lambda<str> name: str: prefix + name + suffix
print(greet("World"))  # Hello, World!
```

!!! note "Read-only captures"
    Captured variables are read-only. Lambdas cannot reassign outer variables.

!!! tip "Performance"
    Captured primitives are passed by value (no heap allocation). 
    Captured objects are passed by reference (pointer copy).


## Common Patterns

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

### Side Effects with `none`

```desi
let items: list[str] = ["a", "b", "c"]
items.map(lambda<none> x: str: print(x))  # Prints each item
```

## Do's and Don'ts

### ✅ Do

- Always specify return type: `lambda<int>`
- Annotate all parameters: `x: int`
- Use for simple transformations
- Store in variables for reuse

### ❌ Don't

- Use complex logic (extract to named function)
- Omit type annotations
- Try to use statements in body (expressions only)
- Attempt to reassign captured variables

## Common Errors

!!! failure "Missing return type"
    ```text
    # ❌ Error: lambda requires explicit return type
    let f = lambda x: int: x * 2
    ```
    
    ```desi
    # ✅ Correct
    let f: int = lambda<int> x: int: x * 2
    ```

!!! failure "Missing parameter type"
    ```desi
    # ❌ Error: lambda parameters must be typed
    let f: int = lambda<int> x: x * 2
    ```
    
    ```desi
    # ✅ Correct
    let f: int = lambda<int> x: int: x * 2
    ```

!!! failure "Type mismatch"
    ```desi
    # ❌ Error: body returns int, not str
    let f: str = lambda<str> x: int: x * 2
    ```
    
    ```desi
    # ✅ Correct
    let f: int = lambda<int> x: int: x * 2
    ```

## Implementation Details

### Compiler Transformation

Lambdas are **desugared** to hidden top-level functions during compilation:

1. Each lambda becomes a hidden function named `__lam$0`, `__lam$1`, etc.
2. Variable bindings track which hidden function they reference
3. Function calls are resolved via alias tracking

**Example transformation:**

```desi
# Source code
let double: int = lambda<int> x: int: x * 2
double(21)
```

The compiler rewrites it to a hidden top-level function. `$` is not valid in a
Desi identifier — these names exist only inside the compiler, so this is a
sketch of the result, not code you can write:

```text
# After desugaring (conceptual)
def __lam$0(x: int) -> int:
    return x * 2

let double: int = __lam$0  # Alias tracked
__lam$0(21)  # Call resolved
```

### Why This Design?

- **Simplicity** - No function pointer runtime needed
- **Performance** - Direct function calls, zero overhead
- **Type Safety** - All types known at compile time
- **Debugging** - Hidden functions appear in stack traces

### Void Return Special Case

For `lambda<none>`, the body is executed as a statement:

```text
# Source
lambda<none> x: int: print(x)

# Desugared (compiler-internal name)
def __lam$N(x: int) -> none:
    print(x)
    return
```

## Comparison with Other Languages

| Language | Syntax | Type Annotations |
|----------|--------|------------------|
| **Desi** | `lambda<int> x: int: x + 1` | Explicit (required) |
| Python | `lambda x: x + 1` | Dynamic typing |
| JavaScript | `x => x + 1` | No types |
| Rust | `\|x\| x + 1` | Type inference |
| TypeScript | `(x: number) => x + 1` | Return type inferred |

Desi's explicit typing prevents ambiguity and catches type errors at compile time.

## See Also

- [Functions](functions.md) - Named function syntax
- [Generics](generics.md) - Generic function types
- [Builtin Functions](../reference/builtins.md) - map, filter, reduce
