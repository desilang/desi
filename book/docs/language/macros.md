# Macros and Compile-Time Code Execution

Desi provides a powerful compile-time execution and macro system. This allows you to run Desi code during compilation to perform checks, calculate constants, and even programmatically mutate the Abstract Syntax Tree (AST) of your program.

---

## 1. Compile-Time Function Execution (`@comptime_run`)

If you have calculations or setups that only need to run once at compile time, you can decorate a function with `@comptime_run`. The compiler will execute the function body using its internal tree-walking interpreter while compile-checking your module.

```desi
@comptime_run
def compute_constants():
    let x = 10 + 20
    let y = 30
    print(f"Compile-time calculation: x + y = {x + y}")
```

When you compile this file, you will see the output in your build logs:
```
Compile-time print: Compile-time calculation: x + y = 60
```
This output is printed to the compiler's stderr stream, meaning it won't interfere with the generated binaries or LLVM IR code.

---

## 2. Writing Custom Procedural Macros (`@macro`)

A procedural macro is a compile-time function decorated with `@macro`. Instead of just running side effects, it receives the Abstract Syntax Tree (AST) of the class or function it decorates. Using Desi's built-in AST helper functions, you can query and modify this AST in-place.

### AST Helper Functions

Desi provides the following built-in helper functions to inspect and modify AST nodes at compile time:

| Function | Signature | Description |
|----------|-----------|-------------|
| `ast_get_name(node)` | `(Any) -> str` | Gets the identifier name of a class or function declaration. |
| `ast_set_name(node, name)` | `(Any, str) -> none` | Renames a class or function. |
| `ast_get_body(node)` | `(Any) -> Any` | Gets the block of statements inside a function. |
| `ast_create_print_stmt(msg)` | `(str) -> Any` | Generates a new statement AST node that prints `msg`. |
| `ast_insert_stmt(block, index, stmt)` | `(Any, int, Any) -> none` | Inserts a statement at `index` in a block. |
| `ast_add_stmt(block, stmt)` | `(Any, Any) -> none` | Appends a statement to the end of a block. |

---

### Example: A Logging Decorator Macro

Here is how you can write a macro that automatically inserts a print statement at the beginning of any function it decorates:

```desi
# 1. Define the macro function
@macro
def my_logger(node: Any):
    # Query properties of the target function
    let name = ast_get_name(node)
    let body = ast_get_body(node)
    
    # Generate the log statement
    let stmt = ast_create_print_stmt(f"[LOG] Executing function: {name}")
    
    # Insert the statement at the very beginning (index 0) of the function body
    ast_insert_stmt(body, 0, stmt)

# 2. Use the macro as a decorator
@my_logger
def say_hello():
    print("Inside say_hello")

def main() -> int:
    say_hello()
    return 0
```

### Result at Runtime

When you compile and run the program, the generated print statement executes automatically:
```
[LOG] Executing function: say_hello
Inside say_hello
```

---

## 3. Declarative Macros (Class-level DSLs)

Desi also supports declarative configuration macros (such as the built-in `@model` decorator for the ORM). These are used to configure table schemas, primary keys, and runtime function bindings for databases or external APIs.

For detailed information on configuring declarative class macros, refer to the contributor design docs.
