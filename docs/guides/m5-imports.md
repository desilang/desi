# M5 — Imports & Resolver (Phase 1)

**Goal:** Introduce `import` / `from … import …` syntax, define how modules are found, and document resolver behavior for Phase-1. This describes compiler rules, not a user tutorial.

---

## Syntax (surface)

```desi
import math
import io as I

from math import sqrt, hypot as hyp
```

* `import dotted.name [as alias]` binds the **leaf** name (or `alias`) in the importing module’s local scope.
* `from dotted.name import a [as x], b, ...` binds each listed item into the local scope.

No relative imports, no `*`, no re-exports in Phase-1.

---

## Module layout & `__mod.desi`

**`__mod.desi` is an optional package initializer** (analogous to Python’s `__init__.py`).

* A directory is a **package iff** it contains `__mod.desi`. If it’s missing, the directory is **not** a package in Phase-1.
* Resolution for `import a.b` walks:

  1. `a/__mod.desi` (must exist — `a` is a package)
  2. then either `a/b/__mod.desi` (subpackage) **or** `a/b.desi` (leaf file module)

`from a import x` allows:

* `x` as a name **exported by** `a/__mod.desi`, **or**
* direct submodule `a/x/__mod.desi` or `a/x.desi`.

**Phase-1 visibility:** treat all names defined in `__mod.desi` as exportable. No `pub` enforcement across packages yet.

---

## Search roots (stdless standard library)

The resolver searches **multiple roots in order**; the first match wins:

1. The **project root** (user code)
2. The **standard library root**: `compiler/lib/`

Dotted names are mapped to paths under a root using the rules above.
There is **no `std.` prefix**. Standard modules are imported as short names:

```desi
import math       # resolves to compiler/lib/math/__mod.desi
import io         # resolves to compiler/lib/io.desi or compiler/lib/io/__mod.desi
from time import now
```

---

## Resolver model

1. **Loader** abstracts where modules come from:

* FS loader: reads `.desi` files from disk using the **multi-root** search order.
* In-memory loader: test fixture map `path → source`.

2. **Module graph** is built (DFS) from the importing module:

* Detect **cycles** (`a ↔ b`, etc.).
* Emit diags for unknown modules, duplicates, alias conflicts.
* Compute bindings for:

  * `import a.b [as x]` → local `x` (or `b` if no alias)
  * `from a.b import y [as z]` → local `z` (or `y`)
* Track **unused import / unused from-item** for Phase-1 lints (usage marked by the checker).

3. **Checker bridge** injects these bindings into the top scope before type checking so imported names don’t cause unknown-identifier errors.

---

## Prelude builtins (no import)

Prelude identifiers (e.g., `print`, `str`) are injected into the top scope by the checker and are always available without import. (Implementation details and codegen/intrinsics are out of scope for M5.)

---

## CLI behavior (Phase-1)

* **Exit codes**

  * `0` — success (`ok`)
  * `1` — diagnostics were emitted (parse/resolve/type)
  * `2` — I/O or argument errors (printed as `check error: …`)

* **Verbose**
  `-v` prints the normalized entry path and the active `-I` roots.

* **Flag ordering**
  Subcommand form accepts flags in any order:
  `desic check -I ROOTS -v FILE`, `desic check FILE -I ROOTS`, or `-I ROOTS -check FILE`.

---

## Diagnostics (Phase-1, use existing IDs)

| Situation                 | Code                                       |
| ------------------------- | ------------------------------------------ |
| Import cycle              | `DME0001` (`module.import_cycle`)          |
| Module not found          | `DME0002` (`module.not_found`)             |
| Bad import (invalid item) | `DME0003` (`module.bad_import`)            |
| Duplicate import          | `DMW0001` (`module.duplicate_import`)      |
| Import alias conflict     | `DMW0003` (`module.import_alias_conflict`) |
| Unused import             | `DMW0004` (`module.unused_import`)         |
| Unused from-item          | `DMW0005` (`module.unused_from_item`)      |

**Type side remains unchanged:** unknown identifiers → `DTE0001`.

---

## Limits in Phase-1

* No relative imports (`.` / `..`)
* No re-exports (`from x import *`)
* No package-level visibility checks (`pub`) across packages
* Subpackages must have `__mod.desi` to be considered packages
* Leaf file modules are allowed only as the **final** segment

---

## Phase-1 callable aliases (note)

For `from a.b import foo as bar`, the checker treats `bar` as a callable name in Phase-1 even if the target module’s exact signature hasn’t been imported yet. This avoids spurious `undefined function` errors during early integration; exact cross-module signatures land in Phase-2.

---

## Suggested layouts (stdless)

```
<project-root>/
  app/
    __mod.desi
    main.desi

compiler/
  lib/
    math/
      __mod.desi
    io.desi
    time/
      __mod.desi
```

Examples:

* `import math` → binds `math` from `compiler/lib/math/__mod.desi`
* `import io as I` → binds `I` from `compiler/lib/io.desi` (or `io/__mod.desi`)
* `from math import sqrt as root` → binds `root`
* `from time import now` → binds `now`
