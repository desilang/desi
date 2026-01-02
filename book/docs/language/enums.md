# Enums

Enums (enumerations) define types with a fixed set of variants. Each variant can optionally carry data.

## Defining Enums

```desi
enum Color:
    Red: none
    Green: none
    Blue: none

enum Shape:
    Circle: float      # radius
    Rectangle: (float, float)  # width, height
```

- **Unit variants** (`none`): No associated data
- **Payload variants**: Carry data of the specified type

## Creating Enum Values

Unit variants (no payload) can be used with or without parentheses:

```desi
# Both syntaxes work for unit variants
let c = Color.Red()
let c2 = Color.Red     # Parentheses optional for unit variants
```

Payload variants require parentheses with the value:

```desi
let shape = Shape.Circle(5.0)
```

## Pattern Matching

Use `match` to handle enum variants:

```desi
match shape:
    Shape.Circle(r): print("Circle with radius", r)
    Shape.Rectangle(w, h): print("Rectangle", w, "x", h)
```

### Wildcard Pattern

Use `_` to match remaining cases:

```desi
match color:
    Color.Red(): print("Stop!")
    _: print("Go")
```

## Public Enums

Use `pub` to export enums from modules:

```desi
pub enum Status:
    Pending: none
    Running: int
    Completed: str
```

## Using Enums as Function Parameters

Enums work as function parameters like any other type:

```desi
def describe_status(s: Status) -> str:
    match s:
        Status.Pending(): "waiting"
        Status.Running(progress): f"running at {progress}%"
        Status.Completed(msg): msg
```

## Nested Enums

Enums can contain other enum types:

```desi
enum Result:
    Ok: Status
    Error: str

let r = Result.Ok(Status.Completed("done"))
```
