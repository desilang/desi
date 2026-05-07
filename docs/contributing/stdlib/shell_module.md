# Shell Module Implementation

Internal documentation for the `shell` standard library module.

## Architecture

```
compiler/lib/shell.desi     → Desi API (exec, file ops)
compiler/runtime/shell.c    → C runtime (popen, POSIX file I/O)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__shell_exec` | `shell.exec` | `(const char*) → char*` |
| `__shell_exec_status` | `shell.exec_status` | `(const char*) → int32` |
| `__shell_glob` | `shell.glob` | `(const char*) → char*` |
| `__shell_which` | `shell.which` | `(const char*) → char*` |
| `__shell_cp` | `shell.cp` | `(const char*, const char*) → int32` |
| `__shell_mv` | `shell.mv` | `(const char*, const char*) → int32` |
| `__shell_rm` | `shell.rm` | `(const char*) → int32` |
| `__shell_mkdir_p` | `shell.mkdir_p` | `(const char*) → int32` |
| `__shell_rmdir` | `shell.rmdir` | `(const char*) → int32` |
| `__shell_exists` | `shell.exists` | `(const char*) → int32` (→ bool) |
| `__shell_is_file` | `shell.is_file` | `(const char*) → int32` (→ bool) |
| `__shell_is_dir` | `shell.is_dir` | `(const char*) → int32` (→ bool) |
| `__shell_cat` | `shell.cat` | `(const char*) → char*` |
| `__shell_write` | `shell.write` | `(const char*, const char*) → int32` |
| `__shell_append` | `shell.append` | `(const char*, const char*) → int32` |
| `__shell_lines` | `shell.lines` | `(const char*) → int32` |

## Implementation Notes

- `exec()` uses `popen()` to capture stdout. `exec_status()` uses `system()`.
- `which()` searches `$PATH` for the executable.
- `mkdir_p()` creates intermediate directories recursively (like `mkdir -p`).
- `lines()` counts newline characters (`\n`) in the file.
- No handles needed — all functions are stateless POSIX wrappers.

## Known Behavior

- `\n` in Desi string literals is currently passed as a literal two-character backslash-n to the C runtime, not as an actual newline. This affects `append()` and `lines()` counts.

## Test Coverage

- `examples/509_shell_module.desi` — exec, status, which, mkdir, write, read, copy, move, rm, exists
