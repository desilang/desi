# Modules & Imports

Desi uses a file-based module system similar to Python.

## Import Syntax

```python
# Import entire module
import math

# Import specific items
from math import add, sub

# Import with alias
from math import add as plus
import some_module as m

# Import all (wildcard)
from math import *
```

## Export Visibility

By default, items are **private**. Use `pub` to export:

```python
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

## Module Structure

```
myproject/
├── main.desi          # Entry point
├── math.desi          # from math import add
└── utils/
    ├── __mod.desi     # Package marker
    └── helpers.desi   # from utils.helpers import foo
```

## Circular Imports

**Not allowed!** Desi detects and reports circular dependencies:

```
# foo.desi
from bar import greet   # ❌ Error: circular import

# bar.desi  
from foo import hello   # Creates a cycle
```

Error:
```
error[DME0008] module: circular import detected
  = help: Circular imports are not allowed. Refactor to break the dependency cycle.
```

**Solution**: Move shared code to a third module that both can import.
