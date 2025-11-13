# Desi — Big Syntax Tour

> This tour mixes **implemented** features (through **M13**) with a few **preview** snippets for later milestones (see `docs/roadmap.md`).
> For what’s *actually* implemented today, `docs/syntax.md` is the source of truth.
>
> **Note on f-strings:** `f"..."` is lexed and typed as `str`. Interpolation (`{expr}` holes) is a **future** M14 feature; in the current compiler they behave like plain strings.

## New here: explicit `for x in ...` loops + comprehensions implemented.

```desi
from task import sleep_ms as sleep, block_on, join, with_timeout, gather
from string import trim, to_upper as upcase, starts_with, ends_with

pub type Seconds = int
type Name = str

pub let APP_NAME: str = "Desi Syntax Tour"
let banner: str = "Pythonic surface • Rusty safety • Elixir-y async"

# NOTE: default parameters are planned (M14+). Today this would be written
#       as `def hello(name: str) -> str:` and called with an explicit argument.
def hello(name: str) -> str:
  """Named-arg example (defaults planned in M14)."""
  f"hello, {name}"
```

## Collections (literals + types)

### Canonical types: list[T], dict[K,V], set[T], tuple[T1,T2,...]

### Sugar (type positions only): [T] ≡ list[T], {K:V} ≡ dict[K,V]

```desi
let nums: [int] = [1, 2, 3, 4]
let kv: {str: int} = {"a": 1, "b": 2}
let tags: set[str] = #{"a", "b", "c"}
let pair: tuple[int, str] = (42, "life")

def list_push(inout xs: list[int], x: int) -> none:  # prelude-native
def dict_set(inout m: dict[str, int], k: str, v: int) -> none:  # prelude-native
def set_add(inout s: set[str], x: str) -> none:  # prelude-native

def collections_demo() -> none:
  let mut xs: [int] = [1, 2]
  list_push(xs, 3)
  dict_set(kv, "z", 99)
  set_add(tags, "z")
  print(xs, kv["a"], "has_z?", "z" in tags)
```

## Pipelines, Lambdas, Comprehensions

```desi
def pipeline_lambda_demo() -> none:
  let evens_times_10 = [1,2,3,4]
    |> filter(x => x % 2 == 0)
    |> map(x => x * 10)
  print(evens_times_10)  # [20, 40]
```

## ---- async lambda (preview; implemented lowering lives in M8) ----

### Uses `async lambda` in a pipeline and awaits all tasks via `gather`.

### Reuses fetch_user from the async section below.

```desi
async def fetch_all_users(uids: [int]) -> [str]:
  await gather(uids |> map(async lambda uid: await fetch_user(uid)))
```

```desi
def comprehensions_demo() -> none:
  let xs: [int] = [1,2,3,4,5]

  let squares_of_evens: [int] = [ x*x for x in xs if x % 2 == 0 ]
  print("squares_of_evens:", squares_of_evens)   # [4,16]

  let pairs: list[tuple[str,int]] =
    [ (k, v) for k in ["a","b","c"] for v in [1,2] if v != 2 ]
  let d: {str:int} = { k: v for (k, v) in pairs if v > 0 }
  print("dict:", d)  # {"a":1, "b":1, "c":1}

  let s: set[int] = #{ x % 3 for x in xs }
  print("set:", s)   # #{0,1,2}

  let coords: list[tuple[int,int]] =
    [ (x,y) for x in [1,2,3] for y in [1,2,3] if x != y ]
  print("coords:", coords)
```

## Explicit for-in loops (with destructuring)

```desi
def for_loops_demo() -> none:
  # Iterate list
  for x in [1,2,3]:
    print("x:", x)

  # Iterate dict as pairs
  for (k, v) in {"a": 1, "b": 2}:
    print(f"{k}={v}")

  # Iterate set
  for item in #{"u","v"}:
    print(item)

  # You can use one-line form too (simple statement only):
  for n in [10,20,30]: print(n)
```

## Overloading (exact arity & types)

```desi
def show(x: int) -> str: f"int:{x}"
def show(x: bool) -> str: f"bool:{x}"
def show(x: f64) -> str: f"f64:{x}"
def show(x: str, y: int) -> str: f"str+int:{x}:{y}"

def overloading_demo() -> none:
  print(show(3))
  print(show(true))
  print(show(2.5))
  print(show("a", 1))
```

## Immutability, one-liners, chained compares, f-strings

```desi
def immutability_demo() -> none:
  let x = 10
  let mut y = 10
  y += 1
  if y > x: print(f"y({y}) > x({x})")
  print(1 <= y <= 100)
```

> **Note:** Today, f-strings are just `str` literals at type-time; `{y}`/`{x}` are future M14-style interpolation holes.

## Multi-return, pipelines, destructuring in let

```desi
# Multi-return signature surface is still evolving; treat this as "shape",
# not a locked-in syntax.
def pair_xy(): int, int -> 3, 7
def half(x: int) -> int: x / 2

def multi_return_demo() -> none:
  let [a, b] = pair_xy()
  print(a, b, pair_xy() |> half())
```

## match (guards, values, enums)

```desi
enum Number:
  One: none
  Many: int

def tier(x: int) -> str:
  match x:
    x < 0: "neg"
    0:     "zero"
    x < 10:"small"
    _:     "big"

def show_num(n: Number) -> str:
  match n:
    One():                 "ONE"
    Many(k) and k < 10:    "few"
    Many(k):               "lots"
    _:                     "unknown"
```

> `match` is implemented with type checks on arm results; richer pattern semantics (esp. for enums) evolve across milestones.

## Async, future, join/timeout, select

```desi
async def fetch_user(uid: int) -> str: await sleep(20); f"user-{uid}"
async def fetch_settings(uid: int) -> str: await sleep(30); "dark"

async def greet(uid: int) -> str:
  let u, s = await join(fetch_user(uid), fetch_settings(uid))
  f"Hello {u} (theme={s |> upcase()})"

async def maybe_greet(uid: int, ms: int) -> ResultStr:
  let r = await with_timeout(ms, greet(uid))
  match r:
    ResultStr.Ok(msg): msg
    ResultStr.Err(_):  "timeout"

async def race_two(a: future[int], b: future[str]) -> str:
  select:
    await a: f"int={a}"
    await b: f"str={b}"
    _:       "none"
```

## Defer + using (RAII over **close**)

```desi
struct Buffer: data: str
def open_buf(s: str) -> Buffer: Buffer{ data: s }
def clear(inout b: Buffer) -> none: b.data := ""

def using_demo() -> bool:
  let buf = open_buf("hi")
  defer clear(buf)
  len(buf.data) > 0
```

## Classes — implicit self, dunders pub, nested visibility

```desi
class Counter:
  """Top-level classes are public by default (cannot be private)."""
  pub value: int

  pub def __new__(start: int) -> Counter:
    Counter{ value: start }

  pub def inc() -> none:             # implicit self
    self.value += 1

  pub def __repr__() -> str:
    f"Counter({self.value})"

  class Meta:           # private by default
    pub version: str
  pub class Info:       # explicitly exported nested class
    pub label: str

def class_demo() -> none:
  let c = Counter(5); c.inc(); print(c)
  let m = Counter.Meta{version: "0.1"}
  let i = Counter.Info{label: "visible"}
```

## Decorators (functions, async functions, classes)

```desi
def trace(fn):
  # NOTE: simple shape that parses today (no varargs surface yet).
  def wrapped(x: int, y: int) -> int:
    print("call:", fn)
    fn(x, y)
  wrapped

def cache(size: int):
  def attach(fn): attach_impl(fn, size)
  attach

@trace
def add(x: int, y: int) -> int: x + y

# NOTE: default parameter `y=1` is a future (M14+) feature; today keep it as `y: int`.
@cache(128)
async def slow_add(x: int, y: int) -> int:
  await sleep(60); x + y

def dataclass(cls): synthesize_dunders_in_place(cls); cls

@dataclass
class User:
  pub id: int
  pub name: str
```

## Lambdas in APIs, defaults & named args

```desi
def apply2(f: (int) -> int, x: int) -> int: f(f(x))

def lambda_demo() -> none:
  print(apply2(x => x + 3, 10))
  # Named arguments (M13): order is free as long as names match.
  print(add(y=2, x=3))
```

> **Named args (M13):** after the first named argument, all following args must be named. Defaults belong to M14+.

## FFI — A) Manifest-driven import

```desi
from sqlite3 import db_open, db_exec, db_errmsg

def exec_or_log(db, sql: str) -> bool:
  match db_exec(db, sql):
    Result.Ok(_): true
    Result.Err(e):
      print(f"sql failed: {sql} — {e}")
      false

def demo_sqlite_cleaner() -> none:
  match db_open(":memory:"):
    Result.Ok(db):
      using db:
        if not exec_or_log(db, "CREATE TABLE t(x)"): return
        if not exec_or_log(db, "INSERT INTO t VALUES(42)"): return
    Result.Err(e):
      print("sqlite open failed:", db_errmsg(e))
```

## FFI — B) Source-level externs via decorators

```desi
@extern("C", link="m") def sin(x: f64) -> f64
@extern("C", link="m") def cos(x: f64) -> f64
def unit_circle(x: f64) -> f64: sin(x)*sin(x) + cos(x)*cos(x)

@extern("C", link="crypto")  def SHA256(data: cptr[u8], n: usize, out: cptr[u8]) -> cptr[u8]
@extern("C", link="crypto")  def ERR_get_error() -> u64
@extern("C", link="crypto")  def ERR_error_string(code: u64, buf: cptr[u8]) -> cptr[u8]

def openssl_last_error() -> str:
  unsafe:
    let code = ERR_get_error()
    if code == 0: "OK"
    else:
      let buf = make_cbuf(256)
      from_cstr(ERR_error_string(code, buf.ptr))

def sha256_hex(s: str) -> Result[str, CError]:
  let bytes = as_bytes(s)
  let mut out: list[u8] = zeroed(32)
  unsafe:
    let p = SHA256(as_ptr(bytes), len(bytes), as_mut_ptr(out))
  if p == null: Result.Err(CError("openssl", openssl_last_error()))
  else:         Result.Ok(hex(out))
```

## Putting it together — main

```desi
pub def main(argc: int, argv: str) -> int:
  print(APP_NAME, banner)
  # Today: `hello("world")` + named variant; defaults land in M14.
  print(hello("world"), hello(name="desi"))

  collections_demo()
  pipeline_lambda_demo()
  comprehensions_demo()
  for_loops_demo()
  overloading_demo()
  immutability_demo()
  multi_return_demo()

  print(tier(10), show_num(Number.Many(3)))
  print("greet:", block_on(greet(123)))
  print("maybe:", block_on(maybe_greet(7, 5)))
  print("unit_circle(1.0)~", unit_circle(1.0))

  class_demo()
  print(User{ id: 1, name: "John Doe" })

  demo_sqlite_cleaner()

  match sha256_hex("hello"):
    Result.Ok(h): print("sha256:", h)
    Result.Err(e): print("openssl err:", e)

  0
```
