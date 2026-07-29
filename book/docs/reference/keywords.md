# Keywords

Reserved keywords in Desi. A reserved word cannot be used as a variable,
parameter, or function name.

This list matches `keywordToken` in `compiler/internal/lex/scanner.go`.

---

## Declaration Keywords

| Keyword | Description |
|---------|-------------|
| `let` | Bind a variable — immutable unless followed by `mut` |
| `mut` | Mark a binding or field mutable (`let mut x = 1`) |
| `const` | Compile-time constant |
| `static` | Belongs to the class rather than to any one instance |
| `def` | Function definition |
| `lambda` | Anonymous function expression |
| `class` | Class definition |
| `struct` | Struct definition |
| `enum` | Enum definition |
| `trait` | Trait definition |
| `impl` | Trait implementation |
| `type` | Type alias |
| `pub` | Public visibility |

!!! note "There is no `var`"
    Mutability is spelled `let mut`, not `var`.

---

## Control Flow Keywords

| Keyword | Description |
|---------|-------------|
| `if` | Conditional |
| `elif` | Else if |
| `else` | Else branch |
| `for` | For loop |
| `while` | While loop |
| `match` | Pattern matching |
| `break` | Break loop |
| `continue` | Continue loop |
| `return` | Return value |
| `pass` | No-op statement |
| `defer` | Run a statement when the scope exits, in reverse order |
| `using` | Resource management — scope-bound cleanup |

!!! note "There is no `case`"
    Match arms are written as `Pattern: body` directly, with no `case` prefix.

---

## Error Handling Keywords

| Keyword | Description |
|---------|-------------|
| `try` | Begin a block whose exceptions are handled |
| `except` | Handle a raised exception |
| `finally` | Run whether or not an exception was raised |
| `raise` | Raise an exception |
| `assert` | Check that a condition holds at runtime |

---

## Concurrency Keywords

| Keyword | Description |
|---------|-------------|
| `async` | Async function |
| `await` | Await an async result |
| `spawn` | Start a concurrent task |
| `select` | Wait on multiple channel operations |

---

## Boolean and Value Keywords

| Keyword | Description |
|---------|-------------|
| `true` | Boolean true |
| `false` | Boolean false |
| `none` | The absence of a value |
| `and` | Logical AND |
| `or` | Logical OR |
| `not` | Logical NOT |

---

## Parameter Modes

| Keyword | Description |
|---------|-------------|
| `ref` | Pass by reference, read-only |
| `inout` | Pass by reference, and the caller sees writes |

---

## Other Keywords

| Keyword | Description |
|---------|-------------|
| `in` | Membership and iteration |
| `is` | Type and variant check |
| `unsafe` | Permit operations that bypass Desi's guarantees, such as `@extern` calls |
| `import` | Module import |
| `from` | Import from a module |
| `as` | Alias |

---

## Not reserved

`self` and `cls` are ordinary parameter names, not keywords. They are
meaningful by position — the first parameter of a method — and the compiler
supplies `self` for you if you leave it off. Either may be used as a normal
variable name, though doing so is confusing and best avoided.

`__new__` is likewise a name rather than a keyword. See
[Classes](../language/classes.md) for the constructor it declares.
