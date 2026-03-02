# JSON Module

The `json` module provides full JSON parsing, building, and serialization.

## Import

```desi
import json
```

## Parsing JSON

```desi
let data = json.parse('{"name": "Alice", "age": 30, "active": true}')

let name = json.get_string(json.object_get(data, "name"))   # "Alice"
let age = json.get_int(json.object_get(data, "age"))         # 30
let active = json.get_bool(json.object_get(data, "active"))  # true
```

## Building JSON

Create JSON objects and arrays programmatically:

```desi
let user = json.new_object()
json.set(user, "name", json.new_string("Bob"))
json.set(user, "score", json.new_number(95.5))
json.set(user, "admin", json.new_bool(false))

let tags = json.new_array()
json.push(tags, json.new_string("staff"))
json.push(tags, json.new_string("writer"))
json.set(user, "tags", tags)

print(json.stringify(user))
# {"name":"Bob","score":95.5,"admin":false,"tags":["staff","writer"]}
```

## Modifying JSON

```desi
let obj = json.parse('{"x": 1, "y": 2}')

# Update existing key
json.set(obj, "x", json.new_number(10.0))

# Remove key
json.remove(obj, "y")

# Add new key
json.set(obj, "z", json.new_number(3.0))

print(json.stringify(obj))  # {"x":10,"z":3}
```

## Iterating Object Keys

```desi
let obj = json.parse('{"a": 1, "b": 2, "c": 3}')
let n = json.object_len(obj)
let i = 0
while i < n:
    let key = json.object_key(obj, i)
    let val = json.object_get(obj, key)
    print(key + " = " + str(json.get_int(val)))
    i = i + 1
```

Or get all keys at once:

```desi
let all_keys = json.keys(obj)  # JSON array of key strings
let count = json.array_len(all_keys)
```

## Working with Arrays

```desi
let arr = json.parse('[1, 2, 3, "hello"]')

let length = json.array_len(arr)              # 4
let first = json.get_int(json.array_get(arr, 0))  # 1
let last = json.get_string(json.array_get(arr, 3)) # "hello"
```

## Type Checking

```desi
let data = json.parse('{"count": 42, "name": "test", "items": [1,2]}')

let count_node = json.object_get(data, "count")
if json.is_number(count_node):
    if json.is_int(count_node):
        print("integer: " + str(json.get_int(count_node)))
    else:
        print("float: " + str(json.get_float(count_node)))

json.is_null(node)     # true if null
json.is_bool(node)     # true if boolean
json.is_number(node)   # true if number (int or float)
json.is_string(node)   # true if string
json.is_array(node)    # true if array
json.is_object(node)   # true if object
json.is_int(node)      # true if number with no decimal part
```

## Stringify

Convert any JSON node back to a string:

```desi
let obj = json.new_object()
json.set(obj, "greeting", json.new_string("hello"))
let text = json.stringify(obj)   # '{"greeting":"hello"}'
```

## HTTP Integration

Parse HTTP request bodies as JSON in server handlers:

```desi
import http
import json

def api_handler(req: Any) -> Any:
    let body = http.req_json(req)   # parse POST body as JSON
    let name = json.get_string(json.object_get(body, "name"))
    return http.json(200, {"received": name})
```

## API Reference

### Parse & Stringify

| Function | Description | Returns |
|----------|-------------|---------|
| `json.parse(text)` | Parse JSON string | JSON node |
| `json.stringify(node)` | Convert to JSON string | `str` |

### Type Checks

| Function | Returns |
|----------|---------|
| `json.is_null(node)` | `bool` |
| `json.is_bool(node)` | `bool` |
| `json.is_number(node)` | `bool` |
| `json.is_string(node)` | `bool` |
| `json.is_array(node)` | `bool` |
| `json.is_object(node)` | `bool` |
| `json.is_int(node)` | `bool` |
| `json.get_type(node)` | `int` (type constant) |

### Value Accessors

| Function | Returns |
|----------|---------|
| `json.get_bool(node)` | `bool` |
| `json.get_int(node)` | `int` |
| `json.get_float(node)` | `float` |
| `json.get_number(node)` | `float` |
| `json.get_string(node)` | `str` |

### Array Operations

| Function | Returns |
|----------|---------|
| `json.array_len(arr)` | `int` |
| `json.array_get(arr, index)` | JSON node |

### Object Operations

| Function | Returns |
|----------|---------|
| `json.object_len(obj)` | `int` |
| `json.object_get(obj, key)` | JSON node |
| `json.object_key(obj, index)` | `str` |
| `json.keys(obj)` | JSON array of key strings |

### Builders

| Function | Returns |
|----------|---------|
| `json.new_object()` | Empty JSON object |
| `json.new_array()` | Empty JSON array |
| `json.new_string(s)` | JSON string node |
| `json.new_number(n)` | JSON number node |
| `json.new_bool(b)` | JSON bool node |
| `json.new_null()` | JSON null node |

### Mutation

| Function | Description |
|----------|-------------|
| `json.set(obj, key, val)` | Add or update key on object |
| `json.push(arr, val)` | Append value to array |
| `json.remove(obj, key)` | Remove key from object |

### Type Constants

```desi
json.JSON_NULL   = 0
json.JSON_BOOL   = 1
json.JSON_NUMBER = 2
json.JSON_STRING = 3
json.JSON_ARRAY  = 4
json.JSON_OBJECT = 5
```
