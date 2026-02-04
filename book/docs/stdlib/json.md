# JSON Module

The `json` module provides functions for parsing and manipulating JSON data. It uses a hybrid Python+Rust approach: Python-style smart detection with Rust-style explicit accessors.

## Quick Start

```desi
import json

# Parse JSON
let data = json.parse('{"name": "desi", "version": 1}')

# Access values
let name = json.object_get(data, "name")
print(json.get_string(name))  # "desi"

let ver = json.object_get(data, "version")
print(json.get_int(ver))      # 1

# Convert back to JSON string
print(json.stringify(data))   # {"name":"desi","version":1}
```

## Parsing

### `json.parse(text: str) -> Any`

Parse a JSON string and return a JSON node.

```desi
let num = json.parse("42")
let arr = json.parse("[1, 2, 3]")
let str_lit = "[1, 2, 3]"
let obj = json.parse(str_lit)
```

> **Note:** When parsing JSON objects `{...}`, assign the JSON string to a variable first to avoid f-string interpolation issues.

### `json.stringify(node: Any) -> str`

Convert a JSON node back to a JSON string.

```desi
let data = json.parse("[1, 2, 3]")
print(json.stringify(data))  # [1,2,3]
```

## Type Checking

Check what type a JSON value is before extracting it:

| Function | Returns | Description |
|----------|---------|-------------|
| `is_null(n)` | `bool` | True if value is `null` |
| `is_bool(n)` | `bool` | True if value is a boolean |
| `is_number(n)` | `bool` | True if value is a number |
| `is_int(n)` | `bool` | True if number has no decimals |
| `is_string(n)` | `bool` | True if value is a string |
| `is_array(n)` | `bool` | True if value is an array |
| `is_object(n)` | `bool` | True if value is an object |

```desi
let val = json.parse("42")

if json.is_int(val):
    print("It's an integer!")
```

## Value Extraction

Extract typed values from JSON nodes:

| Function | Returns | Description |
|----------|---------|-------------|
| `get_bool(n)` | `bool` | Get boolean value |
| `get_int(n)` | `int` | Get number as integer |
| `get_float(n)` | `float` | Get number as float |
| `get_number(n)` | `float` | Alias for `get_float` |
| `get_string(n)` | `str` | Get string value |

```desi
let int_val = json.parse("42")
let float_val = json.parse("3.14")

# Smart integer detection
if json.is_int(int_val):
    print(json.get_int(int_val))     # 42 (clean integer output)
    
print(json.get_float(float_val))     # 3.14
```

### Python+Rust Hybrid Number Handling

Desi combines the best of both approaches:

- **Python-style**: `is_int()` detects if a number is whole (42 vs 42.5)
- **Rust-style**: Explicit `get_int()` / `get_float()` accessors

```desi
let n1 = json.parse("100")     # Whole number
let n2 = json.parse("3.14")    # Decimal

json.is_int(n1)   # true
json.is_int(n2)   # false

json.get_int(n1)  # 100 (proper integer)
json.get_float(n2) # 3.14
```

## Array Operations

| Function | Returns | Description |
|----------|---------|-------------|
| `array_len(arr)` | `int` | Get number of elements |
| `array_get(arr, index)` | `Any` | Get element at index |

```desi
let arr = json.parse("[10, 20, 30]")

print(json.array_len(arr))        # 3

let elem = json.array_get(arr, 1)
print(json.get_int(elem))         # 20
```

## Object Operations

| Function | Returns | Description |
|----------|---------|-------------|
| `object_len(obj)` | `int` | Get number of keys |
| `object_get(obj, key)` | `Any` | Get value by key |

```desi
let json_str = '{"name": "alice", "age": 30}'
let obj = json.parse(json_str)

print(json.object_len(obj))       # 2

let name = json.object_get(obj, "name")
print(json.get_string(name))      # alice
```

## Nested Data

Access nested objects and arrays by chaining operations:

```desi
let json_str = '{"users": [{"name": "alice"}, {"name": "bob"}]}'
let data = json.parse(json_str)

let users = json.object_get(data, "users")
let first = json.array_get(users, 0)
let name = json.object_get(first, "name")
print(json.get_string(name))  # alice
```

## Common Patterns

### Safe Value Access

Always check type before extracting:

```desi
let val = json.object_get(data, "maybe_missing")

if json.is_null(val):
    print("Value is null or missing")
elif json.is_string(val):
    print(json.get_string(val))
```

### Iterating Arrays

```desi
let arr = json.parse("[1, 2, 3, 4, 5]")
let len = json.array_len(arr)
let i = 0
while i < len:
    let elem = json.array_get(arr, i)
    print(json.get_int(elem))
    i = i + 1
```

## See Also

- [Error Handling](../language/error-handling.md) - For safe value access patterns
