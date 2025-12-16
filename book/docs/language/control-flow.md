# Control Flow

Desi provides familiar control flow constructs with Python-inspired syntax and powerful pattern matching.

## Conditional Statements

### if / elif / else

```desi
let x: int = 10

if x > 0:
    print("Positive")
elif x < 0:
    print("Negative")
else:
    print("Zero")
```

### Nested Conditions

```desi
if condition1:
    print("Outer true")
    if condition2:
        print("Inner true")
    else:
        print("Inner false")
else:
    print("Outer false")
```

### Condition Expressions

Conditions must evaluate to `bool`:

```desi
let is_valid: bool = x > 0 and x < 100

if is_valid:
    print("Valid range")
```

### Chained Comparisons

Python-style chained comparisons:

```desi
let y: int = 50

if 0 < y < 100:     # Same as: y > 0 and y < 100
    print("y is between 0 and 100")

if 1 <= y <= 100:   # Inclusive range
    print("y is in range 1-100")
```

## Loops

### while Loop

```desi
let mut count: int = 0

while count < 5:
    print(count)
    count := count + 1

print("Done")  # Prints: 0 1 2 3 4 Done
```

### for...in Loop

Iterate over collections:

=== "List"
    ```desi
    let items: list[int] = [1, 2, 3, 4, 5]
    
    for x in items:
        print(x)
    ```

=== "Dict (items)"
    ```desi
    let prices: dict[str, int] = {"apple": 100, "banana": 50}
    
    for key: str, value: int in prices.items():
        print(key)
        print(value)
    ```

=== "Set"
    ```desi
    let tags: set[str] = #{"python", "rust", "desi"}
    
    for tag in tags:
        print(tag)
    ```

=== "String"
    ```desi
    let name: str = "Desi"
    
    for char in name:
        print(char)  # D e s i
    ```

### enumerate()

Get index and value pairs:

```desi
let items: list[str] = ["apple", "banana", "cherry"]

for i: int, item: str in enumerate(items):
    print(i)     # 0, 1, 2
    print(item)  # apple, banana, cherry
```

### zip()

Iterate over multiple collections in parallel:

```desi
let names: list[str] = ["Alice", "Bob"]
let scores: list[int] = [95, 87]

for name: str, score: int in zip(names, scores):
    print(name)
    print(score)
```

### One-Line Form

For simple statements:

```desi
for n in [10, 20, 30]: print(n)
```

## Pattern Matching

The `match` expression provides powerful pattern matching, especially useful with enums.

### Basic Match

```desi
enum Color:
    Red: none
    Green: none
    Blue: none

def color_name(c: Color) -> str:
    match c:
        Color.Red(): "Red"
        Color.Green(): "Green"
        Color.Blue(): "Blue"
```

### Match with Bindings

Extract values from enum variants:

```desi
enum Result:
    Ok: value: int
    Err: message: str

def handle_result(r: Result) -> str:
    match r:
        Result.Ok(v): f"Success: {v}"
        Result.Err(msg): f"Error: {msg}"
```

### Wildcard Pattern

Use `_` to match any value:

```desi
enum Status:
    Idle: none
    Running: none
    Paused: none
    Stopped: none

def is_active(s: Status) -> bool:
    match s:
        Status.Running(): true
        Status.Paused(): true
        _: false  # Matches Idle and Stopped
```

### Match as Expression

Match expressions return a value:

```desi
let result: str = match status:
    Status.Running(): "active"
    Status.Idle(): "waiting"
    _: "other"
```

### Option Pattern

```desi
def process(value: Option<int>) -> int:
    match value:
        Option.Some(n): n * 2
        Option.Nothing: 0
```

### Result Pattern

```desi
def handle(result: Result<int, str>) -> int:
    match result:
        Result.Ok(value): value
        Result.Err(msg): 
            print(f"Error: {msg}")
            -1
```

### Multiple Arms

```desi
enum Day:
    Monday: none
    Tuesday: none
    Wednesday: none
    Thursday: none
    Friday: none
    Saturday: none
    Sunday: none

def is_weekend(day: Day) -> bool:
    match day:
        Day.Saturday(): true
        Day.Sunday(): true
        _: false
```

## Exhaustiveness Checking

The compiler ensures all enum variants are handled:

```desi
enum Color:
    Red: none
    Green: none
    Blue: none

# ❌ Error: non-exhaustive match
def color_code(c: Color) -> int:
    match c:
        Color.Red(): 1
        Color.Green(): 2
        # Missing Color.Blue()!
```

```desi
# ✅ Correct: all variants handled
def color_code(c: Color) -> int:
    match c:
        Color.Red(): 1
        Color.Green(): 2
        Color.Blue(): 3
```

Or use wildcard to catch remaining:

```desi
# ✅ Correct: wildcard catches Blue
def is_red(c: Color) -> bool:
    match c:
        Color.Red(): true
        _: false
```

## Boolean Operators

### Logical Operators

```desi
let a: bool = true
let b: bool = false

let and_result: bool = a and b   # false
let or_result: bool = a or b     # true
let not_result: bool = not a     # false
```

### Short-Circuit Evaluation

`and` and `or` use short-circuit evaluation:

```desi
# right side not evaluated if left is false
if false and expensive_check():
    ...

# right side not evaluated if left is true
if true or expensive_check():
    ...
```

## Comparison Operators

| Operator | Description |
|----------|-------------|
| `==` | Equal |
| `!=` | Not equal |
| `<` | Less than |
| `>` | Greater than |
| `<=` | Less than or equal |
| `>=` | Greater than or equal |
| `in` | Membership test |

### Membership Testing

```desi
let nums: list[int] = [1, 2, 3, 4, 5]

if 3 in nums:
    print("Found!")

let dict: dict[str, int] = {"a": 1, "b": 2}
if "a" in dict:
    print("Key exists!")
```

## Best Practices

### ✅ Do

- **Use match for enums**: Safer than if/elif chains
- **Prefer for...in over while when possible**: Clearer intent
- **Use enumerate() for indexed iteration**: Avoids manual indexing
- **Handle all enum variants**: Use exhaustive matching

### ❌ Don't

- **Don't use while(true) without break**: Prefer bounded loops
- **Avoid deeply nested conditions**: Extract to functions
- **Don't ignore match warnings**: Compiler catches missing cases

## Common Patterns

### Guard Clauses

```desi
def process(value: Option<int>) -> int:
    if value == Option.Nothing:
        return 0  # Early return
    
    # Continue with valid value
    match value:
        Option.Some(n): n * 2
        _: 0  # Unreachable but satisfies compiler
```

### Loop with Index

```desi
let items: list[str] = ["a", "b", "c"]

for i: int, item: str in enumerate(items):
    print(f"Item {i}: {item}")
```

### Collecting Results

```desi
let numbers: list[int] = [1, 2, 3, 4, 5]
let mut doubled: list[int] = []

for n in numbers:
    doubled.append(n * 2)
```

### Early Exit Pattern

```desi
def find_first(items: list[int], target: int) -> Option<int>:
    for i: int, item: int in enumerate(items):
        if item == target:
            return Option.Some(i)
    return Option.Nothing
```

## See Also

- [Functions](functions.md) - Function definitions
- [Error Handling](error-handling.md) - Option and Result types
- [Types](types.md) - Boolean and comparison types
