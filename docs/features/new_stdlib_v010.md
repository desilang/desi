# New Stdlib Modules (v0.1.0)

## csv Module
**Files:** `compiler/runtime/csv.c`, `compiler/lib/csv.desi`

RFC 4180 compliant CSV parsing/writing. C runtime handles the heavy lifting with proper quote escaping, the Desi wrapper provides a clean public API.

### Architecture
- `__csv_parse(data)` → Splits on newlines (respecting quoted fields), then splits each line on commas with `parse_csv_line()`
- `__csv_format(rows)` → Iterates rows, auto-quotes fields containing commas/quotes/newlines, escapes `"` as `""`
- File I/O: `__csv_read_file` reads entire file, passes to `__csv_parse`. `__csv_write_file` formats then writes.

### API
| Function | Signature |
|---|---|
| `csv.parse` | `(data: str) -> list[list[str]]` |
| `csv.parse_delim` | `(data: str, delim: str) -> list[list[str]]` |
| `csv.format` | `(rows: list[list[str]]) -> str` |
| `csv.read_file` | `(path: str) -> list[list[str]]` |
| `csv.write_file` | `(path: str, rows: list[list[str]]) -> int` |

---

## encoding Module
**Files:** `compiler/runtime/encoding.c`, `compiler/runtime/base64.c`, `compiler/lib/encoding.desi`

Unifies hex and base64 encoding under one import. Hex functions in `encoding.c`, base64 functions were pre-existing in `base64.c`.

### API
| Function | Signature |
|---|---|
| `encoding.hex_encode` | `(data: str) -> str` |
| `encoding.hex_decode` | `(hex: str) -> str` |
| `encoding.hex_encode_upper` | `(data: str) -> str` |
| `encoding.base64_encode` | `(data: str) -> str` |
| `encoding.base64_decode` | `(data: str) -> str` |
| `encoding.base64_url_encode` | `(data: str) -> str` |
| `encoding.base64_url_decode` | `(data: str) -> str` |

---

## template Module
**Files:** `compiler/runtime/template.c`, `compiler/lib/template.desi`

Simple `{{key}}` placeholder substitution engine. Uses parallel `keys`/`values` lists to avoid needing dict FFI at the C level.

### Architecture
- `__template_render(tmpl, keys, values)` — Scans for `{{...}}`, trims whitespace around key name, looks up in keys list, replaces with corresponding value. Unknown keys are preserved.
- `__template_render_pairs(tmpl, pairs)` — Splits a flat `[k1, v1, k2, v2, ...]` list into keys/values, then delegates to `__template_render`.
- `__template_escape_html(input)` — Replaces `&`, `<`, `>`, `"`, `'` with HTML entities.

### API
| Function | Signature |
|---|---|
| `template.render` | `(tmpl: str, keys: list[str], values: list[str]) -> str` |
| `template.render_pairs` | `(tmpl: str, pairs: list[str]) -> str` |
| `template.escape_html` | `(input: str) -> str` |

---

## Parameter Defaults Fix (M14)
**File:** `compiler/internal/lower/lower_call.go`

The parser and type checker already supported parameter defaults (`def foo(x: int, y: int = 10)`), but the lowerer was not injecting default values at call sites when fewer arguments were passed. Fixed by resolving the callee's `FuncDecl` via `ls.info.Funcs` at the fallthrough call path and lowering default expressions for omitted parameters.

---

## net Module
**Files:** `compiler/runtime/net.c`, `compiler/lib/net.desi`

Raw TCP/UDP sockets and DNS resolution. Sits below `http.c`/`websocket.c` — provides direct socket access for custom protocols.

### Architecture
- TCP client: `__net_dial` uses `getaddrinfo()` → `socket()` → `connect()` (IPv4/IPv6)
- TCP server: `__net_listen` binds with `SO_REUSEADDR`, backlog 128
- UDP: standard `sendto()`/`recvfrom()` with `SOCK_DGRAM`
- DNS: `__net_resolve` uses `getaddrinfo()` → `inet_ntop()`
- Cross-platform: POSIX on macOS/Linux, WinSock2 on Windows

### API
| Function | Signature |
|---|---|
| `net.dial` | `(host: str, port: int) -> int` |
| `net.send` | `(fd: int, data: str) -> int` |
| `net.recv` | `(fd: int, max_bytes: int) -> str` |
| `net.close` | `(fd: int) -> none` |
| `net.listen` | `(host: str, port: int) -> int` |
| `net.accept` | `(server_fd: int) -> int` |
| `net.peer_addr` | `(fd: int) -> str` |
| `net.udp_open` | `(host: str, port: int) -> int` |
| `net.udp_send` | `(fd: int, host: str, port: int, data: str) -> int` |
| `net.udp_recv` | `(fd: int, max_bytes: int) -> str` |
| `net.resolve` | `(hostname: str) -> str` |
| `net.set_nonblocking` | `(fd: int) -> int` |
| `net.set_timeout` | `(fd: int, timeout_ms: int) -> int` |

---

## assert_eq / assert_ne Builtins
**Files:** `compiler/runtime/builtins.c`, `compiler/internal/lower/lower_call.go`, `compiler/internal/check/info.go`

Value-comparison assertions with diff output. The lowerer recognizes `assert_eq`/`assert_ne` calls (like `assert`), determines the argument type from the checker, and dispatches to the appropriate typed C runtime function.

### Runtime Functions
- `__desi_assert_eq_int/str/bool(expected, actual, context)` — compare, show diff on mismatch
- `__desi_assert_ne_int/str/bool(a, b, context)` — compare, show value on match

### Failure Output
```
assertion failed: line 5: values should match
  expected: 42
    actual: 99
```

---

## datetime Module
**Files:** `compiler/runtime/datetime.c`, `compiler/lib/datetime.desi`

High-level date/time operations with ISO 8601 support. Builds on top of `time.c` (which provides timestamps and sleep).

### Architecture
- ISO parsing: `sscanf()` with `YYYY-MM-DD` and `YYYY-MM-DDTHH:MM:SS` (also supports space separator)
- Month arithmetic: manual year/month rollover with day clamping to `days_in_month`
- Calendar: `strftime("%V")` for ISO week number, `tm_wday` for weekday
- Date validation: verifies year/month/day ranges against `days_in_month`

### API
| Function | Signature |
|---|---|
| `datetime.parse_date` | `(date_str: str) -> float` |
| `datetime.parse` | `(datetime_str: str) -> float` |
| `datetime.to_date_str` | `(ts: float) -> str` |
| `datetime.to_str` | `(ts: float) -> str` |
| `datetime.today` | `() -> str` |
| `datetime.now` | `() -> str` |
| `datetime.add_days` | `(date: str, n: int) -> str` |
| `datetime.add_months` | `(date: str, n: int) -> str` |
| `datetime.add_years` | `(date: str, n: int) -> str` |
| `datetime.diff_days` | `(d1: str, d2: str) -> int` |
| `datetime.days_in_month` | `(year: int, month: int) -> int` |
| `datetime.is_weekend` | `(date: str) -> bool` |
| `datetime.weekday_name` | `(date: str) -> str` |
| `datetime.month_name` | `(month: int) -> str` |
| `datetime.week_number` | `(date: str) -> int` |
| `datetime.is_valid` | `(date: str) -> bool` |
| `datetime.compare` | `(d1: str, d2: str) -> int` |

---

## args Module
**Files:** `compiler/runtime/args.c`, `compiler/lib/args.desi`, `compiler/runtime/entry.c` (modified)

CLI argument parsing. `entry.c` calls `__args_init(argc, argv)` before runtime init to save args globally.

### Architecture
- `__args_init()` stores `argc`/`argv` in globals at program start
- Flag parsing: scans for `--name value` and `--name=value` formats via `strcmp`/`strncmp`
- Positional args: skips entries starting with `-` and flag values
- **Name collision fix**: `all()` renamed to `argv()` because `all` collided with the builtin `all()` function (returns `bool`/`i1`), causing the lowerer to emit wrong LLVM IR return type

### API
| Function | Signature |
|---|---|
| `args.count` | `() -> int` |
| `args.program` | `() -> str` |
| `args.get_arg` | `(index: int) -> str` |
| `args.argv` | `() -> list[str]` |
| `args.positional` | `() -> list[str]` |
| `args.has_flag` | `(name: str) -> int` |
| `args.get_flag` | `(name: str, default: str) -> str` |
| `args.get_flag_int` | `(name: str, default: int) -> int` |
| `args.is_flag_set` | `(name: str) -> int` |
