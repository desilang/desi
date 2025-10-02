# Imports

This document describes Desi’s Stage-1 import forms, name resolution, visibility checks, and common diagnostics.

## Overview

There are two import mechanisms:

1. **Module alias** (qualify uses with `alias.`):

```desi
import util.math as m
let n = m.add(2, 3)
```

2. **From-import** (bind public symbols directly into the local scope):

```desi
from util.math import add, PI as TauOverTwo
let n = add(2, 3)
```

Both forms can be used in the same file, but **a single alias name cannot be used for both** (see “Conflicts” below).

---

## Syntax

See the grammar for full details:

```ebnf
ImportDecl      = "import" DottedIdent ["as" Ident] ;
FromImportDecl  = "from" DottedIdent "import" ImportList ;
ImportList      = ImportItem {"," ImportItem} ;
ImportItem      = Ident ["as" Ident] ;
```

* `DottedIdent` is a dot-separated module path, e.g. `std.io`, `util.math`.
* `as Ident` provides a local alias for the module or symbol.

### Implicit module alias

If `"as ident"` is omitted in `import X.Y.Z`, the **implicit alias** is the last path segment (`Z`).

---

## What you can import

From-imports may list any **public** symbol exported by the target module:

* `pub def` functions
* `pub type` aliases
* `pub struct` types
* `pub enum` types
* `pub let` constants (must be compile-time literals in Stage-1)

Module aliases expose the module’s namespace for **qualified** access:

* `alias.Func(…)` — call a **public** function
* `alias.CONST` — read a **public** constant
* `alias.TypeName` — **not a value**; types cannot be used as expressions
* `alias.Something` that isn’t found → error

---

## Visibility rules

Desi enforces “public across files” in Stage-1.

### From-import must be public

Each item in a `from … import` list must be public in the target module. If not, the checker emits:

* **DTE0010** `symbol is not public` (span on that import item)

### Module-qualified uses must be public

When using `alias.name`:

* Calling a non-public function → **DTE0010**
* Accessing a non-public constant → **DTE0010**
* Selecting a type/struct/enum as a value is an error (types are not values)

### Unqualified cross-module calls

Calling a function by bare name is only allowed if:

* The function is **defined in the current (entry) file**, or
* It was **from-imported** (with or without `as`)

Otherwise, the checker reports **DTE0010** with context **“unqualified cross-module use”**.

---

## Name resolution & shadowing

Resolution order inside expressions:

1. **Locals** (let-bound names, parameters) — these win over everything.
2. **From-import bindings** — direct names/aliases introduced by `from`.
3. **Module aliases** — only via `alias.symbol` (the bare alias is **not** a value).

Implications:

* A local variable can shadow a module alias:

```desi
import util.math as m
def f() -> void:
  let m = 42      # ok: local wins
  print(m)        # prints 42; not the module
```

* Using a bare module alias as a value is an error:

```
def f() -> void:
  print(m)  # error: module alias 'm' is not a value; use m.<symbol>
```

* Referencing a from-imported function **as a value** (without calling) is also an error in expression position; you must call it:

```
from util.math import add
let x = add   # error: function is not a value; call it as add(...)
```

---

## Alias rules

Some names are **reserved** and cannot be used as module or from-import aliases:

* **Keywords** (e.g., `package`, `import`, `def`, `struct`, `enum`, `type`, `let`, `mut`, `if`, `else`, …)
* **Builtins & shims** used by the language/runtime: `print`, `io`, `fs`, `os`, `mem`, `str`

Using a reserved alias produces a parser/check error (plain error text).

### Duplicate alias checks

* Duplicating an alias within the same `from` line is rejected (`from M import x as a, y as a`).
* Reusing the same alias across separate `from` lines is rejected.
* Reusing a module alias is rejected.
* **Cross-conflict** (same name as both a module alias and a from-import alias) is rejected:

```desi
import util.math as m
from util.more import thing as m   # error: alias 'm' already used for a module
```

---

## Using imported symbols

### Module alias

```desi
import util.math as m

def f() -> void:
  let s = m.add(1, 2)   # call public function
  print(m.PI)           # read public constant
  m.TypeName            # error: types are not values
```

### From-import

```desi
from util.math import add, PI as P

def f() -> void:
  let s = add(1, 2)
  print(P)
```

---

## Diagnostics quick reference

* **DTE0010** `symbol is not public`

  * From-import of non-public symbol
  * Module-qualified call/const access of non-public symbol
  * Unqualified cross-module call to a non-local, non-imported symbol

* Plain errors (no fixed code; message varies)

  * Invalid/duplicate alias names
  * Module alias used as a value (`"module alias 'm' is not a value; use m.<symbol>"`)
  * Selecting a function/type as a value through a module alias without calling (`"is a function; call it with arguments"`, `"is a type; cannot be used as a value"`)

> The loader/linker may also produce module-domain diagnostics (e.g., `missing_module`, `import_cycle`) if/when you wire the module resolver into Stage-1.

---

## Gotchas & tips

* Prefer **from-import** when you want ergonomic, unqualified calls and are OK with importing only **public** API.
* Prefer a **module alias** when you want to call or read many public items behind a stable prefix (namespacing).
* If you need both styles, choose distinct aliases to avoid conflicts.
* Unqualified calls to library functions require a **from-import**; otherwise you’ll get **DTE0010**.

