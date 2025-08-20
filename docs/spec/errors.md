# Errors (Stage-0)

Desi uses **sum-type errors** (`Result[T,E]`) and **early return** with the `?` operator. There are **no exceptions** or stack unwinding in Stage-0. Absence is represented by `Option[T]`.

---

## Result & Option
- `Result[T,E] = Ok(T) | Err(E)`
- `Option[T]   = Some(T) | None`

### Propagation with `?`
`expr?` expects `expr : Result[T,E]`.
- If `expr` is `Ok(v)`, it yields `v`.
- If `expr` is `Err(e)`, it **returns early** from the current function with `Err(e)`.

```desi
def read_all(path: str) -> Result[str, IoError]:
  let fh  = fs.open(path)?          # fh : File
  let txt = fh.read_to_end()?       # txt : str
  Ok(txt)
```

### Option lifting

Inside a function returning `Result[T,E]`, using `?` on an `Option[T]` will return `Err(E)` chosen by the surrounding API (Stage-0 allows a helper to convert; std will provide `ok_or` style helpers later). For Stage-0 samples we recommend explicit mapping:

```desi
def head[T](xs: Vec[T]) -> Result[T, Empty]:
  match xs.len():
    0 => Err(Empty)
    _ => Ok(xs[0])
```

---

## `panic`

Use `panic(msg: str)` only for **bugs** (assertion failures, unreachable states). Stage-0 behavior:

* Prints a diagnostic to stderr.
* Aborts the process (no unwinding).
* In later stages, supervised tasks may isolate panics.

```desi
def idx(xs: Vec[i32], i: u32) -> i32:
  if i >= xs.len(): panic("index out of bounds")
  xs[i]
```

---

## Defer interaction

`defer` thunks run **on all exits**: normal return, `?` early return, and end-of-block.

```desi
def use() -> Result[Unit, IoError]:
  let fh = fs.open("file")?
  defer fh.close()
  do_stuff(fh)?         # if this returns Err, fh.close() still runs
  Ok(())
```

---

## Error type design (Stage-0 guidance)

* Keep error enums small and domain-focused.
* Prefer **opaque** error enums at API boundaries; map from lower-level errors during propagation.
* Provide human-readable formatting via `fmt` (added later in std).

```desi
enum IoError:
  NotFound
  Permission
  Unexpected

def open(path: str) -> Result[File, IoError]:
  # platform-specific mapping performed in runtime
```

---

## Lowering rules (for the C backend)

* `?` lowers to an `if` that returns `Err(e)` immediately.
* `defer` thunks are emitted before each return site created by `?`.
* No exceptions, no setjmp/longjmp.
