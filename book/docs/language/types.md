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
    let d: i64 = 9223372036854775807   # -2^63 to 2^63-1
    
    # Unsigned
    let e: u8 = 255              # 0 to 255
    let f: u16 = 65535           # 0 to 65,535
    let g: u32 = 4294967295      # 0 to 2^32-1
    let h: u64 = 18446744073709551615  # 0 to 2^64-1
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
let greeting: str = "Hello, Desi"
let multiline: str = """
    Multi-line
    string literal
"""

# String concatenation
let full: str = "Hello" + " " + "World"

# F-strings (interpolation)
let msg: str = f"Hello, {name}!"
```

### `none`

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
# Type syntax — dict[K, V] and dict<K, V> are both accepted
let ages: dict[str, int] = {"Alice": 30, "Bob": 25}
let scores: dict<str, int> = {"Alice": 100, "Bob": 95}

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

## Type Conversion with `as`

Desi uses the `as` keyword for explicit type conversions between numeric types:

### Basic Casts

```desi
# Integer size conversions
let x: int = 1000
let y: i32 = x as i32          # int → i32 (may truncate)
let z: i8 = 42 as i8           # literal → i8

# Float ↔ Integer
let pi: float = 3.14159
let pi_int: int = pi as int    # 3 (truncates toward zero)
let n_float: float = 42 as float  # int → float

# Float size conversions
let f: f32 = 3.14 as f32       # double → float
let d: f64 = 1.5 as f64        # float → double
```

### Required for Mixed-Width Operations

```desi
let a: i32 = 100
let b: int = 50

# ❌ Error: mismatched numeric widths
# let sum = a + b

# ✅ Correct: cast to same type
let sum = (a as int) + b       # Cast i32 → int
let sum2 = a + (b as i32)      # Cast int → i32
```

!!! tip "When to Cast"
    - **Always required** when mixing sized types (`i8`, `i32`, `i64`, etc.)
    - **Always required** for float ↔ int conversion
    - Use `as` instead of constructor functions like `int(x)` for numeric casts

### What `as` Does Internally

| Cast Type | LLVM Operation | Notes |
|-----------|---------------|-------|
| Large int → Small int | `trunc` | May lose bits |
| Small int → Large int | `sext` | Sign-extends |
| Float → Int | `fptosi` | Truncates toward zero |
| Int → Float | `sitofp` | May lose precision |
| Double → Float | `fptrunc` | May lose precision |
| Float → Double | `fpext` | No precision loss |

## Decimal Type

The `decimal` type provides arbitrary-precision decimal arithmetic, ideal for financial calculations where floating-point errors are unacceptable:

```desi
# Create decimals from strings
let price: decimal = decimal("19.99")
let tax: decimal = decimal("0.0825")

# Precise arithmetic
let total: decimal = price + (price * tax)  # Exact: 21.6391175

# No floating-point errors!
let a: decimal = decimal("0.1")
let b: decimal = decimal("0.2")
let sum: decimal = a + b  # Exactly 0.3, not 0.30000000000000004
```

### When to Use `decimal`

✅ **Use for:**
- Currency and financial calculations
- Scientific measurements requiring exact precision
- Any calculation where `0.1 + 0.2 == 0.3` must be true

❌ **Don't use for:**
- Performance-critical calculations (slower than `float`)
- Graphics or game development
- General-purpose math where approximation is acceptable

### Decimal Operations

```desi
let a: decimal = decimal("100.50")
let b: decimal = decimal("20.25")

# Arithmetic
let sum: decimal = a + b      # 120.75
let diff: decimal = a - b     # 80.25
let prod: decimal = a * b     # 2035.125
let quot: decimal = a / b     # 4.962962962962962962962962963

# Comparison
if a > b:
    print("a is larger")
```

!!! warning "Memory Managed"
    Decimal values are heap-allocated. The compiler automatically manages their memory through scope-based deallocation.

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

`match` is an expression, so each arm is a single expression — `return` cannot
appear inside an arm. Return the match itself:

```desi
def process(value: Option<int>) -> int:
    return match value:
        Option.Some(n): n * 2
        Option.Nothing: 0
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
