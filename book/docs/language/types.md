# Types

Desi has a static type system that combines the simplicity of Python's syntax with the safety of compile-time type checking.

## Primitive Types

### Integers

=== "`int`"
    ```desi
    let x: int = 42
    let hex: int = 0xDEAD
    let binary: int = 0b1011
    let octal: int = 0o755
    ```
    Default integer type. Maps to platform `int` (typically 32-bit).

=== "Sized Integers"
    ```desi
    # Signed
    let a: i8 = 127              # -128 to 127
    let b: i16 = 32767           # -32,768 to 32,767
    let c: i32 = 2147483647      # -2^31 to 2^31-1
    let d: i64 = 9223372036...   # -2^63 to 2^63-1
    
    # Unsigned
    let e: u8 = 255              # 0 to 255
    let f: u16 = 65535           # 0 to 65,535
    let g: u32 = 4294967295      # 0 to 2^32-1
    let h: u64 = 1844674407...   # 0 to 2^64-1
    ```
    
!!! tip "Integer Literals"
    - Decimal: `42`, `1000`
    - Hexadecimal: `0xDEAD`, `0xFF`
    - Binary: `0b1011`, `0b11111111`
    - Octal: `0o755`, `0o644`
    - Scientific: `1e9` (becomes float)

### Floating Point

=== "`float`"
    ```desi
    let pi: float = 3.14159
    let sci: float = 1.25e-3     # Scientific notation
    let leading: float = .5      # Leading decimal point
    let trailing: float = 2.     # Trailing decimal point
    ```
    Default floating-point type.

=== "Sized Floats"
    ```desi
    let x: f32 = 3.14            # 32-bit float
    let y: f64 = 2.71828         # 64-bit float (double)
    ```

### Boolean

```desi
let is_true: bool = true
let is_false: bool = false

# Boolean operations
let result: bool = true and false
let negated: bool = not true
```

### String

```desi
let name: str = "Desi"
let greeting: str = 'Hello'
let multiline: str = """
    Multi-line
    string literal
"""

# String concatenation
let full: str = "Hello" + " " + "World"

# F-strings (interpolation)
let msg: str = f"Hello, {name}!"
```

### None

```desi
let nothing: none = none

def side_effect() -> none:
    print("This function returns nothing")
```

The `none` type represents the absence of a value. Functions that don't return anything have return type `none`.

## Collection Types

### List

```desi
# Type syntax
let numbers: list[int] = [1, 2, 3, 4, 5]
let names: list[str] = ["Alice", "Bob", "Charlie"]

# Shorthand syntax (sugar)
let nums: [int] = [1, 2, 3]

# Empty list (requires type annotation)
let empty: list[int] = []

# Operations
numbers.append(6)
let first: int = numbers[0]
let length: int = len(numbers)
```

!!! note "Type Annotations Required"
    Empty collections require explicit type annotations since the compiler cannot infer the element type.

### Dictionary

```desi
# Type syntax
let ages: dict[str, int] = {"Alice": 30, "Bob": 25}

# Shorthand syntax (sugar)
let scores: {str: int} = {"Alice": 100, "Bob": 95}

# Empty dict
let empty: dict[str, int] = {}

# Operations
ages["Charlie"] = 35
let age: int = ages["Alice"]
let has_key: bool = "Alice" in ages
```

### Set

```desi
# Type syntax
let tags: set[str] = #{"python", "rust", "desi"}

# Empty set
let empty: set[int] = set()

# Operations
tags.add("go")
let has_python: bool = "python" in tags
let size: int = len(tags)
```

!!! warning "Set Literal Syntax"
    Use `#{...}` for set literals to distinguish from dict literals.  
    Empty sets must use `set()` constructor.

### Tuple

```desi
# Type syntax
let pair: tuple[int, str] = (42, "answer")
let triple: tuple[str, int, bool] = ("test", 1, true)

# Destructuring
let (x, y) = pair
print(x)  # 42
print(y)  # "answer"
```

Tuples are fixed-size, heterogeneous collections with compile-time known types.

## Type Aliases

Type aliases provide alternative names for existing types:

```desi
# Simple alias
type IntList = list[int]
type StringMap = dict[str, str]

# Usage
let numbers: IntList = [1, 2, 3]
let config: StringMap = {"key": "value"}
```

### Generic Type Aliases

```desi
# Generic aliases with type parameters
type Box<T> = Option<T>
type Pair<A, B> = tuple[A, B]
type Triple<X, Y, Z> = tuple[X, Y, Z]

# Usage
let boxed: Box<int> = Option.Some(42)
let pair: Pair<str, int> = ("answer", 42)
let triple: Triple<int, str, bool> = (1, "two", true)
```

## Type Annotations

### Variable Declarations

```desi
# Explicit type annotation
let x: int = 42
let name: str = "Desi"

# Type inference (annotation optional)
let y = 42        # Inferred as int
let msg = "Hello" # Inferred as str
```

### Function Signatures

```desi
def add(a: int, b: int) -> int:
    return a + b

def greet(name: str) -> str:
    return f"Hello, {name}!"

def no_return() -> none:
    print("Side effect only")
```

!!! tip "When to Use Type Annotations"
    - **Required**: Function parameters and return types
    - **Required**: Empty collections (`[]`, `{}`, `set()`)
    - **Optional**: Variable declarations (type can be inferred)
    - **Recommended**: Public APIs and complex expressions

## Type Inference

Desi infers types when possible:

=== "Basic Inference"
    ```desi
    let x = 42              # Inferred as int
    let name = "Desi"       # Inferred as str
    let pi = 3.14           # Inferred as float
    let flag = true         # Inferred as bool
    ```

=== "Collection Inference"
    ```desi
    let nums = [1, 2, 3]           # list[int]
    let strs = ["a", "b", "c"]     # list[str]
    let mixed = [1, 2, 3]          # list[int], not mixed types
    ```

=== "Function Inference"
    ```desi
    def double(x: int):  # Return type inferred as int
        return x * 2
    
    let result = double(21)  # result inferred as int
    ```

!!! warning "Limitations"
    Type inference cannot resolve:
    - Empty collections: `let x = []` ❌ (need `let x: list[int] = []`)
    - Ambiguous expressions without context
    - Generic type parameters without constraints

## Type Conversion

Desi requires explicit type conversions:

```desi
# int <-> float
let x: int = 42
let y: float = float(x)    # Explicit conversion
let z: int = int(3.14)     # Truncates to 3

# int <-> str
let num_str: str = str(42)      # "42"
let num: int = int("123")       # 123

# bool <-> int
let b: bool = true
let i: int = int(b)        # 1
```

!!! failure "No Implicit Conversions"
    ```desi
    # ❌ Error: type mismatch
    let x: int = 42
    let y: float = x
    ```
    
    ```desi
    # ✅ Correct
    let x: int = 42
    let y: float = float(x)
    ```

## Type Compatibility

### Assignability

```desi
# Same types are directly assignable
let x: int = 42
let y: int = x    # OK

# Subtyping with Option/Result
let maybe: Option<int> = Option.Some(42)
let result: Result<int, str> = Result.Ok(42)
```

### Generics

```desi
# Generic types must match exactly
let list_int: list[int] = [1, 2, 3]
let list_str: list[str] = ["a", "b", "c"]

# ❌ Cannot assign list[int] to list[str]
# let mixed: list[str] = list_int  # Error!
```

## Special Types

### Option<T>

Represents a value that may or may not exist:

```desi
let some: Option<int> = Option.Some(42)
let nothing: Option<int> = Option.Nothing

match some:
    Option.Some(value): print(f"Got {value}")
    Option.Nothing: print("No value")
```

See [Error Handling](error-handling.md) for details.

### Result<T, E>

Represents either success or failure:

```desi
let success: Result<int, str> = Result.Ok(42)
let failure: Result<int, str> = Result.Err("failed")

match success:
    Result.Ok(value): print(f"Success: {value}")
    Result.Err(error): print(f"Error: {error}")
```

See [Error Handling](error-handling.md) for details.

## Common Patterns

### Multiple Return Values

Use tuples for multiple return values:

```desi
def divide(a: int, b: int) -> tuple[int, int]:
    return (a / b, a % b)  # (quotient, remainder)

let (quot, rem) = divide(10, 3)
print(quot)  # 3
print(rem)   # 1
```

### Type Guards with Match

```desi
def process(value: Option<int>) -> int:
    match value:
        Option.Some(n): return n * 2
        Option.Nothing: return 0
```

### Collection Initialization

```desi
# Pre-sized list (future feature)
let numbers = [0] * 10  # [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]

# Dict with defaults
let defaults: dict[str, int] = {"a": 0, "b": 0, "c": 0}

# Set from list
let unique: set[int] = set([1, 2, 2, 3, 3, 3])  # {1, 2, 3}
```

## Type System Design

### Strengths

- **Static Checking** - Catch type errors at compile time
- **Type Inference** - Less verbose than Java/C++, safer than Python
- **No Implicit Conversions** - Explicit is better than implicit
- **Generic Types** - Code reuse without sacrificing type safety

### Philosophy

> "Explicit where it matters, inferred where it's obvious"

Desi requires type annotations for:
- Function boundaries (clear contracts)
- Ambiguous situations (empty collections)

But allows inference for:
- Local variables (obvious from RHS)
- Simple expressions (clear types)

## See Also

- [Functions](functions.md) - Function type signatures
- [Generics](generics.md) - Generic programming with type parameters
- [Error Handling](error-handling.md) - Option and Result types
- [Classes](classes.md) - Custom types and methods
