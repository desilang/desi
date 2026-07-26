# desirepl — REPL Internals & Implementation Guide

> **For Contributors**: architecture, the session model, how an input is
> classified, and what is deliberately left out.

User-facing documentation: `book/docs/getting-started/repl.md`.

---

## Architecture

Desi is compiled and has no interpreter, so the REPL does not evaluate
anything itself. It keeps a **session** of everything accepted so far,
assembles a complete program from it plus the new input, and runs that
program through `desic run`:

```
User input → classify → assemble full program → parse + check (in-process)
           → desic run (compile + link + execute) → show new output
```

Source: `compiler/cmd/desirepl/main.go`.

### Why compile instead of interpreting

`compiler/internal/eval` exists to evaluate **compile-time macros**. It
handles a handful of node kinds and 7 builtins (`print` plus `ast_*`
helpers) — no user-defined functions, no `len()`, no control flow. A REPL
built on it would quietly diverge from the language: expressions would
behave differently at the prompt than in a real program.

Driving the real compiler costs roughly half a second per input and
guarantees the prompt agrees with a build. That trade is deliberate.

`desic` is located next to the `desirepl` binary (how releases ship), with
`PATH` as a fallback — see `findDesic`.

---

## The session model

```go
type session struct {
    imports  []string // `import x` — must precede declarations
    decls    []string // def/class/enum/struct/trait — top level
    stmts    []string // let/assignment/if/... — inside main()
    baseline string   // stdout of the session as it currently stands
}
```

`assemble` emits, in order: imports, declarations, then `def main() -> int:`
containing the retained statements, the new input, and `return 0`.

**What is retained:** imports, declarations, and statements. **What is not:**
expressions. An expression has already been displayed, and keeping it would
repeat any side effect on every subsequent input.

### baseline

Retained statements are replayed with each input, so their output would be
printed again every time. After each accepted input the run's full stdout is
stored as `baseline`; the next run prints only the part beyond that prefix
(`emitNew`). Side effects inside retained statements still re-execute — that
is inherent to replay and is documented for users.

---

## Classifying an input

`classify` handles the structural cases by keyword: `import`/`from` →
import, `def`/`class`/`enum`/`struct`/`trait`/`impl`/`macro`/`type` →
declaration. Multi-line input is a block, never an expression.

Everything else is decided by **asking the checker**, not by guessing:

1. Assemble the program with the input as a plain statement.
2. Parse and check in-process (`parseAndCheck`) — no compile, so this is cheap.
3. `trailingExprType` finds the statement `assemble` placed just before
   main's `return 0` and looks up `info.Types[expr]`.
4. If that type exists and is not `none`, re-assemble with the input wrapped
   in `print(...)` so the value is displayed.

> **Do not** revert this to the earlier heuristic of "try `print(<input>)`
> and see whether it type checks". `print` accepts a `none`-typed argument at
> the checker level, so `print(x)` became `print(print(x))`, which passed
> checking and then emitted IR referencing a value that was never defined —
> the user saw a raw LLVM `use of undefined value` dump.

---

## Reading blocks

`readLogical` returns one logical input. A line whose trimmed text ends in
`:` opens a block: subsequent lines are read (with a `... ` prompt) until a
blank line. That is what makes multi-line `def`s enterable. The user's own
indentation is preserved; `indent` adds one tab when a fragment is embedded
into `main()`.

---

## Failure handling

An input that fails to parse, fails checking, or fails to compile/run is
reported and **dropped** — `accept` is never called for it, so the session
stays valid and the next input still works. Compile and runtime failures are
surfaced from `desic`'s stderr.

Diagnostics are rendered by `renderDiag`, which maps `d.Domain` to a readable
prefix (`type` → "type error", `borrow` → "borrow error", …). All domains are
shown; warnings do not invalidate an input.

---

## REPL Commands

| Command | Behavior |
|---------|----------|
| `:help`, `:h` | Command list plus a note that retained statements re-run |
| `:session` | Print the accumulated imports, decls, and statements |
| `:reset` | Clear the session |
| `:ast` | Pretty-print the AST of the next input |
| `:quit`, `:q`, `quit`, `exit` | Exit |

---

## Testing

There are no automated REPL tests yet. Drive it with piped stdin:

```powershell
"1 + 2`nlet x = 10`nx * 4`n:quit" | Out-File -Encoding utf8 in.txt
.\bin\desirepl.exe < in.txt
```

Cases worth covering when adding tests:

- Expression displays a value (`1 + 2` → `3`)
- Binding persists (`let x = 10` then `x * 4` → `40`)
- `print(...)` prints once and is not double-wrapped
- Multi-line `def` then a call to it
- A type error leaves the session usable
- `:reset` clears state

---

## Known Limitations

- **No readline**: no history, no arrow-key line editing. Input is read with
  `bufio.Scanner`. This is the most noticeable gap.
- **No completion or highlighting.**
- **~0.5s per input** — a full compile and link each time.
- **Retained statements re-run**, so side effects in a kept statement repeat.

---

## Future Work

1. **readline** (history, line editing) — the highest-value improvement.
   Needs a dependency or a hand-rolled terminal mode; the Scanner loop in
   `main` is the only thing that has to change.
2. **Avoid recompiling the whole session** — cache the compiled session and
   link only the new input, or keep a persistent process. Reduces latency and
   removes the replay side-effect wart.
3. **`:load <file>`** to seed a session from a file.
4. **`:type <expr>`** — `trailingExprType` already computes exactly this.
