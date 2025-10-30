# M5 — Imports & Resolver (Phases 1–2)

**Scope:** Compiler behavior (not a user tutorial). Phase-1 introduced the syntax, loader, and graph. **Phase-2 adds real cross-module function signatures, `pub` export rules, and unused-import lints**—without changing CLI flags or adding new diagnostic IDs.

---

## Syntax (surface)

```desi
import math
import io as I

from math import add, hypot as hyp
```

* `import dotted.name [as alias]` binds the **leaf** name (or `alias`) in the importing module’s local scope.
* `from dotted.name import a [as x], b, ...` binds each listed item into the local scope.
* No relative imports, no `*`, no re-exports (yet).

---

## Module layout & `__mod.desi`

**`__mod.desi` is an optional package initializer** (analogous to Python’s `__init__.py`).

* A directory is a **package iff** it contains `__mod.desi`. If it’s missing, the directory is **not** a package.
* Resolution for `import a.b` walks:

  1. `a/__mod.desi` (must exist — `a` is a package)
  2. then either `a/b/__mod.desi` (subpackage) **or** `a/b.desi` (leaf file module)
* `from a import x` allows:

  * `x` exported by `a/__mod.desi`, **or**
  * direct submodule `a/x/__mod.desi` or `a/x.desi`.

**Leaf file modules** are allowed only for the **final** segment.

---

## Search roots (std-less standard library)

Multi-root search (first match wins):

1. Project root (user code)
2. Standard library root: `compiler/lib/`

Dotted names map under a root with the rules above. There is **no `std.` prefix**; std modules import directly:

```desi
import math        # -> compiler/lib/math/__mod.desi
import io          # -> compiler/lib/io.desi or io/__mod.desi
from time import now
```

---

## Phase-1 (recap)

**Resolver model**

1. Loader abstraction (FS + in-memory maps for tests).
2. Module graph built from the entry:

  * Detect **cycles**; diags for unknown modules, duplicates, alias conflicts.
  * Compute local bindings for `import …` and `from … import …`.
3. **Checker bridge** injects these bindings + prelude builtins into the top scope to avoid unknown-identifier errors.

**Prelude (no import)**
`print`, `str`, etc. are injected by the checker.

**CLI behavior**

* Exit codes: `0` ok, `1` had diagnostics, `2` arg/I/O error.
* `-v` prints normalized entry + active `-I` roots.
* Flags are order-independent.

**Diagnostics (Phase-1 IDs used)**

| Situation                 | Code                                       |
| ------------------------- | ------------------------------------------ |
| Import cycle              | `DME0001` (`module.import_cycle`)          |
| Module not found          | `DME0002` (`module.not_found`)             |
| Bad import (invalid item) | `DME0003` (`module.bad_import`)            |
| Duplicate import          | `DMW0001` (`module.duplicate_import`)      |
| Import alias conflict     | `DMW0003` (`module.import_alias_conflict`) |
| Unused import             | `DMW0004` (`module.unused_import`)         |
| Unused from-item          | `DMW0005` (`module.unused_from_item`)      |

**Limits**

* No relative imports.
* No `from x import *`.
* Subpackages require `__mod.desi`.
* Leaf modules only at final segment.

---

## Phase-2 (this update)

### 1) Real cross-module function signatures

The resolver now computes an **Exports** table per imported module (functions only), and the checker **consumes exact typed overloads** for `from … import …` aliases.

**Export rules (functions)**

* Only **`pub def`** are exported.
* Function must be **fully typed**: every parameter and the return type annotated.
* Top-level functions only (methods / nested functions are not exported).
* Overloads: multiple `pub def` with the same name are exported as an overload set.
* If a module defines both typed and untyped overloads, **only the typed ones** are exported.

**Checker behavior**

* For `from a.b import foo as bar`, `bar`’s overload set is filled with the **exact** exported signatures from `a.b`.
* When real signatures exist, the Phase-1 “permissive fallback” for calls is **not** used; calls resolve against those real candidates.
* If no exported typed signature exists for a requested item, see **DME0003** below.

**Example (stdlib stub)**

```desi
# compiler/lib/math/__mod.desi
pub def add(x: int, y: int) -> int:
  return x + y
```

```desi
# user module
from math import add as sum
sum(2, 3)   # checked as (int, int) -> int
```

### 2) Import validation (no new codes)

* `from a import x` where `x` is **not exported** by `a` → **`DME0003 module.bad_import`**, message:
  `a has no exported 'x'`.
* “Not exported” includes: missing name, **not `pub`**, or missing type annotations.

### 3) Unused import lints (now emitted)

* **`DMW0004`**: `import mod` bound but never referenced.
* **`DMW0005`**: `from mod import name` bound but never referenced.
* Using a module **as a qualifier** (e.g., `io.println(…)`) counts as usage of `io`.

---

## Visibility policy (what is enforced now)

* **Functions:** `pub def` required for cross-module export (enforced in Phase-2).
* **Classes/types/consts:** not exported/validated across modules in Phase-2.
  (Top-level classes default public; nested classes default private — enforcement across modules is planned for a later phase.)

---

## Migration tips

* If a previous Phase-1 `from a import f` now yields `DME0003`, make sure the provider declares:

  ```desi
  pub def f(...typed...) -> ReturnType: ...
  ```
* Update stdlib stubs to `pub def` with full types (e.g., `math.add`, `math.sub`).
* Keep using leaf file modules only at the final path segment; prefer `__mod.desi` for packages.

---

## Examples

**Good: typed + pub**

```desi
# a/__mod.desi
pub def area(r: float) -> float:
  return 3.14 * r * r
```

```desi
from a import area
area(2.0)  # ok
```

**Bad: private or untyped**

```desi
# a/__mod.desi
def helper(x: int) -> int:   # not pub
  return x

pub def bad(x):              # untyped param
  return x
```

```desi
from a import helper   # DME0003: a has no exported 'helper'
from a import bad      # DME0003: a has no exported 'bad'
```

**Unused**

```desi
import io              # DMW0004: unused import 'io'
from math import add   # DMW0005: unused imported name 'add'
```

---

## CLI (unchanged)

* Exit codes: `0/1/2` unchanged.
* `-v` still prints entry + roots.
* `-I` multi-root search unchanged.

---

## Future work (post M5)

* Re-exports and `*`
* Relative imports
* Cross-module exports for classes/types/consts
* Qualified member resolution with visibility on types

