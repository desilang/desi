# M5 — Imports & Resolver (Phase 1)

**Goal:** Introduce `import` / `from … import …` syntax, define how modules are found, and outline resolver behavior for Phase-1. This guide documents rules the compiler follows; it is not a user tutorial.

---

## Syntax (surface)

```desi
import std
import std.io
import util.math as m

from util import math
from util.math import add, sub as minus
```

* `import dotted.name [as alias]` binds the **leaf** name (or `alias`) in the importing module’s local scope.
* `from dotted.name import a [as x], b, ...` binds each item into the local scope.

No relative imports, no `*`, no re-exports in Phase-1.

---

## Module layout & `__mod.desi`

**`__mod.desi` is an optional package initializer** (analogous to Python’s `__init__.py`).

* A directory is a **package iff** it contains `__mod.desi`. If not present, that directory is not a package in Phase-1.
* Resolution for `import foo.bar` walks:

  1. `foo/__mod.desi` (must exist — `foo` is a package)
  2. then either `foo/bar/__mod.desi` (subpackage) **or** `foo/bar.desi` (leaf file module)

`from foo import x` allows:

* `x` as a name exported by `foo/__mod.desi`
* **or** direct submodule `foo/x/__mod.desi` or `foo/x.desi`

**Phase-1 visibility:** treat all names defined in `__mod.desi` as exportable. No `pub` enforcement across packages yet.

---

## Search roots

The resolver searches under a configured module root (CLI `-I <path>` when enabled). Dotted names are mapped to paths under that root using the rules above.

---

## Resolver model

1. **Loader** abstracts where modules come from:

  * FS loader: reads `.desi` files from disk
  * In-memory loader: test fixture map `name → source`

2. **Module graph** is built in DFS order. The resolver:

  * Detects **cycles** (e.g., `a ↔ b`)
  * Emits diags for unknown modules, duplicates, alias conflicts
  * Computes bindings for:

    * `import a.b [as x]` → local `x` (or `b` if no alias)
    * `from a.b import y [as z]` → local `z` (or `y`)
  * Records **unused import / unused from-item** for Phase-1 lints

3. **Checker bridge** injects these bindings into the top scope before type checking so imported names don’t trip unknown-identifier errors.

---

## Diagnostics (Phase-1, use existing IDs)

| Situation                      | Code                                       |
| ------------------------------ | ------------------------------------------ |
| Import cycle                   | `DME0001` (`module.import_cycle`)          |
| Module not found               | `DME0002` (`module.not_found`)             |
| Bad import (malformed/invalid) | `DME0003` (`module.bad_import`)            |
| Duplicate import               | `DMW0001` (`module.duplicate_import`)      |
| Import alias conflict          | `DMW0003` (`module.import_alias_conflict`) |
| Unused import                  | `DMW0004` (`module.unused_import`)         |
| Unused from-item               | `DMW0005` (`module.unused_from_item`)      |

**Type-side stays unchanged:** unknown identifiers are still `DTE0001`.

---

## Limits in Phase-1

* No relative imports (`.`/`..`)
* No re-exports (`from x import *`) or package-level visibility checks
* Subpackages must have `__mod.desi` to be considered a package
* Leaf file modules are allowed only as the **final** segment

---

## Suggested layouts

```
<root>/
  std/
    __mod.desi
    io/
      __mod.desi
  util/
    __mod.desi
    math.desi
```

Examples:

* `import util.math as m` → binds `m`
* `from util import math` → binds `math` (subpackage or item of util’s `__mod.desi`)
* `from util.math import add as plus` → binds `plus`
