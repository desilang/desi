# Diagnostic Reference

Desi's diagnostics are **single-sourced** from a catalog (`compiler/internal/diag/codes.json`).
Every error and warning has a unique code for easy lookup.

**142 diagnostic codes** across 14 categories.

---

## Lexer Errors

*Tokenization and lexical analysis*

| Code | Title | Help |
|------|-------|------|
| `DLE0001` | unterminated string | did you forget a closing quote? |
| `DLE0002` | mixed tabs and spaces in indentation | use either spaces or tabs consistently; do not mix on the same line |
| `DLE0003` | spaces used for indentation; tabs required | Desi enforces tabs-only indentation. Configure your editor to insert tabs (not spaces) for leading whitespace. |
| `DLE0011` | invalid number literal |  |
| `DLE0012` | invalid float fraction | digits must appear after the decimal point; underscores cannot lead/trail or double |
| `DLE0013` | invalid float exponent | write like '1.2e+3'; exponent must contain digits (underscores allowed between digits) |
| `DLE0014` | invalid hex literal | hex digits are 0-9 or a-f (case-insensitive); underscores cannot lead/trail or double |
| `DLE0015` | invalid hex fraction | fractional part must contain hex digits |
| `DLE0016` | hex float missing exponent 'p' | hex floating-point literals require a 'p' exponent, e.g. 0x1.fp3 |
| `DLE0017` | invalid hex float exponent | use 'p±<digits>', underscores only between digits |
| `DLE0018` | invalid binary literal | binary digits are 0 or 1; underscores cannot lead/trail or double |
| `DLE0019` | invalid octal literal | octal digits are 0..7; underscores cannot lead/trail or double |
| `DLE0020` | invalid escape sequence | supported: \\ \" \n \r \t \0 \xNN \uXXXX \UXXXXXXXX |
| `DLE0099` | lexer error |  |

## Parser Errors

*Syntax and grammar*

| Code | Title | Help |
|------|-------|------|
| `DPE0001` | unexpected token | this token is not valid here; check for a missing delimiter or keyword before it |
| `DPE0002` | expected a different token | the parser expected a specific token here; verify punctuation and keywords |
| `DPE0003` | unclosed delimiter | a '(' '[' or '{' was opened but not closed on this path |
| `DPE0004` | trailing or extra token | remove extra punctuation or a stray comma at the end of the list |
| `DPE0005` | invalid assignment target | only names, tuple patterns, or index/field targets may appear on the left of an assignment |
| `DPE0100` | unexpected token after 'pub' |  |
| `DPE0101` | wildcard '_' cannot have a payload |  |
| `DPE0110` | missing 'let' before variable declaration | Write 'let name = …' to declare a variable; plain '=' is not a statement operator. |
| `DPE1001` | async only valid before 'def' | Place 'async' immediately before 'def', or remove it. |
| `DPE1002` | async not allowed before 'let' | async is not a statement modifier; remove it before 'let'. |
| `DPE1003` | await requires an expression | Write 'await <expr>' where <expr> is a future. |

## Type Errors

*Type checking and inference*

| Code | Title | Help |
|------|-------|------|
| `DDF0005` | inconsistent default parameters across overloads | all overloads for a function name must agree on which parameters have defaults |
| `DTE0001` | undefined name | this name is not defined in the current scope |
| `DTE0002` | arity mismatch in grouped binding | the number of names on the left must equal the number of values on the right |
| `DTE0003` | name already defined in this scope | choose a different name or remove the earlier declaration |
| `DTE0004` | type mismatch | the expression does not have the expected type |
| `DTE0005` | return type mismatch | the returned expression does not match the function's declared return type |
| `DTE0006` | cannot assign to immutable variable | declare the variable with 'mut' if you intend to reassign it |
| `DTE0010` | symbol is not public | symbols imported from another module must be declared with 'pub' |
| `DTE0011` | public constant must be compile-time constant | public constants must be literal compile-time values |
| `DTE0012` | public let cannot be mutable | remove 'mut' from public top-level let declarations |
| `DTE0013` | mutation requires ':=' operator | use ':=' instead of '=' when mutating an existing value (e.g., obj.field := value) |
| `DTE0014` | use 'none' instead of 'void' | Desi does not have 'void'; use 'none' for no-value types |
| `DTE0020` | reserved keyword used as an identifier | choose a different name; language keywords and special names cannot be used as identifiers |
| `DTE0021` | cannot shadow prelude builtin | rename your binding; builtins like 'len' and 'print' cannot be redefined |
| `DTE0022` | name conflicts with imported symbol | rename your declaration; imported names cannot be redefined |
| `DTE0023` | 'void' is not a type; use 'none' | Desi uses 'none' to denote no value. Replace 'void' with 'none'. |
| `DTE0030` | incompatible struct assignment | assign a value of the same struct type |
| `DTE0031` | unsupported assignment target | the left-hand side must be a name, index, or field expression |
| `DTE0032` | unknown struct type | this struct type is not defined |
| `DTE0033` | field access on non-struct | only struct values support field access |
| `DTE0034` | cannot assign to field on non-struct | the left-hand side must be a struct value |
| `DTE0035` | defer is not allowed here | move the 'defer' to the function top level |
| `DTE0036` | defer expects a call expression | use 'defer some_func(...)' |
| `DTE0037` | invalid condition type | conditions must be bool or int |
| `DTE0038` | symbol is not a value | call the function or refer to a constant/type as appropriate |
| `DTE0039` | unknown symbol in module alias | verify the symbol name exported by that module |
| `DTE0040` | unknown field on struct | check the field name against the struct definition |
| `DTE0041` | unknown enum variant | verify the variant name declared in the enum |
| `DTE0042` | duplicate match arm | each variant should appear at most once |
| `DTE0043` | payloadless variant used with a binder | remove the binder or use a variant that carries a payload |
| `DTE0044` | wrong number of arguments | constructors for payloadless variants take no arguments; payloadful ones take exactly one |
| `DTE0045` | wrong number of arguments | verify the arity of the function exported by that module |
| `DTE0046` | wrong number of arguments | pass exactly the required number of arguments |
| `DTE0047` | incompatible enum assignment | assign a value of the same enum type/variant |
| `DTE0050` | single-element tuples are not supported | use the value directly instead of wrapping it in a tuple; single-element tuples add no value |
| `DTE0051` | global variable must be immutable | Global variables cannot be mutable (no 'mut'). Use 'let NAME = ...'. |
| `DTE0052` | non-exhaustive match | add arms for the missing variants or a wildcard '_' arm |
| `DTE0101` | no matching overload | no overload matches the provided argument types |
| `DTE0102` | ambiguous overload | multiple overloads match the provided argument types |
| `DTE0103` | bad pipeline feed | pipeline requires a call on the right; the piped value becomes the first argument |
| `DTE0104` | invalid operand types | operator does not support these operand types |
| `DTE0105` | value is not callable | only functions can be called |
| `DTE0106` | cannot mix positional and named arguments in struct initialization | use either all positional arguments (by field order) or all named arguments; do not mix them |
| `DTE0107` | duplicate field in struct initialization | each field should be specified exactly once |
| `DTE0108` | unknown field in struct initialization | check the field name against the struct definition |
| `DTE0109` | missing field or wrong arity in struct initialization | provide all required fields — either by name or positionally in declaration order |
| `DTE0110` | argument is void | arguments must produce a value; remove expressions of kind 'none' |
| `DTE0111` | unsupported argument kind | convert the expression to a supported kind (int/float/str/bool) |
| `DTE0112` | wrong number of arguments | check the function's signature and adjust the number of arguments |
| `DTE0200` | missing stdlib import | stdlib modules must be imported before use; add 'import <module>' at the top of your file |
| `DTE1001` | `await` is only valid inside async functions | Mark the function `async`, or remove `await`. |
| `DTE1002` | cannot await a non-future value | Only values of kind `future<T>` can be awaited. |

## Module Errors

*Imports, resolution, and module system*

| Code | Title | Help |
|------|-------|------|
| `DME0001` | import cycle | break the cycle by removing or refactoring one of the imports |
| `DME0002` | cannot find module | verify the dotted path and ensure the file exists under a search root (project dir or compiler/lib) |
| `DME0003` | invalid import path | Use dotted names like 'foo.bar'; no spaces or invalid characters. |
| `DME0004` | failed to read file | Check file permissions and path casing. |
| `DME0005` | parse failed during resolution | Fix parser errors in the imported file. |
| `DME0006` | bad entry path | Pass a valid .desi file; avoid directories or missing paths. |
| `DME0007` | entry not found | Make sure the entry file exists and isn’t a directory. |
| `DME0008` | circular import detected | Circular imports are not allowed. Refactor to break the dependency cycle. |
| `DME0009` | reserved namespace | 'std' is reserved for the standard library. Rename your module. |
| `DME0010` | module shadows stdlib | Local module shadows a stdlib module. Rename it or use 'from std.X import ...' for stdlib. |
| `DME0011` | ambiguous module | Both a file (X.desi) and package (X/__mod.desi) exist. Remove one to resolve ambiguity. |
| `DME0099` | internal error | This indicates a compiler bug; please report with a repro. |
| `DMW0001` | duplicate import | remove the repeated import; importing the same module multiple times has no effect |
| `DMW0002` | module imports itself | Remove the self import; a module can reference its own symbols directly. |
| `DMW0003` | import alias conflict | Give different aliases (e.g., 'import foo as f1') or adjust 'from' items to avoid local name collisions. |
| `DMW0004` | unused import | Remove the import or reference a symbol from it. |
| `DMW0005` | unused imported name | Remove the unused item from the 'from' list. |

## Borrow Errors

*Move semantics and borrow checking*

| Code | Title | Help |
|------|-------|------|
| `DBR0001` | cannot hold 'inout' borrow across 'await' | Move the 'await' earlier or avoid 'inout' in async callees. Phase-2 will refine this with data-flow for narrower borr... |
| `DBR0002` | inout argument must be a mutable lvalue | Pass a mutable lvalue such as an identifier, field access, or index expression. |
| `DBR0003` | inout cannot alias with other arguments | Pass distinct base variables/receivers, or avoid combining inout with ref to the same base in a single call. |
| `DBR0004` | use after move | Bind to a new name or avoid moving the value before using it again. |
| `DBR0005` | ref argument must be an lvalue | Pass an lvalue such as an identifier, field access, or index expression. |

## Concurrency Errors

*Guards, arenas, and structured concurrency*

| Code | Title | Help |
|------|-------|------|
| `DORM0001` | unknown ORM field | The field name does not match any field registered in a db.model() definition. Check for typos in the field name. |
| `DSY0001` | TaskGroup must be used with 'using' guard | TaskGroup requires RAII for proper cleanup. Use 'using tg = sync.TaskGroup():' to ensure resources are freed. |
| `DSY0002` | Sender should be used with 'using' guard | Consider using 'using tx = ch.sender():' to ensure the sender is properly dropped. |
| `DSY0003` | Receiver should be used with 'using' guard | Consider using 'using rx = ch.receiver():' to ensure the receiver is properly dropped. |
| `DSY0004` | ReadGuard should be used with 'using' guard | Consider using 'using guard = rw.read():' to ensure the read lock is properly released. |
| `DSY0005` | WriteGuard should be used with 'using' guard | Consider using 'using guard = rw.write():' to ensure the write lock is properly released. |
| `DSY0010` | type does not satisfy trait bound | The type argument does not implement the required trait. Ensure the type implements the trait or use a compatible type. |

## Class Errors

*Classes, inheritance, and dunder methods*

| Code | Title | Help |
|------|-------|------|
| `DCL0001` | dunder method must be pub | All dunder methods (__new__, __repr__, __eq__, __close__, etc.) must be declared with 'pub'. |
| `DCL0002` | base must be a class | Inheritance requires a class type; structs and other types cannot be used as base classes. |
| `DCL0003` | invalid __close__ signature | __close__ must have signature: __close__(self) -> none |
| `DCL0004` | cannot assign to immutable field | declare the field as 'pub mut field: Type' to allow mutation |
| `DTC0020` | __del__ must have zero parameters | __del__ is a destructor and takes only 'self' (no other parameters) |
| `DTC0021` | __del__ must not have a return type | __del__ is a destructor and must return void (no return type annotation) |

## Call Errors

*Function calls, defaults, and named arguments*

| Code | Title | Help |
|------|-------|------|
| `DCA0001` | unknown named argument | the provided argument name does not match any parameter in the selected overload |
| `DCA0002` | duplicate named argument | remove one of the occurrences or rename the argument |
| `DCA0003` | positional argument after named arguments | once a named argument is used, all subsequent arguments must be named |
| `DDF0001` | default value must be a compile-time constant | use a literal or other compile-time constant expression as the default value. |
| `DDF0002` | inout parameters cannot have default values | remove the default value or change the parameter mode; inout parameters cannot have defaults. |
| `DDF0003` | parameters with defaults must come last | once a parameter has a default value, all following parameters must also have defaults. |
| `DDF0004` | default parameters must be consistent across overloads | overloads of the same function must agree on which parameters have defaults. |

## Collection Errors

*Lists, dicts, sets, and other containers*

| Code | Title | Help |
|------|-------|------|
| `DCO0001` | type has no length | len(x) is defined for strings (and, in M10, for list/dict/set/tuple). Use a supported type or convert appropriately. |
| `DCO0002` | unsupported membership | The 'in' operator is defined for strings (substring membership) in this phase. In M10, it will also support list/set ... |

## Numeric Errors

*Numeric types and overflow*

| Code | Title | Help |
|------|-------|------|
| `DNT0001` | mismatched numeric widths | Use the same-sized types or convert explicitly; mixing widths is not allowed yet. |
| `DNT0002` | signed/unsigned mismatch | Use matching signedness or convert explicitly; implicit signed/unsigned mixing is not allowed. |

## FFI Errors

*Foreign function interface*

| Code | Title | Help |
|------|-------|------|
| `DFI0001` | invalid @extern decorator arguments | Use @extern("C"[, "lib"]). The first argument must be a string specifying the ABI; an optional second string may spec... |
| `DFI0002` | @extern only allowed on public functions | Place @extern on 'pub def' so external users can link against it. |
| `DFI0003` | extern call in safe context | Calls to @extern functions must occur inside an 'unsafe:' block. |
| `DFI0004` | non-FFI-compatible field type in @ffi_struct | @ffi_struct fields must use FFI-compatible types: int, i8-i64, u8-u64, float, f32, f64, bool, cptr, or other @ffi_str... |
| `DFI0005` | @ffi_struct cannot be generic | C-compatible structs cannot have type parameters. Remove the generic parameters. |
| `DFI0006` | out parameter not found in function signature | Each name in out=[...] must match a parameter of the safe extern function. Check spelling or remove the invalid entry. |

## Prelude Errors

*Built-in functions and prelude*

| Code | Title | Help |
|------|-------|------|
| `DPL0001` | cannot shadow builtin | builtins like print, len, str, and bool are always available; use a different name. |

## Project Errors

*desi.mod and project configuration*

| Code | Title | Help |
|------|-------|------|
| `DPM0001` | invalid key/section in manifest |  |
| `DPM0002` | invalid value type in manifest |  |
| `DPM0003` | missing entry point in manifest |  |
| `DPM0004` | invalid or non-existent root directory |  |
| `DPM0005` | invalid diagnostics value |  |
| `DPM0006` | entry file not found |  |
| `DPM9999` | manifest I/O error |  |

## Warnings

*Non-fatal issues and code quality*

| Code | Title | Help |
|------|-------|------|
| `DW0001` | unused variable or parameter | Prefix the name with '_' to silence this warning. |
| `DW0002` | name shadows an outer binding | consider renaming to avoid confusion |
| `DW0004` | unreachable code: statement after return |  |
| `DW0006` | function may fall through without an explicit return |  |
| `DW0008` | global constant should be UPPER_CASE | follow the naming convention: use UPPER_CASE for global constants (e.g., MY_CONST) |

