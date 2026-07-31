# Global Constants

Define values at the module level that can be accessed anywhere in your code.

## Basic Syntax

```desi
let MAX_USERS = 100
let APP_NAME = "MyApp"
let PI = 3.14159
let DEBUG = true

def main() -> int:
    print(f"App: {APP_NAME}")
    print(f"Max: {MAX_USERS}")
    return 0
```

## Naming Convention

Use `UPPER_CASE` for globals:

```desi
# ✓ Good
let MAX_SIZE = 1024
let API_KEY = "secret"

# ✗ Warning
let myConst = 42  # Warning: should be UPPER_CASE
```

Underscore prefixes are also valid:
```desi
let _INTERNAL_VALUE = 99  # No warning - underscore prefix allowed
```

## Immutability

Globals are always immutable:

```desi
# ✓ Correct
let VERSION = "1.0.0"

# ✗ Error
let mut counter = 0  # Error: global must be immutable
```

## Visibility

Only `pub` globals can be imported:

```desi
# config.desi
pub let MAX_RETRIES = 3   # Exported
let INTERNAL_KEY = "abc"  # Private
let _SECRET = "hidden"    # Private (underscore convention)
```

```desi
# main.desi
from config import MAX_RETRIES  # Works
from config import INTERNAL_KEY # Error: not public
```

Either import form reaches an exported constant:

```desi
from config import MAX_RETRIES
print(str(MAX_RETRIES))
```

```desi
import config
print(str(config.MAX_RETRIES))
```

## Supported Types

```desi
# Integers
let INT_VAL = 42
let NEG_VAL = -100

# Floats
let FLOAT_VAL = 3.14

# Booleans
let BOOL_VAL = true

# Strings
let STR_VAL = "Hello"
```

## Computed Values

An initializer may be arithmetic over numeric literals, or `+` over string
literals. It is folded at compile time, so the constant is usable from other
modules like any other:

```desi
let SECONDS_PER_DAY = 60 * 60 * 24   # 86400
let TIMEOUT_MS: int = 30 * 1000      # 30000
let AREA = (2 + 3) * 10              # 50
let RATIO = 1.5 * 2.0                # 3.0
let BANNER = "desi " + "0.1.0"       # "desi 0.1.0"
```

Anything that has to run to produce its value — a function call, a
constructor — is not a constant expression:

```desi
let LIMIT = compute_limit()   # Not a constant
```

!!! warning "Call the function where you use it"
    A global initialized by a function call is only initialized when the module
    that declares it runs. Importing such a constant does not run it, and the
    link fails with an undefined symbol. Put the call in a function and call it
    from `main`, or write the value out as a literal.

## Using in F-Strings

```desi
let APP_NAME = "Desi"
pub let VERSION = "0.1.0"
let _BUILD = 123

def main() -> int:
    print(f"{APP_NAME} v{VERSION} (build {_BUILD})")
    return 0
```

## Best Practices

| Do | Don't |
|----|-------|
| Use for configuration | Use for mutable state |
| Keep values simple | Define complex objects |
| Use `pub` for exported values | Assume visibility |
| Use `UPPER_CASE` or `_UPPER_CASE` | Use `camelCase` |
