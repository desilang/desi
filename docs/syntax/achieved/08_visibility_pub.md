
# 08 · Visibility (`pub`)

`pub` enables cross-module use via from-import or unqualified references.
Rules demonstrated by examples:

* `pub def` is visible to importers.
* `pub let` **must** be immutable and a **compile-time literal** (no expressions).
* `pub let mut` is **not allowed**.
* Unqualified cross-module access to non-`pub` should error.

```desi
# examples/util/consts.desi
pub let CONST_HEIGHT: int = 180
let LOCAL_WIDTH: int = 75

pub def area(w: int, h: int) -> int:
  w * h
```

