# IO Module Implementation

This document covers the internal implementation details of the `io` module for contributors.

## Architecture

```
compiler/lib/io.desi         ← Desi public API (6 functions)
         ↓ (extern "C" calls)
compiler/runtime/io.c        ← C runtime (stdin/stderr operations)
```

## C Runtime Details

### `__io_input(prompt)`

1. Prints prompt to stdout (no newline)
2. Calls `fflush(stdout)` to ensure prompt is visible
3. Reads from stdin with `fgets()` into 4096-byte buffer
4. Strips trailing `\n` if present
5. Returns `strdup()` of the result

### `__io_has_input()`

Uses POSIX `select()` with zero timeout to check if stdin has data without blocking:

```c
fd_set fds;
FD_ZERO(&fds);
FD_SET(STDIN_FILENO, &fds);
struct timeval tv = {0, 0};  // zero timeout
return select(STDIN_FILENO + 1, &fds, NULL, NULL, &tv) > 0;
```

This allows programs to detect piped input vs interactive mode.

### `__io_read_all()`

Dynamic buffer growth:
1. Start with 1024-byte buffer
2. Read chunks with `fread()`
3. Double buffer when full
4. Return null-terminated string

## Windows Compatibility Note

`select()` on stdin doesn't work on Windows. Future Windows port will need:
- `_kbhit()` for interactive check
- `WaitForSingleObject(GetStdHandle(STD_INPUT_HANDLE), 0)` for pipe detection
