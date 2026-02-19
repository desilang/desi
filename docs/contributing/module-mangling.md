# Module Function Name Mangling

When Desi imports a module, the compiler **mangles** all non-`@extern` function
definitions with a `__desi$` prefix to prevent them from shadowing C standard
library or POSIX functions.

## Problem

Desi modules wrap `@extern` C functions in `pub def` wrappers:

```desi
# math/__mod.desi
@extern
def __math_sin(x: float) -> float: ...

pub def sin(x: float) -> float:
    return __math_sin(x)
```

Without mangling, the compiled `sin()` wrapper shares its name with C's `sin()`,
causing linker symbol collisions or infinite recursion.

## Architecture

### Definition Side (module_lower.go)

`LowerModuleOptions.IsImportedModule` controls mangling:

```go
type LowerModuleOptions struct {
    SkipBuiltinEnums bool
    IsImportedModule bool  // force-mangle all non-extern definitions
}
```

When `IsImportedModule=true`, `LowerFuncFromDeclEx` prefixes all non-`@extern`
function names with `__desi$`:

| Source Name | Compiled Name    | Reason                                |
|-------------|------------------|---------------------------------------|
| `sin`       | `__desi$sin`     | pub def wrapper — mangled             |
| `__math_sin`| `__math_sin`     | @extern — NOT mangled                 |

### Caller Side (hir_lower.go → calleeName)

The `calleeName` function resolves call targets through several strategies:

1. **Module-qualified calls** (`math.sin`) → `extractExternFromWrapper` finds
   the `@extern` function called inside the wrapper → resolves to `__math_sin`
2. **ChosenOverloads** → For overloaded from-import calls, checks `ModuleDecl`
   and applies the same wrapper-extraction logic
3. **info.Funcs[name]** → If the candidate has `ModuleDecl`, resolves to
   `@extern` inner function or mangles with `__desi$`
4. **FromItems** → For `from X import Y` calls where previous checks didn't
   match, mangles the name if it's a lowercase function (not a class) and not a
   Desi compiler builtin

### Exclusions from Mangling

The `desiBuiltins` map (in `hir_lower.go`) lists all compiler intrinsics that
must NOT be mangled even when from-imported (`print`, `len`, `str`, etc.).
These have special lowering in `lowerCall` that checks `calleeName == "print"`.

Uppercase names (classes like `Mutex`, `Channel`) are also excluded because
they use separate constructor lowering paths.

## Adding New Stdlib Modules

When adding a new module:

1. Set `IsImportedModule: true` in `LowerModuleOptions` (already automatic for
   all imported modules via `closure_lower.go` and `emit_ir_cmd.go`)
2. Name your `@extern` C functions with a module prefix (e.g., `__math_sin`,
   `__fs_read_file`) to avoid collisions
3. Use `pub def` wrappers for the user-facing API
4. **No need to worry about C name collisions** — the compiler handles it

## Adding New Builtins

If you add a new Desi compiler builtin (handled in `lowerCall`):

1. Add it to the `desiBuiltins` map in `hir_lower.go`
2. This prevents mangling when the builtin name is from-imported

## Runtime: Recursion Depth

`__desi_call_enter(const char* func_name)` is called at the start of every user
function. It tracks call depth and panics with the function name if the
recursion limit is exceeded:

```
Desi panic: maximum recursion depth exceeded (1000) in 'fibonacci'
```

The `__desi$` prefix is stripped before display so users see clean function names.
