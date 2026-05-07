# Using the REPL

The Desi REPL (`desirepl`) is an interactive environment for exploring the language.
It parses and type-checks each line you enter, giving instant feedback.

---

## Starting the REPL

```bash
desirepl
```

You'll see:

```
desirepl 0.1.0
Type expressions or statements. :quit to exit.

>>>
```

---

## Quick Tour

### Variables and Expressions

```
>>> let x = 42
ok
>>> let name = "Desi"
ok
>>> print(f"Hello, {name}!")
ok
>>> let items = [1, 2, 3]
ok
>>> print(items[0])
ok
```

The REPL prints `ok` when your code is valid. Each line is independently
parsed and type-checked.

### Type Errors

The REPL catches type errors immediately:

```
>>> let x: str = 42
  [DTE0004] type error: cannot assign 'int' to 'str'
>>> let y: int = "hello"
  [DTE0004] type error: cannot assign 'str' to 'int'
```

### Data Structures

```
>>> let nums = [10, 20, 30]
ok
>>> let info = {"name": "Desi", "version": "0.1.0"}
ok
>>> let point = (3.14, 2.71)
ok
```

### Control Flow

Single-line control flow works:

```
>>> if true: print("yes")
ok
>>> for i in range(5): print(i)
ok
```

### Mutable Variables

```
>>> let mut counter = 0
ok
```

### Boolean Expressions

```
>>> let a = true and false
ok
>>> let b = 5 > 3
ok
>>> let c = 10 == 10
ok
```

---

## REPL Commands

| Command | Description |
|---------|-------------|
| `:help` or `:h` | Show available commands |
| `:quit` or `:q` | Exit the REPL |
| `quit` or `exit` | Exit the REPL (convenience) |
| `:ast` | Show the AST for the next input |

### Viewing the AST

The `:ast` command is useful for understanding how Desi parses your code:

```
>>> :ast
(will show AST for next input)
>>> let x = 42
Module("<repl:1>")
    Func __repl_1()
        Block
            Let x = Int(42)
ok
```

---

## Error Recovery

The REPL handles malformed input gracefully — it will never crash:

```
>>> )))
  parse error: unexpected token while parsing expression
>>> [[[
  parse error: expected ]
>>> print(
  parse error: unclosed )
>>> let x = 10
ok
```

You can always continue entering code after an error.

---

## What the REPL Checks

The REPL runs the full Desi type checker on each input, including:

- **Type errors** — mismatched types, undefined variables
- **Borrow errors** — move-after-use, aliasing violations
- **Call errors** — wrong argument count, missing named args
- **Warnings** — unused variables, unreachable code
- **All other diagnostics** — collection errors, numeric overflow, FFI issues

---

## Limitations

The REPL is a **check-only** tool — it validates your code but does not execute it.
This means:

- **No output from `print()`** — the call is type-checked but not run
- **No cross-line variables** — each line is checked independently, so
  `let x = 10` on one line and `print(x)` on the next will report `x` as undefined
- **No function definitions** — `def foo():` inside the REPL won't work
  due to the wrapping mechanism

For running code, use `desic run`:

```bash
# Run a file
desic run hello.desi

# Run a project (uses desi.mod)
desic run
```

---

## See Also

- [First Program](first-program.md) — Write and run your first Desi program
- [desic CLI Reference](../reference/desic-cli.md) — Full command-line reference
- [Editor Setup](editor-setup.md) — Configure your editor for Desi
