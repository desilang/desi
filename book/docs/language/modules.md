# Modules & Imports

Desi uses a file-based module system similar to Python.

## Import Syntax

```desi
# Import entire module
import math

# Import specific items
from math import add, sub

# Import with alias
from math import add as plus
import some_module as m

# Import all (wildcard)
from math import *

# Relative import (current directory)
from .utils import helper

# Stdlib import (always uses stdlib, never local)
from std.math import add
```

## Export Visibility

By default, items are **private**. Use `pub` to export:

```desi
# Private - only visible in this file
def helper():
    pass

# Public - can be imported by other modules
pub def add(x: int, y: int) -> int:
    return x + y

# Classes at top-level are public by default
class Point:
    x: int
    y: int
```

## Public Re-exports

Use `pub from` to re-export items as part of your module's public API:

```desi
# mylib.desi - re-export print for users of mylib
pub from io import print
pub from math import add, sub

# Consumers can now do: from mylib import print, add
```

## Module Structure

```
myproject/
├── main.desi          # Entry point
├── math.desi          # from math import add
└── utils/
    ├── __mod.desi     # Package marker
    └── helpers.desi   # from utils.helpers import foo
```

## Import Errors

### Circular Imports (DME0008)
```
# foo.desi
from bar import greet   # ❌ Error

# bar.desi  
from foo import hello   # Creates a cycle
```

### Reserved Namespace (DME0009)
```desi
import std   # ❌ 'std' is reserved for stdlib
```

### Shadowing Stdlib (DME0010)
```desi
# If you have local math.desi AND stdlib has math:
from math import add   # ❌ Local shadows stdlib
from std.math import add  # ✓ Use this instead
```

### Ambiguous Module (DME0011)
```
# If BOTH exist:
# - foo.desi
# - foo/__mod.desi
from foo import bar   # ❌ Ambiguous, remove one
```

## Lazy Initialization

Imports are **lazy by default** — modules initialize only when first used:

```desi
import log
import math

def main() -> int:
    print("Starting...")
    log.info("Using log now")  # log module initializes here
    # math is never used, so it never initializes
    0
```

### Benefits

- **Performance** — unused imports have zero runtime cost
- **Circular imports** — modules can import each other safely
- **No boilerplate** — lazy behavior is automatic

### How It Works

Each module has an init guard:

1. First call to `log.info()` triggers `__ensure_log_init()`
2. The init thunk checks a flag and calls the module's `__top__()` if needed
3. Subsequent calls skip initialization

This happens transparently — you don't need to think about it!
