# Macro System

**Status**: ✅ Implemented  
**Since**: v0.10  
**Related**: [ORM Models](orm_models.md), [Classes](classes.md)

---

## Overview

Desi's macro system lets users define custom class decorators entirely in `.desi` files. The compiler is 100% domain-agnostic — it contains zero hardcoded decorator names. All behavior comes from the macro registry, which is populated from macro definitions.

The built-in `@model` decorator (for ORM) is itself defined via this macro system in `stdlib/macros/orm.desi`.

## Defining a Macro

A macro is a class decorated with `@macro(target="class")`:

```desi
@macro(target="class")
class store:
    const property_name: str = "catalog"
    const chainable: list = ["find", "sort_by"]
    const terminal: list = ["fetch", "add", "remove", "tally"]
    const builtins: dict = {"QQ": "str"}
    const operators: dict = {"QQ|QQ": "__q_or"}
    const runtime: dict = {
        "reset":    "__qs_reset",
        "find":     "__qs_filter",
        "fetch":    "__qs_fetch",
        "add":      "__qs_do_insert"
    }
    const require_fields: bool = true
    const auto_pk: bool = true
```

## Configuration Keys

| Key | Type | Description |
|-----|------|-------------|
| `property_name` | `str` | Injected property on decorated classes (e.g., `"objects"`, `"catalog"`) |
| `chainable` | `list` | Methods that return the manager for chaining |
| `terminal` | `list` | Methods that execute and return results |
| `builtins` | `dict` | Global functions this macro introduces (`name → return_type`) |
| `operators` | `dict` | Binary/unary operator overloads (`pattern → c_func`) |
| `runtime` | `dict` | C runtime function mapping (`method → c_function_name`) |
| `runtime_type` | `str` | Runtime type: `"c"` (default), future: `"desi"` |
| `require_fields` | `bool` | Require at least one field in decorated classes |
| `auto_pk` | `bool` | Auto-add `id: AutoField` if no PK exists |
| `forbidden_field_names` | `list` | Field names users cannot declare |

## Using a Custom Macro

After defining `@store`, use it exactly like `@model`:

```desi
@store("products")
class Product:
    pub name: CharField(100)
    pub price: IntField()

def main() -> int:
    # The compiler resolves "catalog" and "find" from the registry!
    # Product.catalog.find(name="Widget")
    0
```

## Operator Patterns

| Pattern | Type | Example |
|---------|------|---------|
| `Q\|Q` | Binary OR | `Q(name="Ali") \| Q(age=25)` |
| `Q&Q` | Binary AND | `Q(name="Ali") & Q(active=true)` |
| `~Q` | Unary NOT | `~Q(active=false)` |
| `F+F` | Arithmetic | `F("price") + F("tax")` |

## Macro Loading Pipeline

```
1. loadEmbeddedMacros()      — stdlib/macros/*.desi (embedded in binary)
2. loadProjectMacros()       — project/macros/*.desi (from desi.mod)
3. LoadMacrosFromModule()    — inline @macro classes in entry file
4. Go init() fallback        — orm_protocol.go (backward compat)
```

All loading happens **before type checking**, so macros are available for decorator resolution.

## Compiler Architecture

The compiler never mentions any domain-specific strings. All dispatch goes through the `MacroRegistry`:

| Compiler File | What It Does |
|---------------|-------------|
| `check/expr_field.go` | `Registry.LookupProperty()` — resolves `Class.property` |
| `check/expr_field.go` | `Registry.LookupMethodOnClass()` — resolves `Class.property.method()` |
| `check/expr_call.go` | `Registry.LookupBuiltin()` — resolves `Q()`, `F()`, etc. |
| `check/check_type.go` | `cls.MacroDecorator != ""` — gates ORM field resolution |
| `lower/lower_call.go` | `Registry.LookupProperty()` — emits `__qs_reset` calls |
| `lower/class_lower.go` | `cls.MacroDecorator != ""` — gates `LowerModelInit` |
| `macro/registry.go` | Central registry for all protocols, properties, builtins |

## Runtime Mapping

The `runtime` dict maps abstract method names to C function names:

```desi
const runtime: dict = {
    "reset":    "__qs_reset",     # binds table name
    "filter":   "__qs_filter",   # adds WHERE clause
    "fetch":    "__qs_fetch",    # executes SELECT
    "create":   "__qs_do_insert" # executes INSERT
}
```

Custom macros can:
1. **Reuse existing C functions** (e.g., the ORM's `__qs_*` functions)
2. **Point to custom C libraries** via `@extern` declarations

## File Map

| File | Role |
|------|------|
| `stdlib/macros/orm.desi` | `@model` macro definition |
| `compiler/internal/macro/registry.go` | `MacroRegistry`, `PropertySpec`, `MacroBuiltin` |
| `compiler/internal/macro/loader.go` | Parses `@macro` classes from AST |
| `compiler/internal/macro/orm_protocol.go` | Go fallback for `@model` registration |
| `compiler/internal/macro/builtin_decorators.go` | Tier 1 decorator guard |

## Testing

```bash
# Custom macro test
./bin/desic run examples/442_custom_macro.desi

# Full regression suite (438 tests)
bash test_examples.sh
```

## Future Work

- [ ] `runtime_type = "desi"` — pure Desi macro implementations (no C)
- [ ] `[macros]` section in `desi.mod` for macro path configuration
- [ ] Macro composition — combining multiple macros on one class
- [ ] `@macro(target="func")` — function-level macros
- [ ] `@macro(target="field")` — field-level macros
