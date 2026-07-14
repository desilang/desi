# Drops Implementation (Hybrid MM, Phases 1–2)

How Desi actually frees heap values today — what gets dropped, where the
decisions are made, and the invariants to preserve when touching the
lowerer, the LLVM backend, or the collection runtime. The long-term
design (arenas, rc/arc, optional GC island) lives in
[memory-management.md](memory-management.md).

## The pipeline

```
lowerer (scope tracking)          backend (emission)              runtime (C)
────────────────────────          ────────────────────            ─────────────
scope.locals / types      ──►     hir.Drop{Val, Type}     ──►     list_free /
scope.moved / borrowed            emit_func.go case               set_free /
emitScopeDrops()                  *hir.Drop →                     dict_free /
                                  emitDropForType()               free()
                                  (drop_impl.go)
```

1. **Lowerer** (`compiler/internal/lower`): every `let` registers the local
   and its checked type in the current `scope`. At scope exit
   (`emitScopeDrops` in `hir_lower.go`) each local that is not `moved`,
   `borrowed`, or arena-owned gets a typed `hir.Drop`. Returns run
   `emitAllDefersAndDrops()` *after* evaluating the return value and mark
   a returned ident as moved in all scopes.
2. **Backend** (`compiler/internal/backend/llvm/emit_func.go`, `*hir.Drop`
   case): resolves the SSA alias or loads from the alloca — this
   distinction caused the crash that originally disabled drops in commit
   `fad63ec6`; always free the *heap pointer*, never the stack slot. Then
   dispatch by type: classes get the `__del__` destructor chain;
   `List`/`Set`/`Dict`/`Enum`/`Struct` go through `emitDropForType`
   (`drop_impl.go`), which null-checks and emits the runtime free calls.
3. **Runtime** (`compiler/runtime/list.c`): float elements (type tag 3)
   are the one element kind the list owns — 8-byte boxes malloc'd by
   codegen. `list_free`/`list_clear` free them; every cross-list element
   flow (`copy`, `slice`, `slice_step`, `extend`, `filter`, the iterator
   collectors) clones the box via `list_clone_elem` so no two lists ever
   share one. Int/bool elements are stored inline; str elements are not
   owned (they may alias literals).

## What is dropped

| Kind | Mechanism |
|---|---|
| `list` / `set` / `dict` locals | `list_free` / `set_free` / `dict_free` |
| Enum locals | free payload box (all variants) + recursive heap-field drop + free wrapper |
| Struct locals | recursive heap-field drop + free (offsets MUST match the aligned construction layout — `field_offset.go`) |
| Class instances | `__del__` chain (child→parent) + free |
| `rc`/`arc` handles | `DecRef` → `__rc_dec` |

**Not dropped (deliberately):** `str` locals (may alias string-literal
globals — freeing one crashes; str ownership is phase 3), function
parameters (caller owns), match arm bindings (aliases, see below), and
temps other than tracked string temps.

## Ownership rules in the lowerer ("leak rather than crash")

The lowerer cannot see across function boundaries, so anything with
unclear ownership is conservatively marked and will NOT be freed:

- **moved**: `let y = x`, `return x`, `xs.append(x)`, `xs[i] = x`, set
  literals/`add`, dict-literal heap values, collection/enum/struct idents
  passed to *user-defined* calls (the callee may retain them), and a
  matched enum local whose arms bind payloads (bindings alias the payload
  and may escape the match).
- **borrowed**: `let x = xs[i]` (IndexExpr), `let x = obj.field`
  (FieldExpr — the owner's recursive drop frees fields), `let n = res?`
  (payload extraction), tuple-iteration loop vars, mutable dict-items
  values.
- **nonOwnedTemps** (`markNonOwnedResult` in `lower_call.go`):
  Option/Result method results (`map`, `and_then`, `or_else`, …) are
  scalars, payload aliases, or **stack-allocated** slots — binding one
  must never produce a drop.

Every rule errs toward leaking. A missed `moved` mark shows up as a
double-free / heap corruption (0xC0000374 on Windows, often silent
corruption on macOS); an over-aggressive mark only leaks.

## Gotchas that already bit us

- **Stack-allocated enum results**: Option/Result methods alloca their
  result — freeing it crashed examples 319–323 until `nonOwnedTemps`.
- **Payload boxes leak unless freed in *every* variant path** of
  `emitEnumDrop`, not only heap-payload variants.
- **Unit variants must null the payload slot with a full 8-byte store**;
  a 4-byte `i32 0` leaves garbage that the null-check then frees.
- **Struct drop offsets**: `calculateFieldOffset` mirrors the lowerer's
  aligned layout (`getSize`/`getAlign`). Summing raw sizes reads the
  wrong slot — `{id: int, status: Enum}` stores the pointer at offset 8,
  not 4.
- **`using` classification**: an unmatched type falls through to the
  arena fallback, and `__arena_destroy` on a non-arena corrupts the heap
  (the TaskGroup bug). Add an explicit case for any new RAII type.

## Verifying changes

- Full example suite on every platform (`test_examples.sh` /
  `test_examples.ps1`) — double-frees crash loudly, especially on the
  Windows heap.
- Churn probes for leaks: a `while` loop allocating millions of
  lists/enums must hold a flat working set (~3 MB), not grow.
- ASAN works on Windows too: `clang -fsanitize=address program.ll
  build/libdesi.lib -o t.exe -lws2_32` (copy
  `clang_rt.asan_dynamic-x86_64.dll` from the LLVM tree next to the exe).
  `bad-free` / `access-violation` reports point at the exact drop.

## Future (phase 3+)

- `str` ownership (needs heap-vs-literal discrimination)
- Function-scoped arenas + escape analysis
  (`docs/roadmap/todo/hybrid_memory_management.md`)
- Freeing heap elements stored *inside* collections (today the
  container's pointer array is freed; enum/struct elements it holds leak)
