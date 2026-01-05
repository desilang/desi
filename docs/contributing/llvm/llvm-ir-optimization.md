# Desi Compiler — LLVM IR Optimization Guide

> **Scope & intent.** This document specifies how the Desi compiler **should** produce LLVM IR that is easy for LLVM to optimize. It is written for contributors who already know basic compiler architecture and LLVM, and for those learning the system via Desi. The guidance is **language- and backend-centric**, not “Go-specific”; the implementation language of the compiler is irrelevant to the quality of generated code.
> Wherever you see “will/should have,” treat it as a design requirement for Desi’s codegen and runtime.

---

## 0) Core principles

1. **Semantics first, then speed.** Correct, information-rich IR is the single highest leverage. LLVM’s middle-end and LTO will do the heavy lifting if the IR exposes intent.
2. **Emit idiomatic SSA.** Prefer value form over memory; keep address-taken locals rare and scoped.
3. **Tell the optimizer what is true.** Attributes, metadata, and intrinsics (e.g., `noalias`, `nonnull`, TBAA, `llvm.assume`, loop hints, profile weights) are free performance.
4. **Don’t pre-optimize in the frontend.** Desi’s frontend should not re-implement constant folding, CSE, LICM, vectorization, etc. LLVM already does these better.
5. **Minimize abstraction penalty.** Lower constructs (match, comprehensions, async) to shapes LLVM recognizes (switches/jump tables, canonical loops, straight-line fast paths).

---

## 1) Targeting & pipelines

Desi’s toolchain **will** expose standard optimization controls and feed LLVM the target information needed for high-quality code.

### 1.1 Driver defaults & flags

* Default to **`-O2`** (balanced speed/compile time). Support `-O0, -O1, -O3, -Os, -Oz`.
* **CPU tuning:** `--cpu=native` and `--cpu=<name>` map to LLVM function/module attributes (`"target-cpu"`, `"target-features"`).
* **LTO:** `--lto=thin|full|off` (default: `thin` for release).
* **PGO:** `--pgo=gen` and `--pgo=use:<profdata>`.
* **Linker:** prefer **lld** for faster, more consistent LTO/ThinLTO.
* **Relocation model:** `-fPIC` for shared libraries; PIE for executables by default on modern platforms.

### 1.2 Pass pipeline (new PM)

* Instantiate `PassBuilder` and use **`buildPerModuleDefaultPipeline(OptLevel)`**.
* Provide **TargetLibraryInfo** so **libcall simplification** and vectorizers know the target’s libc and math model.
* Respect `-g` and **optnone** for debug builds; keep a debug-friendly `-O1 -g` preset.

### 1.3 Cross-TU optimization

* **ThinLTO** is the default trade-off: cross-module inlining, function attribute inference, dead-stripping, and constant prop with minimal wall-time.
* Mark internal helpers with **internal/linkonce_odr** as appropriate to unlock inlining and specialization without exporting symbols.

### 1.4 Optimization diagnostics

* Enable **optimization remarks** (`-pass-remarks` & `-pass-remarks-analysis`) behind a developer flag. Desi’s build output **should** surface why vectorization/inlining did or didn’t happen.

---

## 2) Language semantics that unlock optimization

These are source-level choices that massively influence LLVM’s opportunities. Desi **should** commit to them clearly and encode them in the IR.

### 2.1 Integer overflow

* Decide and document: **UB on signed overflow** (C-like) **vs** **defined wraparound** (Rust-like).

  * If **UB**: set `nsw`/`nuw` where valid; huge scalar and loop opts unlock.
  * If **defined wrap**: do **not** set those flags; use explicit wrapping intrinsics where desired.
* For division by zero semantics, be explicit; it affects `exact` flags and transforms.

### 2.2 Floating-point

* Default **precise FP** (no fast-math flags). Expose a flag like `--fast-math` to set LLVM’s fast-math on FP ops.
* Avoid mixing precise and fast operations in one function unless intentional.

### 2.3 Aliasing & ownership

* If Desi’s borrowing model guarantees “unique mutable” aliasing, encode with:

  * **`noalias`** on function parameters representing unique borrows/owned temporaries.
  * **TBAA** hierarchy reflecting Desi types (see §4.4).
* Clearly forbid “wild” type punning in safe code; keep it **`unsafe`** so TBAA remains valid.

### 2.4 Exceptions/panics

* If panics **abort** the process (no stack unwinding), mark functions that cannot unwind as **`nounwind`**.
* If an exception model is introduced later, do **not** lie with `nounwind`. Wrong there → miscompiles.

### 2.5 Concurrency & atomics

* Specify Desi’s memory model and map to LLVM atomics (`monotonic`, `acquire`, `release`, `acq_rel`, `seq_cst`).
* Avoid overusing `seq_cst`. Favor acquire/release where adequate.
* Use fences rarely; prefer atomic operations with the right orderings.

---

## 3) IR emission: the “make it easy to optimize” checklist

### 3.1 Values over memory

* Emit variables as SSA values. Use `alloca` for:

  * Address-taken locals, `inout` parameters lowered by address, and phi-demoted temps that LLVM can promote via **mem2reg/SROA**.
* Place `alloca` in the entry block. Annotate with `llvm.lifetime.start/end` for short-lived temps.

### 3.2 Control flow lowering

* **`match`**:

  * Dense integer/enum cases → a single **`switch`** (enables jump tables).
  * Sparse/guarded patterns → structured decision trees; avoid deep serial `if` chains when possible.
* **Comprehensions**:

  * Lower to canonical counted/iterator loops; emit loop metadata for vectorization/unrolling hints where the trip count is known or large.
* **Pipelines**:

  * Preserve SSA and avoid materializing tuples temporaries when arity lines up; let values flow.

### 3.3 Function boundaries & calling conventions

* External/FFI: use the platform **C calling convention** and match the ABI exactly.
* Internal: **`fastcc`** may be used for leaf helpers if it measurably helps; don’t fight the inliner.
* Multi-return:

  * Prefer a small **aggregate return** when ABI returns it in registers; otherwise lower to **`sret`** (hidden pointer to caller-allocated result) to avoid extra copies.
* Tail calls:

  * Use `musttail` only with ABI-exact signatures. Otherwise use `tail` as a hint where profitable.

### 3.4 Attributes: high-leverage contracts

**Function attributes**

* `readnone` / `readonly` / `writeonly`
* `nounwind`, `nosync`, `willreturn`, `nocallback` (for callbacks that never invoke user code)
* `cold` / `hot`
* `alwaysinline` / `noinline` (use sparingly)
* `"target-cpu"`, `"target-features"`; `"frame-pointer"="none"` in release

**Argument/return attributes**

* `noalias` (unique borrow/owned)
* `nonnull`, `noundef`, `dereferenceable(N)`, `align(N)`
* `byval`, `sret`, `nocapture` (for pointers that never escape)
* `swiftself`/`sret`/`inreg` as required by ABI choices

**Instruction/operation flags**

* `nsw`/`nuw`/`exact` on integer ops when semantically valid
* Fast-math flags on FP when explicitly enabled

### 3.5 Metadata & intrinsics

* **TBAA**: Provide a simple but consistent type tree; e.g.,

  * `tbaa_root` → `{object}` → `{struct Foo}` / `{i32}` / `{f64}` …
* **Loop metadata**: `llvm.loop.unroll.enable`, `unroll.disable`, `vectorize.enable`, `interleave.count`.
* **Branch weights**:

  * Use `llvm.expect` or `llvm.expect.with.probability` for obvious hot/cold predicates.
  * Attach `!prof` metadata to branches/switches in instrumentation builds.
* **`llvm.assume`**:

  * Encode facts proven by the type system or prior checks (e.g., length > 0 before a division/index).
* **Invariant loads**:

  * `!invariant.load` for truly immutable memory under the lifetime.
* **Memcpy/memset**:

  * Lower bulk copies/zeroing to `llvm.memcpy/memmove/memset` with correct alignment and `isvolatile=false`.

### 3.6 Globals & constants

* Mark literal aggregates as **`constant`** and **`unnamed_addr`** when their address is irrelevant.
* Use **`internal`** linkage for private helpers; do not export what you don’t need.

---

## 4) Data layout, strings, and collections

### 4.1 Data layout

* Trust the target’s **data layout string** from `TargetMachine`. Do not hand-roll packing rules.
* Only apply `packed` structs when ABI or foreign layout demands it; otherwise let the backend align naturally.

### 4.2 Strings

* Choose a canonical runtime string form (e.g., `(ptr,len)` UTF-8). For interop with C, provide zero-terminated views when needed.
* Literal strings **should** be emitted as module-local globals; avoid dynamic allocation for compile-time constants.

### 4.3 I/O boundary

* Provide builtins that lower to **simple libcalls for constant arguments** and to efficient `fwrite`/`write` for dynamic buffers to avoid format overhead.
* Keep wrappers thin or inlined so LLVM’s libcall simplifications can see through them.

### 4.4 TBAA policy

* Build a **stable TBAA tree** that mirrors Desi’s high-level types:

  * Scalar nodes for `i8/i16/i32/i64`, float nodes for `f32/f64`.
  * Aggregate nodes referencing their field nodes.
* In unsafe blocks that violate alias rules (e.g., byte-wise access to a `struct`), attach the **generic “char” TBAA**.

---

## 5) High-level construct lowerings

### 5.1 Match

* Prefer a single `switch` on normalized discriminants for enums/sum types.
* For guards, structure as early rejections to preserve a dense switch fast path.

### 5.2 Comprehensions

* Lower to loops with tight induction variables; enable vectorization by:

  * Hoisting bounds and invariant predicates.
  * Avoiding hidden function calls inside the loop body where possible.

### 5.3 Lambdas & closures

* Represent closures as `{env* + codeptr}` pairs.
* For non-escaping lambdas, elide heap allocation; pass captures by value or pointer with `nocapture` on callsites.

### 5.4 Async/await

* Stage-1: manual state machines that resemble LLVM’s coroutine shape (coalesce allocations, keep fields POD when possible).
* Later (discuss first): adopt **LLVM coroutines** if they prove beneficial for inlining and frame elision. Validate code size and debuggability impacts.

### 5.5 Using/RAII & defer

* Desugar to structured `try/finally` blocks in HIR; ensure the fast path is fall-through and mark cleanup paths **`cold`**.

---

## 6) Runtime & stdlib design for performance

* Keep hot helpers **inlineable** and side-effect annotated (`readnone/readonly` where true).
* Avoid over-abstracted I/O: prefer direct buffer operations over formatted variants for non-formatting prints.
* Separate **panic/error** paths into cold functions (`cold`, `noinline`), enabling size-saving and branch prediction.
* Enable dead-stripping: **`-ffunction-sections -fdata-sections`** with `--gc-sections` at link (lld).

---

## 7) Profiles, size, and post-link polish

### 7.1 PGO

* Support instrumentation-based and sample-based PGO (AutoFDO).
* Feed **branch weights** back into IR so inliner and code layout improve.

### 7.2 Code size modes

* `-Os/-Oz` imply: lower inlining thresholds, disable loop unrolling unless profitable, prefer library calls over expansions.

### 7.3 Post-link optimizers (nice-to-have, discuss first)

* **BOLT** (binary rewriter) can reorder hot code and improve I-cache; worthwhile for large services, not default.
* **CTF/CFI** or PAC features should be evaluated for security; expect minor perf trade-offs.

---

## 8) Diagnostics, testing, and regression guards

* Keep micro-benchmarks for:

  * tight loops (map/filter/comprehensions),
  * small string operations,
  * match/switch decision trees,
  * borrow-heavy code paths (alias-sensitive).
* Track optimization health with **remarks** and perf tests in CI for critical kernels.
* Provide **IR dumpers** (`--emit-ir`, `--emit-obj`, `--emit-asm`) to inspect codegen changes.

---

## 9) Do / Don’t quick reference

**Do**

* Emit accurate attributes: `noalias`, `nonnull`, `noundef`, `readonly/readnone`, `nounwind`.
* Use `llvm.lifetime.*`, `llvm.assume`, loop metadata, and `!prof`.
* Lower high-level constructs to recognizable canonical forms.
* Keep wrappers thin so the optimizer can see real work.

**Don’t**

* Lie with attributes (especially `noalias`/`nounwind`).
* Over-constrain with `volatile` (reserve it for true device/atomic cases).
* Force inlining everywhere or abuse `alwaysinline`.
* Materialize temporaries into memory when values suffice.

---

## 10) Open “discuss-first” items (not yet committed)

1. **Signed overflow semantics**: UB (max perf) vs defined wrap (safety/consistency). Huge downstream impact.
2. **Coroutine adoption**: stick to manual async lowering vs LLVM coroutine intrinsics pipeline.
3. **Default calling convention for internal funcs**: `fastcc` vs `ccc`.
4. **AutoFDO and BOLT** integration in release builds for “big binaries.”
5. **Advanced alias model**: richer TBAA (e.g., distinguishing mutable vs immutable borrows) for extra LICM/vectorization wins.
6. **Specialized small-vector/string optimizations** in stdlib (SSO, short-string elision) versus simplicity.

---

## 11) Minimal IR patterns (illustrative)

> These fragments demonstrate the **style** Desi should emit. They are not tied to any particular stdlib function names.

**Argument attributes & lifetime**

```llvm
define i64 @sum(ptr noalias noundef nonnull dereferenceable(32) %p, i64 %n) {
entry:
  call void @llvm.lifetime.start.p0(i64 32, ptr %p)
  ; ... loop using %p[0..n) ...
  call void @llvm.lifetime.end.p0(i64 32, ptr %p)
  ret i64 %acc
}
```

**Loop hints & branch prediction**

```llvm
br i1 %likely_true, label %hot, label %cold, !prof !0
!0 = !{!"branch_weights", i32 2000, i32 1}

; Or via intrinsic:
%cond = icmp ne i64 %x, 0
%biased = call i1 @llvm.expect.i1(i1 %cond, i1 true)
br i1 %biased, label %hot, label %cold
```

**Switch-friendly lowering**

```llvm
switch i32 %tag, label %default [
  i32 0, label %case0
  i32 1, label %case1
  i32 2, label %case2
]
```

---

## 12) Integration with the Roadmap

* **M7 (HIR & Tier-0 codegen)** will include: SSA-first IR emission, core attributes, basic TBAA root, lifetime intrinsics, canonical control-flow shapes, IR/obj/asm dump flags.
* **M8 (Async)** will lower async to state machines, marking cold paths and avoiding aliasing of the frame; coroutines are deferred pending evaluation.
* **M9–M10 (FFI & Prelude)** will ensure FFI surfaces retain optimizer visibility (thin wrappers, correct attributes), and prelude functions are annotated for IPO and libcall folds.
* **M12 (Diagnostics & Tooling)** will expose optimization remarks and perf microbench harnesses.

---

## 13) Summary

Desi’s codegen does not win by out-smarting LLVM. It wins by **telling LLVM the truth**—precisely, consistently, and with just enough structure that the middle-end can do its job. The items above are the contracts and shapes that make that happen. Implement them rigorously, and Desi will get “free” performance from the LLVM ecosystem while keeping the frontend simple and maintainable.
