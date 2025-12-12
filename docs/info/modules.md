# Module System in Desi - Complete Guide

**Table of Contents:**
1. [Quick Start](#quick-start)
2. [Import Syntax](#import-syntax)
3. [Module Resolution](#module-resolution)
4. [Exporting Functions](#exporting-functions)
5. [Package Structure](#package-structure)
6. [Re-exporting](#re-exporting)
7. [Best Practices](#best-practices)

---

## Quick Start

```desi
# Import a module and use qualified calls
import math
print(math.add(2, 3))

# Import specific functions
from math import add, sub
print(add(2, 3))

# Import with alias
from math import add as plus
print(plus(2, 3))

# Import everything
from math import *
print(add(2, 3))
print(sub(5, 2))
```

---

## Import Syntax

### Module Import

Import a module and access its exports via qualified names:

```desi
import math

def main():
    let x: int = math.add(2, 3)
    let y: int = math.sub(5, 2)
```

### From Import

Import specific items directly into the current namespace:

```desi
from math import add, sub

def main():
    print(add(2, 3))  # No qualifier needed
```

### Aliased Import

Rename imports to avoid conflicts or for convenience:

```desi
from math import add as plus
from other_math import add as other_add

def main():
    print(plus(2, 3))       # Uses math.add
    print(other_add(2, 3))  # Uses other_math.add
```

### Wildcard Import

Import all public exports from a module:

```desi
from math import *

def main():
    print(add(2, 3))  # All pub functions available
    print(sub(5, 2))
```

---

## Module Resolution

Desi automatically searches for modules in this order:

1. **Entry file's directory** - Same folder as your main file
2. **Standard library** - `compiler/lib/` directory
3. **DESI_PATH** - Paths in the `DESI_PATH` environment variable
4. **Explicit roots** - Paths specified with `-I` flag

### Example Project Structure

```
my_project/
├── main.desi           # from utils import helper
├── utils.desi          # pub def helper()
└── math_utils/
    ├── __mod.desi      # Package entry point
    └── trig.desi       # pub def sin(), cos()
```

### Dotted Paths

For nested modules, use dotted paths:

```desi
import math_utils.trig

def main():
    # Access via qualified name
    math_utils.trig.sin(1.0)
```

Or import directly:

```desi
from math_utils.trig import sin, cos
```

---

## Exporting Functions

Only `pub` (public) functions are visible to importers:

```desi
# math.desi

pub def add(x: int, y: int) -> int:
    return x + y

pub def sub(x: int, y: int) -> int:
    return x - y

# Private - not exported
def internal_helper() -> int:
    return 42
```

### Visibility Rules

| Declaration | Exported? |
|-------------|-----------|
| `pub def foo()` | ✅ Yes |
| `def foo()` | ❌ No |
| `pub struct Point` | ✅ Yes |
| `struct Point` | ❌ No |

---

## Package Structure

A package is a directory containing `__mod.desi`:

```
my_package/
├── __mod.desi     # Required - Package entry point
├── utils.desi     # Submodule
└── helpers.desi   # Submodule
```

### Package Entry Point (`__mod.desi`)

The `__mod.desi` file defines what the package exports:

```desi
# my_package/__mod.desi

# Re-export from submodules
from my_package.utils import helper
from my_package.helpers import *

# Define package-level functions
pub def package_func() -> int:
    return 42
```

---

## Re-exporting

Packages commonly re-export items from submodules:

```desi
# prelude/__mod.desi

# Simple re-export
from prelude.memory import zeroed

# Re-export multiple items
from prelude.numeric import to_u8, to_f32, to_f64

# Wildcard re-export entire submodule
from prelude.dict import *
from prelude.set import *
```

Consumers can then import from the package:

```desi
from prelude import zeroed, to_u8
```

---

## Best Practices

### ✅ DO

```desi
# DO: Use explicit imports for clarity
from math import add, sub

# DO: Use aliases to avoid conflicts
from module_a import func as func_a
from module_b import func as func_b

# DO: Use __mod.desi for package organization
```

### ❌ DON'T

```desi
# DON'T: Use wildcard imports in large projects
from everything import *  # Hard to track what's imported

# DON'T: Create circular imports
# a.desi imports b.desi, b.desi imports a.desi

# DON'T: Export internal helpers
pub def _internal_helper():  # Should be private
```

### Module Naming

- Use **lowercase** with **underscores** for module names
- Keep names short but descriptive
- Match directory names to import paths

---

## Diagnostics

| Code | Description |
|------|-------------|
| `DME0003` | Requested export not found in module |
| `DMW0004` | Unused import statement |
| `DMW0005` | Unused from-import item |

---

## Examples

See working examples:
- `examples/13_m5_imports_basic.desi` - Basic from-import
- `examples/13_m5_imports_qualified.desi` - Qualified calls
- `examples/13_m5_imports_wildcard.desi` - Wildcard import
- `examples/13_m5_imports_simple.desi` - Simplified syntax
