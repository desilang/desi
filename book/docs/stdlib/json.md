# JSON Module

The `json` module provides functions for parsing and manipulating JSON data. It uses a hybrid Python+Rust approach: Python-style smart detection with Rust-style explicit accessors.

## Quick Start

```desi
import json

# Parse JSON
let data = json.parse("{\"name\": \"desi\", \"version\": 1}")

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
let json_str = "{\"name\": \"alice\", \"age\": 30}"
let obj = json.parse(json_str)

print(json.object_len(obj))       # 2

let name = json.object_get(obj, "name")
print(json.get_string(name))      # alice
```

## Nested Data

Access nested objects and arrays by chaining operations:

```desi
let json_str = "{\"users\": [{\"name\": \"alice\"}, {\"name\": \"bob\"}]}"
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
let count = json.array_len(arr)
let mut i = 0
while i < count:
    let elem = json.array_get(arr, i)
    print(json.get_int(elem))
    i := i + 1
```

## Serialization

### `json.dumps(node) -> str`

Serialize a JSON node to a compact string (alias for `stringify`).

```desi
let obj = json.new_object()
json.set(obj, "name", json.new_string("desi"))
json.set(obj, "version", json.new_number(1.0))
print(json.dumps(obj))  # {"name":"desi","version":1}
```

### `json.pretty(node, indent) -> str`

Pretty-print JSON with indentation. Like Python's `json.dumps(data, indent=2)`.

```desi
let json_str = "{\"name\": \"alice\", \"scores\": [100, 95, 87]}"
let data = json.parse(json_str)
print(json.pretty(data, 2))
# {
#   "name": "alice",
#   "scores": [
#     100,
#     95,
#     87
#   ]
# }
```

## Builder API

Build JSON objects and arrays programmatically:

| Function | Description |
|----------|-------------|
| `new_object()` | Create empty `{}` |
| `new_array()` | Create empty `[]` |
| `new_string(s)` | Create string node |
| `new_number(n)` | Create number node |
| `new_bool(b)` | Create boolean node |
| `new_null()` | Create null node |
| `set(obj, key, val)` | Set key-value on object |
| `push(arr, val)` | Append to array |
| `remove(obj, key)` | Remove key from object |
| `keys(obj)` | Get all keys as array |

```desi
import json

def main() -> int:
    let user = json.new_object()
    json.set(user, "name", json.new_string("alice"))
    json.set(user, "age", json.new_number(30.0))
    json.set(user, "admin", json.new_bool(true))
    
    let tags = json.new_array()
    json.push(tags, json.new_string("dev"))
    json.push(tags, json.new_string("ops"))
    json.set(user, "tags", tags)
    
    print(json.pretty(user, 2))
    0
```

## Advanced Operations

### `json.clone(node) -> Any`

Deep copy a JSON node. Like Python's `copy.deepcopy()`.

```desi
let original = json.parse("{\"x\": 1}")
let copy = json.clone(original)
json.set(copy, "y", json.new_number(2.0))
# original still has only "x"; copy has "x" and "y"
```

### `json.merge(base, overlay) -> Any`

Merge two JSON objects. Overlay's keys take precedence.
Like Python's `{**base, **overlay}` or JavaScript's `Object.assign()`.

```desi
let defaults = json.parse("{\"theme\": \"dark\", \"lang\": \"en\"}")
let user_cfg = json.parse("{\"lang\": \"hi\"}")
let merged = json.merge(defaults, user_cfg)
print(json.pretty(merged, 2))
# {"theme": "dark", "lang": "hi"}
```

### `json.equals(a, b) -> bool`

Deep equality check between two JSON nodes.

```desi
let a = json.parse("[1, 2, 3]")
let b = json.parse("[1, 2, 3]")
let c = json.parse("[1, 2, 4]")

print(json.equals(a, b))  # true
print(json.equals(a, c))  # false
```

### `json.has_key(obj, key) -> bool`

Check if a JSON object contains a key. Like Python's `"key" in dict`.

```desi
let config = json.parse("{\"debug\": true}")
if json.has_key(config, "debug"):
    print("Debug mode configured")
```

### `json.values(obj) -> Any`

Get all values of a JSON object as an array. Like Python's `dict.values()`.

```desi
let scores = json.parse("{\"math\": 95, \"science\": 87}")
let vals = json.values(scores)
print(json.array_len(vals))  # 2
```

## Comparison

| Desi | Python | Go | Rust |
|---|---|---|---|
| `json.parse(s)` | `json.loads(s)` | `json.Unmarshal()` | `serde_json::from_str()` |
| `json.dumps(n)` | `json.dumps(d)` | `json.Marshal()` | `serde_json::to_string()` |
| `json.pretty(n, 2)` | `json.dumps(d, indent=2)` | `json.MarshalIndent()` | `serde_json::to_string_pretty()` |
| `json.merge(a, b)` | `{**a, **b}` | Manual | Manual |
| `json.has_key(o, k)` | `k in d` | `_, ok := d[k]` | `d.get(k)` |

## See Also

- [Error Handling](../language/error-handling.md) - For safe value access patterns
