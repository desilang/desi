# Contributing to Desi's Standard Library

This guide explains how to add a new module or extend an existing one in Desi's stdlib.

## Architecture Overview

Each stdlib module consists of **3 layers**:

```
compiler/runtime/<module>.c    ← C implementation (compiled into libdesi.a)
compiler/lib/<module>.desi     ← Desi bindings (@extern wrappers)
book/docs/stdlib/<module>.md   ← User-facing documentation
```

```mermaid
flowchart LR
    A["Desi Source<br/>import path"] --> B["path.desi<br/>pub def exists()"]
    B --> C["@extern binding<br/>__path_exists()"]
    C --> D["path.c<br/>C implementation"]
    D --> E["libdesi.a<br/>static library"]
```

### How It Works

1. **Desi source** calls `path.exists("/tmp")`
2. **Module resolver** finds `compiler/lib/path.desi` and resolves the call
3. **`path.desi`** declares `@extern("C")` binding → calls `__path_exists()` in C
4. **`path.c`** implements the actual logic, compiled into `build/libdesi.a`
5. **Linker** links the compiled Desi program against `libdesi.a`

## Step-by-Step: Adding a New Module

### 1. Create the C Runtime (`compiler/runtime/<module>.c`)

Use the `__<module>_` prefix for all function names to avoid collisions:

```c
// mymod.c - My module C runtime
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Always handle NULL inputs gracefully
char* __mymod_greet(const char* name) {
    if (!name) return strdup("");
    size_t len = strlen(name) + 8; // "Hello, " + name + \0
    char* result = (char*)malloc(len);
    if (!result) return strdup("");
    snprintf(result, len, "Hello, %s", name);
    return result;
}

int __mymod_add(int a, int b) {
    return a + b;
}
```

> [!IMPORTANT]
> **Memory rules for C functions:**
> - Functions returning `str` (`char*`) must return `malloc`'d memory — the caller owns it
> - Always handle `NULL` inputs — return `strdup("")` for strings, `0`/`-1` for ints
> - Never return stack-allocated strings (no `return buf;` where `buf` is a local array)
> - String literals like `return "darwin"` are OK (static storage) for fixed values

### 2. Create the Desi Bindings (`compiler/lib/<module>.desi`)

Follow this exact pattern — extern declarations at top, pub def wrappers below:

```python
# mymod module — description
#
# Usage:
#   import mymod
#   let msg = mymod.greet("World")

# --- C runtime bindings (private) ---

@extern("C")
def __mymod_greet(name: str) -> str

@extern("C")
def __mymod_add(a: int, b: int) -> int

# --- Public API ---

## Greet someone by name.
pub def greet(name: str) -> str:
    unsafe:
        return __mymod_greet(name)

## Add two integers.
pub def add(a: int, b: int) -> int:
    unsafe:
        return __mymod_add(a, b)
```

> [!NOTE]
> **Desi ↔ C type mapping:**
> | Desi | C | Notes |
> |------|---|-------|
> | `str` | `const char*` (input) / `char*` (return) | Returns must be `malloc`'d |
> | `int` | `int` | |
> | `bool` | `int` | 0 = false, non-zero = true |
> | `float` | `double` | |
> | `none` | `void` | |
> | `list[T]` | `DesiList*` | C function returns/accepts `DesiList*` pointer |
> | `dict[K,V]` | `DesiDict*` | C function returns/accepts `DesiDict*` pointer |
> | `set[T]` | `DesiSet*` | C function returns/accepts `DesiSet*` pointer |

### 3. No Registration Needed

The build system auto-discovers files:
- **C files**: `Makefile` globs all `compiler/runtime/*.c` into `libdesi.a`
- **Desi files**: The import resolver searches `compiler/lib/` for `<module>.desi`

No edits to `Makefile`, `embed.go`, or any Go files required.

### 4. Check the Collision Table

If your public function names match C standard library names (e.g., `basename`, `getpid`, `system`), you **must** add them to the collision table in:

**`compiler/internal/lower/hir_lower.go`** → `mangleDesiName()` function

```go
cStdlibConflicts := map[string]bool{
    // ... existing entries ...
    // Add yours here:
    "basename": true, "dirname": true,
}
```

> [!CAUTION]
> **If you skip this step**, calling your function will cause **infinite recursion** (stack overflow).
> The Desi function name collides with the C function name at link time, and the Desi function
> calls itself instead of the C runtime function.
>
> **How to tell**: If your function triggers `Desi panic: maximum recursion depth exceeded (1000)`,
> the function name needs to be added to the collision table.

Common POSIX/C names that need mangling: `getcwd`, `chdir`, `getpid`, `setenv`, `basename`,
`dirname`, `system`, `exit`, `getenv`, `sleep`, `read`, `write`, `close`, `stat`, `realpath`.

### 5. Write Tests

Create `examples/<NNN>_<module>_module.desi` with `# EXPECTED_OUTPUT:` comments:

```python
import mymod

def main() -> int:
    print(mymod.greet("Desi"))
    print(mymod.add(2, 3))
    0

# EXPECTED_OUTPUT:
# Hello, Desi
# 5
```

> [!TIP]
> - Use `true`/`false` checks for system-dependent values (e.g., `print(os.getpid() > 0)`)
> - Run `./test_examples.sh` to verify all tests pass
> - Always do a clean build first: `make clean && make`

### 6. Write Documentation

Create `book/docs/stdlib/<module>.md` with:

- Import instruction
- API reference table (Function | Returns | Description)
- Usage example
- See Also links to related modules

## Extending an Existing Module

Adding functions to an existing module (e.g., `strings`) is simpler — same steps but you append to existing files:

1. Add C function to `compiler/runtime/<module>.c` (keep the `__<module>_` prefix)
2. Add `@extern("C")` declaration + `pub def` wrapper to `compiler/lib/<module>.desi`
3. Check collision table for any new names that match C stdlib
4. Update the test example and docs
5. `make runtime && go build -ldflags="-s -w" -o bin/desic ./compiler/cmd/desic`
6. Test: `./bin/desic run examples/<NNN>_<module>.desi`

## Quick Reference

| What | Where |
|------|-------|
| C runtime implementations | `compiler/runtime/*.c` |
| Desi module bindings | `compiler/lib/*.desi` |
| Name collision table | `compiler/internal/lower/hir_lower.go` → `mangleDesiName()` |
| User-facing docs | `book/docs/stdlib/*.md` |
| Test examples | `examples/3xx_*.desi` |
| Build runtime only | `make runtime` |
| Build compiler only | `go build -ldflags="-s -w" -o bin/desic ./compiler/cmd/desic` |
| Full clean build | `make clean && make` |
| Run all tests | `./test_examples.sh` |

## Current Modules

| Module | Functions | C File | Status |
|--------|-----------|--------|--------|
| `strings` | 38 | `strings.c` | Complete (incl. split/join/splitlines) |
| `os` | 12 | `os.c` | Complete |
| `path` | 12 | `path.c` | Complete |
| `log` | 6 | `log.c` | Complete |
| `math` | 30+ | `math.c` | Complete |
| `json` | 4 | `json.c` | Complete |
| `time` | 10+ | `time.c` | Complete |
| `sync` | — | `mutex.c`, `rwlock.c`, etc. | Complete |

## Known Limitations

- **Variadic arguments** are not supported in `@extern("C")` declarations.
- **Struct return types** are not yet supported through the module path.
