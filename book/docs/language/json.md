# JSON

Parse and stringify JSON data with the `json` module.

## Import

```desi
import json
```

## Parsing JSON

### json.parse()

Parse a JSON string into a `JsonValue`:

```desi
import json

let data = json.parse("42")
let text = json.parse("\"hello\"")
let flag = json.parse("true")
let empty = json.parse("null")
```

The return type is `JsonValue`, an enum with variants for each JSON type.

## JsonValue Type

```desi
pub enum JsonValue:
    Null: none
    Bool: bool
    Number: float
    String: str
```

### Pattern Matching

Use `match` to extract values:

```desi
import json

let data = json.parse("42")

match data:
    JsonValue.Number(n): print("Number:", n)
    JsonValue.String(s): print("String:", s)
    JsonValue.Bool(b): print("Bool:", b)
    JsonValue.Null: print("null")
```

### Using `is` Operator

Check the variant with `is`:

```desi
import json

let data = json.parse("true")

if data is JsonValue.Bool(b):
    print("Boolean value:", b)
```

## Examples

### Parse Different Types

```desi
import json

def main() -> int:
    # Numbers
    let num = json.parse("3.14")
    
    # Strings
    let str_val = json.parse("\"hello world\"")
    
    # Booleans
    let flag = json.parse("true")
    
    # Null
    let nothing = json.parse("null")
    
    print("Parsed successfully")
    0
```

## Supported JSON Types

| JSON | Desi Variant | Example |
|------|--------------|---------|
| `null` | `JsonValue.Null` | `json.parse("null")` |
| `true`/`false` | `JsonValue.Bool` | `json.parse("true")` |
| `123`, `3.14` | `JsonValue.Number` | `json.parse("42")` |
| `"text"` | `JsonValue.String` | `json.parse("\"hi\"")` |

## Coming Soon

- `json.stringify()` - Convert `JsonValue` back to JSON string
- Array and Object support
- `json.get()` - Access nested values
