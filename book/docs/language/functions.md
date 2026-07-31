# Functions

Functions are the fundamental building blocks of Desi programs. They support typed parameters, overloading, async execution, and more.

## Basic Syntax

```desi
def function_name(param1: Type1, param2: Type2) -> ReturnType:
    # function body
    return value
```

### Simple Function

```desi
def add(a: int, b: int) -> int:
    return a + b

def main():
    let result: int = add(10, 20)
    print(result)  # 30
```

### No Parameters

```desi
def greet():
    print("Hello from Desi!")

def main():
    greet()  # Calls the function
```

### No Return Value

```desi
def log_message(msg: str) -> none:
    print(msg)
    # Implicit return (or explicit `return`)
```

!!! tip "Return Type Inference"
    If you omit the return type, Desi infers it from the function body:
    ```desi
    def double(x: int):     # Return type inferred as int
        return x * 2
    ```
    Best practice: Always declare return types for public APIs.

## Parameters

### Type Annotations

All parameters require type annotations:

```desi
def format_greeting(name: str, age: int) -> str:
    return f"Hello {name}, you are {age} years old"
```

### Parameter Modes

Desi supports three parameter modes:

=== "Normal (Copy/Move)"
    ```desi
    def increment(x: int) -> int:
        return x + 1  # x is copied in
    ```
    Default mode. Value is copied or moved into the function.

=== "`inout` (Mutable Reference)"
    ```desi
    def double_it(inout x: int) -> none:
        x = x * 2
    
    def main():
        let mut value: int = 5
        double_it(value)
        print(value)  # 10
    ```
    Allows the function to modify the caller's variable.

=== "`ref` (Immutable Borrow)"
    ```desi
    def print_length(ref items: list[int]) -> none:
        print(len(items))  # Read-only access
    ```
    Borrows the value for read-only access.

!!! warning "Mutable Variables Required for `inout`"
    To pass a variable as `inout`, it must be declared with `let mut`:
    ```desi
    let mut x: int = 5
    double_it(x)  # ✅ OK
    
    let y: int = 5
    double_it(y)  # ❌ Error: y is not mutable
    ```

### Variadic Functions

Use `*` prefix to accept variable number of arguments:

```desi
def add_all(first: int, *rest: int) -> int:
    let mut total: int = first
    for x in rest:
        total = total + x
    return total

def main():
    print(add_all(5))           # 5
    print(add_all(1, 2, 3))     # 6
    print(add_all(10, 20, 30))  # 60
```

!!! note "Variadic Parameter Position"
    The variadic parameter must be last in the parameter list.

### Keyword Arguments (`**kwargs`)

Use `**` prefix to accept keyword arguments as a dictionary:

```desi
def create_user(**fields: str) -> int:
    print("creating user")
    0

def main():
    create_user(name="Alice", email="alice@test.com")
```

The `**kwargs` parameter becomes `dict[str, T]` inside the function body.

#### Mixed Types with `Any`

For Django-style calls with different value types, use `**kwargs: Any`:

```desi
def insert(**fields: Any) -> int:
    print("inserting record")
    0

def main():
    # Mix of str, int, float, and bool values
    insert(name="Alice", age=30, score=3.14, active=true)
```

#### Combining Positional and Kwargs

You can mix regular positional parameters with `**kwargs`:

```desi
def create(table: str, **fields: Any) -> int:
    print("table: " + table)
    0

def main():
    create("users", name="Alice", age=30, email="alice@test.com")
```

**Rules:**

1. Only ONE `**kwargs` parameter allowed per function
2. `**kwargs` must be the LAST parameter
3. Cannot have both `*args` and `**kwargs` in the same function
4. No default value allowed for `**kwargs`

!!! tip "Python Developers"
    Desi's `**kwargs` works just like Python's — if you know Django's
    `User.objects.create(name="Alice", age=30)`, you'll feel right at home.

## Return Values

### Single Return

```desi
def square(x: int) -> int:
    return x * x
```

### Expression Return

For simple functions, omit `return` and just write the expression:

```desi
def double(x: int) -> int:
    x * 2  # Last expression is returned
```

### Multiple Returns with Tuples

```desi
def divide_with_remainder(a: int, b: int) -> tuple[int, int]:
    return (a / b, a % b)

def main():
    let (quotient, remainder) = divide_with_remainder(10, 3)
    print(quotient)   # 3
    print(remainder)  # 1
```

### Early Return

```desi
def find_first_negative(nums: list[int]) -> Option<int>:
    for n in nums:
        if n < 0:
            return Option.Some(n)
    return Option.Nothing
```

## Function Overloading

Define multiple functions with the same name but different parameter types:

```desi
def show(x: int) -> str:
    return f"int: {x}"

def show(x: bool) -> str:
    return f"bool: {x}"

def show(x: str) -> str:
    return f"str: {x}"

def show(x: str, y: int) -> str:
    return f"str+int: {x}, {y}"

def main():
    print(show(42))         # "int: 42"
    print(show(true))       # "bool: true"
    print(show("hello"))    # "str: hello"
    print(show("test", 5))  # "str+int: test, 5"
```

!!! tip "Overload Resolution"
    The compiler selects the best matching overload based on:
    1. Exact parameter type matches
    2. Number of parameters (arity)

## Async Functions

Use `async` for asynchronous functions:

```desi
async def fetch_data(url: str) -> str:
    let response = await http_get(url)
    return response.body

async def main():
    let data = await fetch_data("https://example.com")
    print(data)
```

### Await

Use `await` to wait for an async function to complete:

```desi
async def add_async(a: int, b: int) -> int:
    return a + b

async def main():
    let result = await add_async(10, 20)
    print(result)  # 30
```

### Concurrent Execution

```desi
async def fetch_user(uid: int) -> str:
    await sleep(20)
    return f"user-{uid}"

async def fetch_all() -> list[str]:
    # Run concurrently
    let users = await gather(
        fetch_user(1),
        fetch_user(2),
        fetch_user(3)
    )
    return users
```

!!! info "Async Status"
    Async/await is partially implemented. Check the roadmap for current status.

## Defer

Execute code when the current scope exits:

```desi
def process_file(path: str) -> none:
    let file = open(path, "r")
    defer file.close()  # Always runs when function exits
    
    let content = file.read()
    process(content)
    # file.close() is automatically called here
```

Multiple `defer` statements execute in reverse order (LIFO):

```desi
def demo() -> none:
    defer print("first")
    defer print("second")
    defer print("third")
    # Prints: "third", "second", "first"
```

## Recursion

```desi
def factorial(n: int) -> int:
    if n <= 1:
        return 1
    return n * factorial(n - 1)

def fibonacci(n: int) -> int:
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)
```

!!! warning "Stack Limits"
    Deep recursion may cause stack overflow. Use iteration for very deep calculations.

## Generic Functions

Functions can be generic over types:

```desi
def identity<T>(x: T) -> T:
    return x

def main():
    print(identity(42))      # Works with int
    print(identity("hello")) # Works with str
    print(identity(true))    # Works with bool
```

See [Generics](generics.md) for more details.

## Visibility

### Public Functions

Use `pub` to make functions accessible from other modules:

```desi
# In math_utils.desi
pub def add(a: int, b: int) -> int:
    return a + b

def internal_helper() -> int:  # Private, not exported
    return 42
```

### Module-Level

```desi
# In main.desi
from math_utils import add

def main():
    print(add(1, 2))  # Uses imported function
```

## Entry Point

Every executable Desi program needs a `main` function. It can return either `int` (exit code) or `none`:

=== "Without Return (Default)"
    ```desi
    def main():
        print("Hello, Desi!")
        # Implicitly returns none
    ```

=== "With Exit Code"
    ```desi
    def main() -> int:
        print("Hello, Desi!")
        return 0  # Exit code
    ```

=== "Expression Return"
    ```desi
    def main() -> int:
        0  # Last expression as return
    ```

!!! tip "Which to use?"
    - Use `main()` (no return) for most programs - simpler and cleaner
    - Use `main() -> int` when you need to return an exit code to the shell

## Best Practices

### ✅ Do

- **Always type parameters**: `def foo(x: int) -> int`
- **Use descriptive names**: `calculate_total()` not `calc()`
- **Keep functions focused**: One function, one purpose
- **Document public APIs**: Add docstrings for public functions
- **Use `inout` sparingly**: Prefer returning new values

### ❌ Don't

- **Avoid very long functions**: Break into smaller pieces
- **Don't use global state**: Pass data as parameters
- **Avoid deep recursion**: Use iteration when possible
- **Don't overload excessively**: Keep overloads distinguishable

## Common Patterns

### Builder Pattern

```desi
def create_user(name: str, age: int, email: str) -> User:
    return User(name=name, age=age, email=email)
```

### Factory Functions

```desi
def from_string(s: str) -> Option<int>:
    if len(s) == 0:
        return Option.Nothing
    return Option.Some(int(s))
```

### Higher-Order Functions

```desi
def apply_twice(f: (int) -> int, x: int) -> int:
    return f(f(x))

def main():
    let double: int = lambda<int> x: int: x * 2
    print(apply_twice(double, 3))  # 12
```

## See Also

- [Lambda Functions](lambdas.md) - Anonymous functions
- [Generics](generics.md) - Generic function types
- [Classes](classes.md) - Methods in classes
- [Error Handling](error-handling.md) - Returning Option/Result
