# io Module Implementation

The `io` module provides standard input/output: reading from stdin, writing to stderr.

## Architecture

```
import io
    ↓
compiler/lib/io.desi  →  @extern("C") bindings
    ↓
compiler/runtime/io.c  →  C functions (__io_*)
```

## Files

| File | Purpose |
|------|---------|
| `compiler/lib/io.desi` | Desi bindings |
| `compiler/runtime/io.c` | C runtime (stdin/stderr operations) |

## C Runtime Functions

| C Function | Desi Binding | Return | Description |
|-----------|-------------|--------|-------------|
| `__io_input(prompt)` | `input(prompt)` | `str` | Read line with prompt |
| `__io_read_all()` | `read_all()` | `str` | Read all stdin until EOF |
| `__io_eprint(msg)` | `eprint(msg)` | `void` | Print to stderr + newline |
| `__io_ewrite(msg)` | `ewrite(msg)` | `void` | Write to stderr, no newline |
| `__io_flush()` | `flush()` | `void` | Flush stdout |
| `__io_has_input()` | `has_input()` | `bool` | Non-blocking stdin check |

## Design Decisions

- `input()` mirrors Python's `input()` — prints prompt, reads until `\n`, strips trailing newline
- `has_input()` uses `select()` with zero timeout for non-blocking detection
- `ewrite()` calls `fflush(stderr)` immediately after write
- Buffer for `input()` is 4096 bytes — sufficient for interactive use
- `read_all()` grows buffer dynamically (1KB initial, doubles as needed)

## Cross-Platform Notes

- `has_input()` uses `select()` on POSIX, not available on Windows — will need `WaitForSingleObject` or `_kbhit()` for Windows support
- All file descriptors use standard C `FILE*` — portable across platforms
