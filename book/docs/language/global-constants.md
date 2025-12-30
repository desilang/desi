# Global Constants

Global constants let you define values at the module level that can be accessed anywhere in your code.

## Basic Syntax

```desi
# Define globals at the top of your file
let MAX_USERS = 100
let APP_NAME = "MyApp"
let PI = 3.14159
let DEBUG = true

def main() -> int:
    print(f"App: {APP_NAME}")
    print(f"Max users: {MAX_USERS}")
    return 0
```

## Naming Convention

Global constants should be `UPPER_CASE`:

```desi
# ✓ Good - UPPER_CASE
let MAX_SIZE = 1024
let API_KEY = "secret"

# ✗ Avoid - will show warning
let myConst = 42  # Warning: should be UPPER_CASE
```

## Immutability

Globals are always immutable. You cannot use `mut`:

```desi
# ✓ Correct
let VERSION = "1.0.0"

# ✗ Error - globals cannot be mutable
let mut counter = 0  # Error: global must be immutable
```

## Visibility and Exports

Only `pub` globals can be imported by other modules:

```desi
# config.desi
pub let MAX_RETRIES = 3    # Can be imported
let INTERNAL_KEY = "abc"   # Private to this module
```

```desi
# main.desi
from config import MAX_RETRIES  # Works!
from config import INTERNAL_KEY # Error: not public
```

## Supported Types

### Primitives

```desi
let INT_VAL = 42
let NEG_VAL = -100
let FLOAT_VAL = 3.14
let BOOL_VAL = true
let STR_VAL = "Hello"
```

### Explicit Types

```desi
let U8_VAL: u8 = 255
let I64_VAL: i64 = 9223372036854775807
let F32_VAL: f32 = 1.5
```

## Using in F-Strings

Globals work seamlessly in f-strings:

```desi
let APP_NAME = "Desi"
let VERSION = "0.1.0"

def main() -> int:
    print(f"{APP_NAME} v{VERSION}")  # Output: Desi v0.1.0
    return 0
```

## Best Practices

| Do | Don't |
|----|-------|
| Use for configuration | Use for mutable state |
| Keep values simple | Define complex objects |
| Use `pub` explicitly | Assume visibility |
| Use `UPPER_CASE` | Use `camelCase` or `snake_case` |

## Current Limitations

1. **No struct/enum instances**: `let ORIGIN = Point{...}` not yet supported
2. **Pub recommended**: Non-pub globals may have issues in some contexts
