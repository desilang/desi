
# 07 · Enums & Match

Declare sum types; use qualified constructors; pattern-match variants.

```desi
enum Result:
  Ok: int
  Err: str

def mk_ok() -> Result: Result.Ok(200)

def main() -> int:
  let r: Result = mk_ok()
  match r:
    Ok(x):
      print("ok:", x)
    Err(e):
      print("err:", e)
  0
```

Zero-payload variants use `none`:

```desi
enum MaybeStr:
  Some: str
  None: none
```

Patterns:

* `Ok(v)` — binds payload
* `None()` — zero-payload
* `_` — wildcard
