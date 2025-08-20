# Types (Stage-0)

This document defines the **minimal type system** needed to implement the compiler and standard library seed.

---

## 1) Primitive types
- `bool` — `true` / `false`
- Signed integers: `i32`, `i64`
- Unsigned integers: `u32`, `u64`, `u8` (bytes)
- Floating point: `f64`
- `str` — immutable UTF-8 string (ARC-managed)

**Copy vs Move**
- **Copy**: `bool`, all integers, `f64`, `u8`
- **Move**: `str`, `Vec[T]`, slices `[]T`, and all user aggregates (`struct`, `enum`)

---

## 2) Composite types

### 2.1 Slices
- Syntax: `[]T`
- Semantics: non-owning “view” `(ptr,len)` into contiguous `T`
- Bounds-checked indexing

### 2.2 Vectors
- Syntax: `Vec[T]`
- Owning growable buffer `{data,len,cap}` (ARC-managed)
- Methods (Stage-0 minimum): `with_cap(n)`, `len()`, `push(v)`, `clear()`

### 2.3 Structs
```desi
struct Span:
  start: u32
  end: u32
````

* Default field immutability; mutation requires `let mut s = ...` then `s.field := ...`

### 2.4 Enums (ADTs)

```desi
enum Token:
  Ident(name: str)
  Int(value: i64)
  Plus
  EOF
```

* Tagged union with payloads
* Construct with variant name, e.g., `Ident("x")`

### 2.5 Option & Result

* `Option[T] = None | Some(T)`
* `Result[T,E] = Ok(T) | Err(E)`
* `?` operator propagates `Err` (or lifts `None` to an error in contexts returning `Result`)

---

## 3) Function types

* Syntax: `(A, B) -> R`
* First-class: pass/return functions, store in variables
* Closures capture by **move** of captured bindings

Example:

```desi
def apply[T](x: T, f: (T)->T) -> T:
  f(x)
```

---

## 4) Generics (monomorphization)

* Parametric types and functions use `[]`:

```desi
def head[T](xs: Vec[T]) -> Option[T]:
  if xs.len() == 0: None
  else: Some(xs[0])
```

* Compiled via **monomorphization**: each used `T` gets a concrete instantiation
* No trait/constraint system in Stage-0; only “used operations” on `T` are those provided by containers themselves

---

## 5) Type inference

* **Local inference** for `let`:

```desi
let n = 10          # n: i32
let s = "hi"        # s: str
```

* **Function parameters and returns** must be annotated
* Generic parameters must be explicit where inference is impossible:

```desi
let bs = Vec[u8].with_cap(64)
```

---

## 6) Conversions & casts

* No implicit numeric widening/narrowing
* Use explicit cast syntax:

```desi
let x: i64 = (i64)(some_i32)
```

(Cast rules are conservative in Stage-0 and may be restricted further.)

---

## 7) Strings

* `str` is immutable, UTF-8; concatenation produces a new `str`
* Slicing `str` yields `[]u8` in Stage-0 (text algorithms can live in std later)

---

## 8) Equality & ordering

* `==` / `!=` available for primitives and same-shape enums without payloads
* Ordering `< <= > >=` defined for numeric types only (Stage-0)

---

## 9) No null

* The language has no null value. Use `Option[T]` for absence.

---

## 10) FFI & layout (forward-looking note)

* Stage-0 reserves `repr(C)`/ABI decisions for Stage-1+.
* Struct/enum layout is defined but not yet exposed for FFI.
