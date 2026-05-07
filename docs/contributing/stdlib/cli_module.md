# CLI Module Implementation

Internal documentation for the `cli` standard library module.

## Architecture

```
compiler/lib/cli.desi        → Desi API (define, validate, get flags)
compiler/runtime/args.c      → C runtime (argc/argv parsing, flag registry)
```

**Note**: The `cli` module wraps the same `__args_*` C runtime used by the `args` module. The `cli` module provides a higher-level interface with flag definitions, validation, help generation, and subcommands.

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__args_count` | `cli.count` | `() → int32` |
| `__args_program` | `cli.program` | `() → char*` |
| `__args_get` | `cli.get` | `(int32) → char*` |
| `__args_has_flag` | `cli.has_flag` | `(char*) → int32` (→ bool) |
| `__args_get_flag` | `cli.get_flag` | `(char*, char*) → char*` |
| `__args_get_flag_int` | `cli.get_flag_int` | `(char*, int32) → int32` |
| `__args_get_flag_bool` | `cli.get_flag_bool` | `(char*) → int32` (→ bool) |
| `__args_set_description` | `cli.set_description` | `(char*) → int32` |
| `__args_define` | `cli.define` | `(char*, char*, char*, char*) → int32` |
| `__args_define_bool` | `cli.define_bool` | `(char*, char*, char*) → int32` |
| `__args_require` | `cli.require` | `(char*) → int32` |
| `__args_print_help` | `cli.print_help` | `() → int32` |
| `__args_check_help` | `cli.check_help` | `() → int32` (→ bool) |
| `__args_validate` | `cli.validate` | `() → char*` |
| `__args_subcommand` | `cli.subcommand` | `() → char*` |

## Implementation Notes

- The `cli` module was refactored to use `__args_*` symbols (not `__cli_*`) since the C runtime only exports `__args_*`.
- `args` and `cli` share the same C runtime — `args` is the low-level interface, `cli` adds definitions and validation.
- `validate()` returns an empty string on success, or an error message describing which required flags are missing.
- `check_help()` returns `true` if `--help` or `-h` was passed.
- No handles needed — all state is global (argc/argv + flag registry).

## Test Coverage

- `examples/512_cli_module.desi`
