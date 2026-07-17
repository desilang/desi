# Drops Implementation (Hybrid MM, Phases 1–3)

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
scope.tempDrops                   *hir.Drop →                     dict_free /
emitScopeDrops()                  emitDropForType()               free()
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
   `Str` drops are freed **only for `hir.Temp` values** (see phase 3).
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
| `rc`/`arc` handles | `DecRef` → `__rc_dec` (atomic; weak-aware) |
| `weak` handles | `__weak_dec` |
| **String temporaries** (phase 3) | plain `free` at scope end, temps only |

**Not dropped (deliberately):** `str` *locals* (may alias string-literal
globals — freeing one crashes; full str ownership needs the heap-vs-literal
ABI decision), function parameters (caller owns), match arm bindings
(aliases, see below).

## Phase 3: string temporary ownership (`temp_tracking.go`)

Strings are the one heap type where a pointer may reference an
**immutable global** (a string literal) instead of the heap, so blanket
freeing is impossible. Phase 3 draws the ownership boundary at
*temporaries*:

- **Tracked (freed if unconsumed)** — results of operations that provably
  malloc: `string_concat` (the `+` operator), `__desi_sprintf`
  (f-strings), `str(int/float/bool)`, `s.replace()`, `list<str>.join()`,
  the `float_to_str` intermediates inside f-strings, and the
  `list/dict/set_to_str` / `__desi_default_repr` conversions in `print`.
  Registered via `addTempDrop`; emitted as `Drop{hir.Temp, types.Str}`;
  the backend frees exactly this shape and ignores `Drop{hir.Var, Str}`.
- **Consumption transfers ownership** (`consumeTemp`) — every construct
  that stores the raw pointer somewhere longer-lived removes the temp
  from tracking: `let` bindings, assignments (incl. fields, statics,
  `lst[i]=`), `return`, call arguments (the callee may retain), list
  `append`/`insert`/`set`/literals, set `add`/literals, dict **values**
  (literals, `insert`, `setdefault`), tuple/struct/class construction,
  channel `send`. Dict **keys** are *not* consumed — `dict_insert`
  strdups keys, so the temp key is safely freed.
- **`str(str)` is an identity bitcast** (same pointer) — never tracked.
  `bool_to_cstring` returns static strings — never tracked. User dunders
  (`__str__`/`__repr__`/`to_str`) may return literals — never tracked.
- **Suppression** (`suppressTempDrops`) — comprehensions and match
  expressions lower sub-expressions into conditionally-executed blocks.
  A temp defined there does not dominate the scope end, so a scope-end
  `free` would be invalid LLVM IR (dominance violation = compile
  failure). Registration is suppressed for their dynamic extent; those
  temps leak instead. `and`/`or` and ternary (`IfExpr`) lower eagerly
  (BinaryOp / Select — no blocks), so they need no suppression.
- **Drops emit per exit path**: `emitTempDrops` runs from
  `emitScopeDrops`, which executes for the normal scope end *and* each
  early-return path. Like `scope.locals`, the set must not be cleared
  between emissions — each runtime execution takes exactly one path.

### f-strings: no alloca

F-strings compile to one call: `__desi_sprintf(fmt, ...) -> char*`
(`builtins.c`, two-pass `vsnprintf`). The previous lowering alloca'd an
out-parameter slot per evaluation — an alloca inside a loop body is only
reclaimed on function return, so ~64k loop iterations of `print(f"…")`
overflowed the stack. Any new lowering that needs scratch space inside an
expression must NOT emit `hir.Alloca` at the expression site for the same
reason. (`__desi_sprintf` is registered in `abi/abi.go` for the ARM64
stack-based-varargs i32→i64 promotion, like `asprintf`/`printf`.)

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

Every rule errs toward leaking. A missed `moved` mark or a missed
`consumeTemp` at a pointer-storing site shows up as a use-after-free or
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
- **`isHeapType` in `drop_impl.go` must NOT include `Str`**: it drives
  struct/enum *field* recursion, and string fields may alias literals.
  String freeing happens only through the temp path in `emit_func.go`.
- **Allocas inside loop bodies** (the f-string stack overflow, above).

## Verifying changes

- Full example suite on every platform (`test_examples.sh` /
  `test_examples.ps1`) — double-frees crash loudly, especially on the
  Windows heap.
- Churn probes for leaks: a `while` loop allocating millions of
  lists/enums/string-temps must hold a flat working set (~3 MB), not
  grow. For strings: `print("a" + str(i))` + `print(f"n: {i}")` at 500k
  iterations peaks under 3 MB.
- ASAN works on Windows too: `clang -fsanitize=address program.ll
  build/libdesi.lib -o t.exe -lws2_32` (copy
  `clang_rt.asan_dynamic-x86_64.dll` from the LLVM tree next to the exe).
  `bad-free` / `access-violation` reports point at the exact drop.

## Future (phase 4+)

- Full `str` local ownership: requires making every owned string
  heap-allocated (strdup literals at owning-binding sites and at literal
  returns) — an ABI-level decision, then locals can drop like
  collections.
- Collection string-element ownership: strdup-on-insert +
  free-on-`list_free` (the float-box model, extended to tag 1) with an
  adopting `list_append_owned` for runtime producers like `split`.
- Freeing the old value on mutable-string reassignment (today it leaks:
  the accumulator pattern `s := s + "x"` keeps only the final value
  alive).
- Function-scoped arenas + escape analysis
  (`docs/roadmap/todo/hybrid_memory_management.md`)
- Freeing heap elements stored *inside* collections (today the
  container's pointer array is freed; enum/struct elements it holds leak)
