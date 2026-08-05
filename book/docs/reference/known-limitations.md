# Known Limitations

Everything on this page is a real limitation of Desi 0.1.0, confirmed by testing
rather than inferred from the source. Where there is a workaround it is given.

If you hit something that is not listed here, please
[open an issue](https://github.com/desilang/desi/issues) — an unlisted surprise
is a bug report we want.

---

## Platform and distribution

### No prebuilt binary for macOS on Intel

The prebuilt macOS binary is arm64 only — it runs on Apple Silicon and not on an
Intel Mac. Desi builds from source there, so `make` works; there is just no
package to download.

The release matrix used to build this on `macos-13`, the last Intel macOS runner
image, which GitHub has retired. The intended fix is a universal binary produced
from the arm64 image, which can target both architectures — that changes the
macOS build path and was not something to ship untested during the 0.1.0
release, so it is scheduled for 0.1.1.

### No prebuilt binary for Linux arm64

Desi builds from source on Linux arm64, but there is no prebuilt binary and no CI
coverage, because the release matrix has no runner for it. macOS arm64 is fully
covered. See the platform table in the README.

### Release binaries are not code-signed

Windows SmartScreen warns on first run — choose **More info → Run anyway**. macOS
Gatekeeper may quarantine the download:

```bash
xattr -d com.apple.quarantine desic
```

Building from source avoids both.

### `desic watch` is not available on Windows

Hot reload needs Unix signals (`SIGUSR1`) and shared-library reloading, neither of
which has a Windows equivalent in place. Running it prints a message and exits 1.
Use `desic run` on Windows, or use the watch workflow on Linux or macOS.

### The database drivers have no TLS on Windows

The PostgreSQL and MySQL drivers are built on Windows and connect over plain TCP,
but they are compiled without an OpenSSL include path, so their TLS support
compiles out. Connecting to a server that *requires* TLS will fail there.

This affects the database drivers only. HTTPS is unaffected on Windows — the HTTP
client and server use Schannel.

Workarounds: connect without TLS (a local or private-network server), or use
Linux or macOS where the OpenSSL path is compiled in. Teaching the drivers the
Schannel path that `compiler/runtime/http/tls_win.h` already implements is a
post-0.1.0 task.

---

## Language and compiler

### There is no `loop:` construct

`break` and `continue` both work in `for` and `while`, but there is no infinite
`loop:` block. Write `while true:`.

### A match arm is a single expression

`match` is an expression, so each arm is `pattern: expression` on one line.
There is no block body, and no `case` keyword before the pattern. `return`,
`break` and other statements cannot appear in an arm:

```desi
let area = match shape:
    Shape.Circle(r): 3.14159 * r * r
    Shape.Rect(w, h): w * h
```

To return the result, return the whole match:

```desi
def describe(v: Option<int>) -> str:
    return match v:
        Option.Some(n): "got " + str(n)
        Option.Nothing: "nothing"
```

When a branch needs several statements, call a function from the arm. A
multi-line `match` must be bound with `let` or returned — it cannot be passed
directly as a call argument.

### There is no line continuation

An expression has to fit on one line. A long method chain or pipeline cannot be
broken across lines; bind an intermediate result to a name instead:

```desi
let result = numbers.map(lambda<int> x: int: x * 2).filter(lambda<bool> x: int: x > 5)
```

### There is no `not in` operator

Write `not (x in y)`. The parentheses are required, because `not` binds tighter
than `in`.

### `range` is the only lazy sequence

`range` is a real value — bind it, pass it, index it, call `len()` on it, test
membership — and none of that materialises its elements. No other lazy sequence
exists yet: `map`, `filter` and comprehensions all build a list.

### `enumerate`, `reversed` and `zip` are loop syntax

Unlike `range`, these three are recognised by the for-loop desugaring and have
no value behind them. They are only valid as the iterable of a `for`:

```desi
for i, v in enumerate(items):
    print(str(i))
```

Binding one, or using it in a comprehension, is `DTE0132`. To reverse into a
list, iterate and append.

### Function types cannot be written in a signature

A parameter cannot be annotated with a function type:

```text
# Does not parse
def apply(handler: () -> int) -> int:
	return handler()
```

Use `Any` for the parameter, which is what the `http` and `signal` modules do:

```desi
def apply(handler: Any) -> int:
	return handler()
```

### Tuple type aliases cannot name their fields

```text
# Does not parse
type Point = (x: int, y: int)
```

Use a positional tuple, or a class or struct when the fields need names:

```desi
type Point = (int, int)
```

```desi
class Point:
	pub x: int
	pub y: int
```

### Generic constructors need turbofish

Type arguments at a constructor call must be written with `::`:

```text
let s = Stack::<int>()   # Works
let s = Stack<int>()     # Does not parse
```

Leaving the type arguments off entirely is also fine when they can be inferred
from the constructor's arguments:

```desi
let b = Box(5)
```

### A constant cannot be initialized by a function call

Constant initializers are folded at compile time. Arithmetic over numeric
literals and `+` over string literals are supported:

```desi
pub let SECONDS_PER_DAY = 60 * 60 * 24
pub let BANNER = "desi " + "0.1.0"
```

An initializer that has to *run* is not a constant expression:

```desi
pub let LIMIT = compute_limit()   # Not a constant
```

A module that only reads such a constant never runs the code that initializes
it, and the link fails with an undefined symbol. Put the call in a function and
call it from `main`, or write the value out as a literal. See
[Global Constants](../language/global-constants.md).

---

## Environment problems that look like Desi bugs

These are not defects in Desi, but they produce failures shaped exactly like
compiler or library bugs, so they are worth recognising.

### Ephemeral TCP port exhaustion

On a machine whose dynamic port range is exhausted, anything that opens many
short-lived sockets fails in ways that look like a bug in the networking or
database layer: row counts of `0`, return codes of `-1`, connections that seem
to succeed and then produce nothing.

The recognisable signature is a **blocking** `connect()` that succeeds instantly
while the non-blocking path reports `revents = POLLERR|POLLHUP|POLLWRNORM`.

Check the TIME_WAIT count before investigating anything network-shaped. On
Windows the default dynamic range is 49152–65535, so 16,384 ports:

```powershell
(Get-NetTCPConnection -State TimeWait).Count
```

A count in the tens of thousands means the machine, not your program. It also
takes down a WSL localhost relay, so both `127.0.0.1` and the WSL IP stop
working at the same time.

[Windows: dial failed on localhost](../guides/windows-networking.md) works
through a four-language comparison that isolates this to the machine.
