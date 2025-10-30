# M5 — Imports & Resolver (Phases 1–3)

**Scope:** Compiler behavior (not a user tutorial). Phases 1–2 established the import surface, loader, resolver Exports table (functions only), and checker bridges including unused-import lints. **Phase-3** adds **module-qualified calls**: `import math; math.add(2,3)` resolves using the same exported signatures available to `from math import add`.

---

## Syntax (surface)

```desi
import math
import io as I

from math import add, hypot as hyp
```

* `import dotted.name [as alias]` binds the **leaf** name (or `alias`) in the importing module’s local scope.
* `from dotted.name import a [as x], b, …` binds `a` (or `x`) directly as a local callable.

### Package rule

A directory is a package **iff** it contains `__mod.desi`. Files at the leaf (e.g., `util/math.desi`) are valid standalone modules but packages require `__mod.desi`.

---

## Phase-3 — Module-qualified calls

**Goal:** Allow `import math; math.add(…)` to type-check using `math`’s **exported** function signatures.

### Behavior

* If the callee is a field expression `module.name` and `module` is a local **import binding**, look up `name` in that module’s exported function surface (collected by the resolver in Phase-2).
* Overloads are resolved exactly like normal calls.
* If `name` is **not exported** (missing, private, or untyped), emit **`DME0003 module.bad_import`** at the field name span with message:
  `"<module> has no exported '<name>'"`.

### Lints

* Using a module **as a qualifier** counts as a use of that import, so `DMW0004` (unused import) is **not** emitted for `math` in `math.add(...)`.
* Unused `from … import …` entries still trigger `DMW0005`.

---

## Visibility policy (enforced)

* **Functions:** only `pub def` with fully annotated parameters and return type are exported cross-module.
* **Classes/types/consts:** not exported/validated across modules yet (future milestone).

---

## CLI notes

* Exit codes: `0` ok, `1` had diagnostics, `2` arg/I/O error.
* `-v` prints normalized entry + active `-I` roots.
* Multi-root search unchanged.

---

## Example

```desi
import math

def main() -> int:
	print(math.add(2, 3))
	return 0
```

If `math` does not export `add`, the checker emits:

```
DME0003: math has no exported 'add'
```

---

## Future work (post M5)

* Re-exports and `*`
* Relative imports
* Cross-module exports for classes/types/consts
* Qualified member resolution on types

