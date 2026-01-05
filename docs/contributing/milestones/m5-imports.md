# M5 — Imports & Resolver (Phases 1–3)

**Scope:** Compiler behavior (not a user tutorial). Phases 1–2 established the import surface, loader, resolver Exports table (functions only), and checker integration (including unused-import lints). **Phase-3** adds **module-qualified calls**: `import math; math.add(2,3)` resolves using the same exported signatures available to `from math import add`.

---

## Syntax (surface)

```desi
import math
import io as I

from math import add, hypot as hyp
from net.http import get as http_get
```

* `import dotted.name [as alias]` binds the **leaf** name (or `alias`) in the importing module’s local scope.
* `from dotted.name import a [as x], b, …` binds each listed item directly into the local scope.

### Package rule

A directory is a package **iff** it contains `__mod.desi`. Files at the leaf (e.g., `util/math.desi`) are valid standalone modules, but packages require `__mod.desi`.

Resolution search respects **multi-roots** (e.g., `-I "examples:compiler/lib"`), probing each root in order:

```
import math      # -> <root>/math/__mod.desi
import io        # -> <root>/io.desi  or  io/__mod.desi
from time import now
```

---

## Phase-1 (recap): Loader, graph, and bindings

1. Loader abstraction (FS + in-memory for tests).
2. Module graph from entrypoint across `import` / `from … import …`.

* Detect **cycles**, unknown modules, duplicates, alias conflicts.

3. Bind locals:

* `import a.b.c` binds `c` (leaf) or the alias after `as`.
* `from a.b import x [as y]` binds `x`/`y` as locals.

---

## Phase-2 (recap): Exports & checker bridge

* Resolver builds `Exports` **per module** (functions only):

  * **Only** `pub def` with **fully typed** params + return are exported.
* Checker populates `Info.Funcs[...]` for `from … import f [as g]` using **exact** signatures from Exports.
* If `x` is missing/private/untyped:

  * **`DME0003 module.bad_import`** — “`<module> has no exported '<name>'`”.
* Lints:

  * **`DMW0004`**: unused `import` binding.
  * **`DMW0005`**: unused item in `from … import …`.

---

## Phase-3: Module-qualified calls

**Goal:** Allow `import math; math.add(…)` to type-check using the **exported** function signatures from `math` (same surface exposed to `from … import …`).

### Behavior

* If the callee is a field expression `module.name` and `module` is a **local import binding**, the checker looks up `name` in that module’s exported function set (from resolver Exports).
* Overloads are resolved **exactly** like normal calls.
* If `name` is **not exported** (missing, private, or untyped), emit:

  * **`DME0003 module.bad_import`** at the field **name** span:
    `"<module> has no exported '<name>'"`.

### Lints stay correct

Using the qualifier **counts as usage** of the import, so:

```desi
import math
math.add(1,2)           # does NOT trigger DMW0004
```

Unused `from` items still trigger **`DMW0005`**.

---

## Examples

```desi
import math

def main() -> int:
  print(math.add(2, 3))     # OK if math exports: pub def add(int,int)->int
  return 0
```

```desi
import math

def main() -> int:
  print(math.sqrt(9))       # DME0003 if sqrt is not exported
  return 0
```

```desi
import math as M

def main() -> int:
  print(M.add(2, 3))        # alias works the same way
  return 0
```

---

## Diagnostics & CLI

* **Exit codes:** `0` ok, `1` had diagnostics, `2` arg/I/O error.
* `-I` multi-root search unchanged; `-v` prints normalized entry and active roots.
* Error limiting: the CLI caps diagnostics (nice UX for very error-y files), but **Phase-3 introduces no new codes**.

---

## Future work (post-M5)

* Re-exports and `from pkg import *`
* Relative imports
* Cross-module exports for classes/types/consts
* Qualified member resolution on types (post-typechecker)
