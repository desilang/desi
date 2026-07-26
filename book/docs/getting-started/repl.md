# Using the REPL

`desirepl` is a small interactive prompt for trying out Desi — expressions,
a few bindings, a short function. It is deliberately simple in 0.1.0: it is
a scratchpad, not a development environment. See
[Limitations](#limitations) before you settle in.

Desi is a compiled language, so there is no interpreter behind the prompt.
The REPL keeps track of what you have entered, assembles a real program from
it, and compiles and runs it with `desic`. That means the REPL can never
disagree with a real build — the trade-off is roughly half a second per
input.

---

## Starting the REPL

```bash
desirepl
```

```
desirepl 0.1.0
Type expressions or statements. :help for commands, :quit to exit.

>>>
```

---

## Quick Tour

### Expressions show their value

```
>>> 1 + 2
3
>>> 5 > 3
true
```

### Bindings persist across lines

```
>>> let x = 10
>>> x * 4
40
>>> let name = "Desi"
>>> print(f"Hello, {name}!")
Hello, Desi!
```

### Collections

```
>>> let nums = [10, 20, 30]
>>> nums[0]
10
>>> len(nums)
3
>>> let info = {"lang": "desi"}
>>> info["lang"]
desi
```

### Functions

A line ending in `:` starts a block. Keep typing, then finish with a blank
line:

```
>>> def double(n: int) -> int:
...     n * 2
...
>>> double(21)
42
```

### Errors do not end the session

Type errors are reported and the offending input is discarded — everything
you entered before it is still there:

```
>>> let x: str = 42
  [DTE0004] type error: cannot assign 'int' to 'str'
>>> 1 + 1
2
```

---

## REPL Commands

| Command | Description |
|---------|-------------|
| `:help` or `:h` | Show available commands |
| `:session` | Show everything currently in the session |
| `:reset` | Clear the session |
| `:ast` | Show the AST for the next input |
| `:quit` or `:q` | Exit the REPL |
| `quit` or `exit` | Exit the REPL (convenience) |

### Viewing the AST

```
>>> :ast
(will show AST for next input)
>>> let z = 42
Module("<repl>")
	Func __repl()
		Block
			Let z = Int(42)
```

---

## How it works

Each input is classified and stored:

- **Imports and declarations** (`import`, `def`, `class`, `enum`, …) are kept
  and placed at the top of the generated program.
- **Statements** (`let`, assignments, `if`/`while` blocks) are kept and
  replayed inside `main()`.
- **Expressions** are wrapped in `print(...)` so you see the value, and are
  *not* kept — they have already been shown.

Because retained statements are replayed with every later input, a statement
with a side effect runs again each time. Output you have already seen is not
reprinted, but something like `let f = open_file(...)` in the session will be
re-executed. Use `:reset` to start over.

---

## Limitations

`desirepl` is intentionally minimal in 0.1.0. It is **not** an IDE like
Python's IDLE, and it is not as capable as `python`'s own REPL:

- **No command history or line editing.** Arrow keys do not recall previous
  input — there is no readline support yet. This is the roughest edge.
- **No tab completion and no syntax highlighting.**
- **No editor window, no debugger, no inspector.**
- **About half a second per input**, because each one is compiled and run.
- **Retained statements re-run**, as described above.

For anything beyond a quick experiment, write a file and run it:

```bash
# Run a file
desic run hello.desi

# Run a project (uses desi.mod)
desic run
```

The REPL is expected to grow — history and line editing first. Its current
scope is deliberately narrow rather than half-finished in a way that misleads
you about what it can do.

---

## See Also

- [First Program](first-program.md) — Write and run your first Desi program
- [desic CLI Reference](../reference/desic-cli.md) — Full command-line reference
- [IDE Setup](ide-setup.md) — Configure your editor for Desi
