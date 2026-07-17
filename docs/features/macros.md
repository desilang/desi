# Macro System

**Status**: ✅ Fully Implemented (Declarative & Procedural)  
**Since**: v0.1.0  
**Related**: [ORM Models](orm_models.md), [Classes](classes.md)

---

## Overview

Desi features a dual-mode macro system designed to extend the compiler without modifying its source code:

1. **Declarative Macros**: Scheme configurations defined via class-level decorators (such as `@model` or `@store`) that inject properties, chainable/terminal QuerySet methods, operator overloads, and custom primary keys.
2. **Procedural Macros (Compile-Time)**: Custom functions decorated with `@macro` that are executed by the compiler's built-in interpreter during typechecking to inspect and mutate the Abstract Syntax Tree (AST) in-place.

---

## Defining and Using Declarative Macros

A declarative macro is defined by decorating a configuration class with `@macro(target="class")`:

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

Decorating a class with `@store` automatically registers it, letting you call methods like `Product.catalog.find(...)`:

```desi
@store("products")
class Product:
    pub name: CharField(100)
    pub price: IntField()
```

---

## User-Defined Procedural Macros (Compile-Time AST Mutation)

Desi v0.1.0 supports procedural macros written in Desi itself. They are executed at compile-time during the semantic typechecking phase.

### Writing a Procedural Macro

A procedural macro is a top-level function decorated with `@macro`. It receives the AST node of the decorated target as its first parameter:

```desi
@macro
def my_logger(node: Any):
    # 1. Query properties of the AST node
    let name = ast_get_name(node)
    let body = ast_get_body(node)
    
    # 2. Synthesize new AST statements
    let stmt = ast_create_print_stmt(f"[CUSTOM MACRO] Executing function: {name}")
    
    # 3. Mutate the AST in-place (inserted at the beginning of the body block)
    ast_insert_stmt(body, 0, stmt)
```

Applying the macro:

```desi
@my_logger
def say_hello():
    print("Inside say_hello")
```

At compile-time, the decorator intercept runs, rewriting `say_hello` to print the custom log statement before executing the main function body.

---

## Compiler Architecture (For Contributors)

### 1. Macro Collection & Decorator Interception
The typechecker pipeline resides in `compiler/internal/check/check.go` and `check_type.go`:
* **Phase 1 (Collection)**: The compiler scans declarations. Functions decorated with `@macro` are collected into `checker.macroDefs`.
* **Phase 2 (Typechecking)**: Right before checking function body blocks (`checkFunc`) or class declarations (`checkClass`), the compiler loops over any applied decorators. If a decorator name matches a registered custom macro, it suspends checking, initializes a new environment, and invokes the interpreter.

### 2. The Tree-Walking Interpreter
The evaluator package is located in [compiler/internal/eval/eval.go](file:///Users/desiprogrammer/Desktop/Projects/go/desilang/desi/compiler/internal/eval/eval.go):
* **Values**: Values are wrapped in Go structures implementing the `Value` interface (`IntValue`, `StrValue`, `AstNodeValue` for wrapping Go AST nodes, etc.).
* **Scoping**: Lexical environment nesting is managed via the `Env` struct.
* **Introspection & Mutation API**: Builtin compile-time methods are intercepted and handled inside the interpreter's `CallExpr` case:
  * `ast_get_name(node)`: Inspects `*ast.FuncDecl` or `*ast.ClassDecl` and returns their string identifier name.
  * `ast_set_name(node, new_name)`: Updates the name identifier of the AST node.
  * `ast_get_body(node)`: Extracts the `*ast.Block` statement from a function declaration.
  * `ast_create_print_stmt(msg)`: Allocates an `*ast.ExprStmt` containing a `*ast.CallExpr` targeting `print`.
  * `ast_insert_stmt(block, index, stmt)`: Mutates a block's statement array (`block.Stmts`) to insert a new statement in-place.
  * `ast_add_stmt(block, stmt)`: Appends a statement to the end of the block.

* **Printing in Macros**: When `print` is called inside a macro at compile-time, it writes to `os.Stderr`. This prevents printing into standard output (`stdout`), which is reserved for emitting LLVM IR assemblies.

### 3. Registering New AST Helpers
To add a new AST mutation or query function:
1. Register its callable name and type signature in `addPreludeBuiltins` inside [compiler/internal/check/info.go](file:///Users/desiprogrammer/Desktop/Projects/go/desilang/desi/compiler/internal/check/info.go).
2. Add the name to the `preludeBuiltinNames` map in [compiler/internal/check/prelude_scope.go](file:///Users/desiprogrammer/Desktop/Projects/go/desilang/desi/compiler/internal/check/prelude_scope.go) to avoid shadowing diagnostics.
3. Add a corresponding `case` handling inside `Eval` for `*ast.CallExpr` in [compiler/internal/eval/eval.go](file:///Users/desiprogrammer/Desktop/Projects/go/desilang/desi/compiler/internal/eval/eval.go).

---

## Testing & Verification

To run tests specifically covering custom macros:

```bash
# Compile-time evaluation smoke test
./test_examples.sh 385,385

# Procedural macro AST mutation test
./test_examples.sh 443,443

# Full regression suite (487 tests)
./test_examples.sh
```
