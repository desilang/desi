# Codegen Cleanup & Post-M7 Follow-ups

Goal: keep generated C minimal and shift correctness checks to the compiler (parser/checker), not the emitter. This reduces noisy C comments, keeps semantics clear, and makes it easier to swap the backend (e.g., LLVM) later.

---

## Immediate (Stage-1) cleanup

### 1) Eliminate C-side “undeclared” warnings
**Today:** codegen warns when assigning to unknown `e.vars` names (e.g., `/* warning: assigning to undeclared u.id; assuming int */`).
**Plan:** move this entirely to the checker.

- Parser already supports dotted LHS (`a.b.c := x`).
- Checker validates base existence, mutability, and field shape (done).
- **Action:** Codegen should:
  - Use `AssignStmt.LHS` (Expr) instead of `Names` (string).
  - Treat `FieldExpr` as a valid lvalue without emitting “undeclared” warnings.

### 2) Temporary variable elision (single assignment)
**Today:** temps are used aggressively. We already fast-path single assignment.
**Plan:** keep temps only when required to preserve evaluation order:
- Parallel assignment (e.g., `a, b := b, a`).
- Future: if LHS and RHS overlap in a multi-target assignment.

Optional heuristic (later): for multi-assignments where no LHS identifiers appear in any RHS, skip temps.

---

## Checker tightening

1) **Decl-before-use on assignment**
   Emit a type error instead of relying on codegen to warn.

2) **Struct locals initialization**
   Track “definite init” for struct locals before field writes (e.g., forbid `u.id := 1` if `u` is uninitialized).

3) **Struct literals**
  - Validate field names against the struct.
  - Optional: allow partial designated init; missing fields get defaults (spec decision).

4) **Nested field access/assign**
   ✓ Implemented: `a.b.c` typing and base var marked `read=true`.
   Add diagnostics for bad hops (e.g., non-struct in the middle).

---

## Backend roadmap

- **C backend (current):** keep it simple, readable enough, and let `-O2` do the obvious cleanups.
- **LLVM (later):** temps become SSA values; optimizations (DCE, inlining, RA) remove noise automatically. Memory/ownership rules remain a front-end responsibility.

---

## Tests to add

- `examples/m7_structs_assign_direct.desi` — verify no temps for single assigns.
- `examples/m7_structs_parallel_swap.desi` — verify temps preserved.
- `examples/m7_structs_dotted_lhs.desi` — no C-side “undeclared” warnings.
- Negative: field on non-struct, unknown field in literal, assign to immutable, etc.

---

## Tracking

- Issue: “Adopt AssignStmt.LHS in C codegen; drop string Names path”
- Issue: “Checker: enforce decl-before-use on assignment (struct field too)”
- Issue: “Definite-init for struct locals before field writes”
