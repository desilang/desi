# Compile-Time Macro Introspection — Design Notes (v0.2.0)

**Status**: PLANNED for v0.2.0
**Priority**: High
**Related**: Macro protocol system (`internal/macro/`), Runtime `ast` library, `@perf` advisor

## Summary

Allow users to write macro rules in Desi that execute during compilation with full AST access. This enables user-defined linters, code generators, and compile-time analyzers — without writing Go compiler plugins.

## Rationale

### Why this matters
1. **Attracts Rust/C++ programmers** — Proc macros and `comptime` are headline features in those ecosystems.
2. **Enables community-driven tooling** — Users can build domain-specific analyzers (e.g., SQL injection detection, API contract validation) without modifying the compiler.
3. **Extends the advisor pattern** — The `@perf` Performance Advisor in v0.1.0 is Go-only. In v0.2.0, users should be able to define their own advisor-style rules in Desi.

### Why deferred from v0.1.0
1. **Requires a Desi interpreter embedded in the Go compiler** — 4-6 weeks of work with significant stability risk.
2. **AST API stability pressure** — Node shapes exposed to macros become a public contract. Better to stabilize the AST in v0.1.0 first.
3. **Risk to launch quality** — Interpreter bugs could crash the compiler. Unacceptable for a first release.

## v0.1.0 Foundation (What ships first)

These features land in v0.1.0 and form the foundation for compile-time macros:

1. **Runtime `ast` library** — `import ast` to parse/walk `.desi` files at runtime. Uses the same parser as the compiler. Lets users build linters, formatters, and analysis tools.
2. **Declarative macro protocols** — `@macro` class declarations with properties, chainable/terminal methods, validation rules, and C runtime bindings. No imperative code, but covers 80% of use cases (ORM, `@perf`, `@test`).
3. **`strip_in_release` macro property** — Generic mechanism for dev-only decorators.
4. **`[permissions]` manifest section** — Build audit for sensitive API usage.

## Proposed Implementation (v0.2.0)

### Option A: Tree-Walking Interpreter (Recommended)

Embed a Desi interpreter in the Go compiler that can evaluate a **subset** of Desi at compile time.

**What the interpreter needs to support:**
- ✅ Literals, binary ops, comparisons
- ✅ Variables, `let`, assignment
- ✅ `if`/`else`, `for`, `while`, `return`
- ✅ Function definitions and calls
- ✅ Basic types: `int`, `str`, `bool`, `list`, `dict`
- ✅ AST node types exposed as Desi structs
- ✅ Pattern matching on AST nodes via `is`
- ❌ Does NOT need: file I/O, networking, concurrency, FFI, full OOP

**Estimated effort:** 4-6 weeks

**User-facing API:**
```desi
import ast
import diag

# This runs at COMPILE TIME when someone uses @perf_check
@macro(target="func")
class perf_check:
    def on_check(self, func_ast: ast.FuncDecl):
        for stmt in func_ast.body.stmts:
            if stmt is ast.ForStmt:
                for inner in stmt.body.stmts:
                    if inner is ast.ForStmt:
                        diag.warn("USR0001", inner.span,
                                  "O(n²) nested loop detected")
```

### Option B: Compile-and-Load Plugin (Alternative)

Compile macro code to a shared library (`.so`/`.dylib`) using the existing LLVM pipeline, then load it via `dlopen`.

```
macro.desi → parse → LLVM IR → .so → dlopen → call macro functions
```

**Pros:** Uses existing compiler pipeline, macros run at native speed.
**Cons:** Platform-specific, more complex build, slower macro iteration cycle.
**Estimated effort:** 3-4 weeks

### Option C: Declarative Rule DSL (Fallback)

Extend `LoadMacrosFromModule()` with a pattern-matching DSL:

```desi
@macro(target="func")
class perf_check:
    rules = [
        {
            "pattern": "for > for",
            "code": "USR0001",
            "message": "O(n²) nested loop"
        }
    ]
```

**Pros:** Lowest risk, builds on existing infrastructure.
**Cons:** Not Turing-complete, limited expressiveness.
**Estimated effort:** 2-3 weeks

## Design Considerations

### AST Type Stability

The AST types exposed to user macros must have a stable public API:

```desi
# ast module — stable public types
class FuncDecl:
    pub name: str
    pub params: list[Param]
    pub body: Block
    pub decorators: list[Decorator]
    pub return_type: TypeExpr
    pub span: Span

class ForStmt:
    pub target: str
    pub iter: Expr
    pub body: Block
    pub span: Span

# ... etc for all statement/expression types
```

**Rule:** Internal compiler AST nodes (`ast.FuncDecl` in Go) are the source of truth. The Desi `ast` module provides a stable projection of these types. Internal node shapes can change without breaking user macros as long as the projection is maintained.

### Sandboxing

Compile-time code runs inside the compiler process. It must be sandboxed:

- **No file I/O** — Macros cannot read or write files.
- **No networking** — Macros cannot make network requests.
- **No process spawning** — Macros cannot execute shell commands.
- **Execution limits** — Maximum instruction count per macro invocation to prevent infinite loops.
- **Memory limits** — Maximum heap allocation per macro invocation.

### Diagnostic API

Macros emit diagnostics through a structured API:

```desi
import diag

# Emit a warning
diag.warn("USR0001", span, "message")

# Emit an error (blocks compilation)
diag.error("USR0002", span, "message")

# Emit a note (informational)
diag.note("USR0003", span, "message")

# Add a help suggestion
diag.help(span, "Consider using X instead")
```

User diagnostic codes use the `USR` prefix to distinguish from compiler diagnostics (`DPR`, `DTE`, etc.).

### Integration with Existing Macro System

The compile-time evaluator hooks into the existing `MacroProtocol` lifecycle:

```
OnCollect (AST phase)  → declarative config extraction (existing)
OnCheck   (type phase) → compile-time Desi code runs here (NEW in v0.2.0)
OnLower   (HIR phase)  → C runtime wiring (existing)
```

No changes to the protocol interface — only the `OnCheck` implementation changes from "Go callback" to "interpreted Desi code."

## Prior Art

| Language | Mechanism | Compile-time execution |
|----------|-----------|----------------------|
| Rust | proc macros | Separate crate compiled to `.so`, operates on `TokenStream` |
| Zig | `comptime` | Full Zig interpreter embedded in compiler |
| Elixir | `quote`/`unquote` | BEAM VM always available at compile time |
| Nim | `macro` | Nim VM embedded in compiler |
| Julia | `@macro` | Runtime available at compile time |

## Timeline

- **v0.1.0**: Ship runtime `ast` library + declarative macros + `[permissions]` audit
- **v0.2.0**: Ship compile-time interpreter (Option A) with sandboxing
- **v0.3.0**: Stabilize AST type API, remove `@experimental` flag

## Open Questions

1. **Should compile-time macros be opt-in per project?** E.g., `[build] enable_comptime = true` in `desi.mod`?
2. **Should we support `comptime` blocks (Zig-style) in addition to macro classes?**
3. **What's the right error recovery when a macro crashes?** Skip the macro and continue? Abort the file? Abort the build?
