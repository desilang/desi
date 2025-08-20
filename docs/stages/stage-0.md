# Stage-0 Scope

The Stage-0 toolchain is a usable compiler front-end in Go that emits portable C, plus a minimal C runtime. It must be sufficient to begin re-implementing parts of the compiler in Desi.

## Language surface (minimum)
- **Bindings**: `let` (immutable), `let mut` (mutable)
- **Init vs assign**: `=` for initialization, `:=` for reassignment
- **Functions**: explicit param and return types; expression-valued bodies allowed
- **Types**:
  - Primitives: `bool, i32, i64, u32, u64, f64, u8, str`
  - Collections: `[]T` (slice), `Vec[T]` (growable)
  - ADTs: `struct`, `enum` (tagged unions with payloads)
  - Option/Result: `Option[T]`, `Result[T,E]`
  - **Generics**: monomorphization for `Vec`, `Option`, `Result`, and small utilities used by the compiler
- **Control flow**: `if / elif / else`, `while`, `for … in …`, `return`
- **Pattern matching**: `match` with `_` catch-all (exhaustiveness checks may be relaxed in Stage-0)
- **Errors**: `?` operator for `Result` propagation; `panic` for bugs
- **Modules**: `package` and `import` basics (single-file packages acceptable to start)
- **Immutability first**: moves by default; scalars are `Copy`

## Concurrency (Stage-0 subset)
- **Channels**: `chan[T]`, send/receive with optional timeout
- **spawn**: start lightweight tasks (initial impl may use OS threads or a simple pool)
- **Actors**: library atop channels; supervision may be shallow initially

## Runtime requirements
- ARC for heap objects (strings, vectors, closures)
- String type: UTF-8, immutable; sliceable (fat pointer `{ptr,len}`)
- Vec: `{data, len, cap}` with growth policy
- Deterministic cleanup via `defer` (lowered in codegen)

## Tooling
- `desic version`, `desic help`
- `desic build <file.desi>` → emit C and shell out to `clang` (or platform C compiler) + link runtime
- `desic run <file.desi>` (optional convenience)
- Formatter `desi fmt` may be a no-op placeholder initially

## Out of scope for Stage-0
- Traits/typeclasses, interfaces
- Macros
- Full borrow checker (we rely on moves + ARC)
- Exceptions/stack unwinding (use `Result`)
- Package manager

## Examples (must compile in Stage-0)
```desi
let x = 10
let mut y = 0
y := y + x

enum Op: Add | Sub | Mul | Div

def apply[T](x: T, f: (T)->T) -> T:
  f(x)

match Op.Add:
  Add => 1
  _   => 0
```

## Deliverables checklist

* [ ] Lexer (indentation aware), tokens
* [ ] Parser for decls/exprs/statements
* [ ] AST nodes + diagnostics with source spans
* [ ] Type checker (names, simple inference, generics mono)
* [ ] Desi-IR + ARC insertion + basic escape analysis
* [ ] C emitter + runtime (ARC/str/vec/chan)
* [ ] `desic build` CLI command
* [ ] Golden tests for lexer/parser/codegen
