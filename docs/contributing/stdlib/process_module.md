# Process Module Implementation

Internal documentation for the `process` standard library module.

## Architecture

```
compiler/lib/process.desi     → Desi API (safe wrappers)
compiler/runtime/process.c    → C runtime (fork/exec, popen)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__process_run` | `process.run` | `(cmd, args, argc) → ProcessResult*` |
| `__process_run_shell` | `process.shell` | `(cmd_str) → ProcessResult*` |
| `__process_get_stdout` | `process.get_stdout` | `(ProcessResult*) → char*` |
| `__process_get_stderr` | `process.get_stderr` | `(ProcessResult*) → char*` |
| `__process_get_exit_code` | `process.get_exit_code` | `(ProcessResult*) → int32` |
| `__process_get_ok` | `process.get_ok` | `(ProcessResult*) → int32` (→ bool) |
| `__process_output` | `process.output` | `(cmd, args, argc) → char*` |
| `__process_shell_output` | `process.shell_output` | `(cmd_str) → char*` |
| `__process_free` | `process.free` | `(ProcessResult*) → int32` |
| `__process_pid` | `process.pid` | `() → int64` |
| `__process_ppid` | `process.ppid` | `() → int64` |

## Handle Pattern

`process.shell()` and `process.run()` return a `cptr` handle to a `ProcessResult` struct containing stdout, stderr, and exit code. Handles must be freed with `process.free()`.

## Implementation Notes

- `process.run()` accepts `list<str>` for arguments. The C runtime receives this as a `DesiList*` and must extract elements via `data[i]`.
- `get_stdout()` returns the raw captured output including trailing newline. When printed with `print()`, this produces an extra blank line.
- Shell commands are run via `/bin/sh -c`.

## Portability

- Use `printf` instead of `echo -n` for portable output capture (BSD vs GNU echo).

## Test Coverage

- `examples/501_process_module.desi` — shell, shell_output, pid, ppid, exit codes
