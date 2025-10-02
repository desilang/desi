
# 05 · Modules & Imports

Import whole modules, alias them, or import specific symbols.

```desi
import util.math
import util.math as m
from util.math import add
from util.math import add as addition
```

Using a module alias:

```desi
let a = m.add(1, 2)
```

**Notes**

* The standard library’s common facilities (e.g., `print`, `str.*`) are available **without imports**.
* A module alias is not a value (you can't `let x = m`).
* Duplicate imports are diagnosed.
* Cycles are diagnosed (e.g., `m9_cycle_a` ↔ `m9_cycle_b`).
