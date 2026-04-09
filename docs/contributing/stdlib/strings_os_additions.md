# strings Module — Case Converters & Text Utilities

New additions to the strings module: case convention converters, text processing utilities, and numeric validation.

## New C Runtime Functions (in `compiler/runtime/strings.c`)

### Case Convention Converters

| C Function | Desi Binding | Description |
|-----------|-------------|-------------|
| `__strings_camel_case(s)` | `camel_case(s)` | `"hello_world"` → `"helloWorld"` |
| `__strings_pascal_case(s)` | `pascal_case(s)` | `"hello_world"` → `"HelloWorld"` |
| `__strings_snake_case(s)` | `snake_case(s)` | `"helloWorld"` → `"hello_world"` |
| `__strings_kebab_case(s)` | `kebab_case(s)` | `"helloWorld"` → `"hello-world"` |
| `__strings_screaming_snake(s)` | `screaming_snake(s)` | `"helloWorld"` → `"HELLO_WORLD"` |

### Text Utilities

| C Function | Desi Binding | Description |
|-----------|-------------|-------------|
| `__strings_word_wrap(s, width)` | `word_wrap(s, width)` | Break text at word boundaries |
| `__strings_is_numeric(s)` | `is_numeric(s)` | Check if string is valid number |
| `__strings_dedent(s)` | `dedent(s)` | Remove common leading whitespace |
| `__strings_abbreviate(s, max)` | `abbreviate(s, max)` | Truncate at word boundary with `...` |

## Algorithm Notes

### Case Convention Detection

All converters handle three input formats:
- `snake_case` (underscore-separated)
- `kebab-case` (dash-separated)
- `camelCase` / `PascalCase` (detected via uppercase transitions)

For camelCase detection, an uppercase letter preceded by a lowercase letter triggers a word boundary. Example: `"firstName"` → boundary detected between `t` and `N`.

### `screaming_snake` Implementation

Delegates to `snake_case()` then `upper()` — avoids duplicating word-splitting logic.

### `dedent` Algorithm

1. Scan all non-empty lines, count leading whitespace (tabs = 4 spaces)
2. Find minimum indentation
3. Strip that many characters from each line's start
4. Empty lines are preserved but not used for minimum calculation

### `is_numeric` Validation

Accepts: `42`, `3.14`, `-5`, `+0.5`, `.5`
Rejects: `""`, `abc`, `1.2.3`, `+`, `12abc`

## Files Modified

| File | Changes |
|------|---------|
| `compiler/runtime/strings.c` | Added 9 C functions (lines 540-800) |
| `compiler/lib/strings.desi` | Added 9 @extern bindings + 9 pub def wrappers |
| `examples/454_strings_extras.desi` | Test for new functions |

---

# os Module — New Utilities

New additions to the os module: home directory, temp directory, command lookup, and sleep.

## New C Runtime Functions (in `compiler/runtime/os.c`)

| C Function | Desi Binding | Description |
|-----------|-------------|-------------|
| `__os_home_dir()` | `home_dir()` | User's home directory |
| `__os_temp_dir()` | `temp_dir()` | System temp directory |
| `__os_which(name)` | `which(name)` | Find executable in PATH |
| `__os_sleep_ms(ms)` | `sleep_ms(ms)` | Sleep for N milliseconds |

## Cross-Platform Implementations

### `home_dir()`
- **POSIX**: `$HOME`
- **Windows**: `%USERPROFILE%`, fallback to `%HOMEDRIVE%%HOMEPATH%`

### `temp_dir()`
- **POSIX**: `$TMPDIR`, fallback to `/tmp`
- **Windows**: `%TEMP%`, fallback to `%TMP%`, then `C:\Temp`

### `which(name)`
- Splits `$PATH` by `:` (POSIX) or `;` (Windows)
- Tests each `dir/name` with `access(full, X_OK)`
- Returns first match or empty string

### `sleep_ms(ms)`
- **POSIX**: `usleep(ms * 1000)`
- **Windows**: `Sleep(ms)` (kernel32)

## Files Modified

| File | Changes |
|------|---------|
| `compiler/runtime/os.c` | Added 4 C functions |
| `compiler/lib/os.desi` | Added 4 @extern bindings + 4 pub def wrappers |
| `examples/455_os_extras.desi` | Test for new functions |
