# Desi — Memory Management Plan & Optional GC Strategy

> Pythonic surface • Rust-like safety • Elixir-style async • Performance oriented.
> This document records the memory-management design direction (and the optional-GC ideas), so contributors have a single source of truth.
> **The implemented drop system is documented in [drops-implementation.md](drops-implementation.md)** — scope-exit drops for collections, enums, structs, and classes shipped in v0.1.0 (hybrid MM phases 1-2).

---

## TL;DR

- **Default:** Ownership + borrow checking (no global GC), deterministic destruction (RAII), fast FFI hygiene.
- **Sharing:** Opt-in `rc[T]` / `arc[T]` with `weak[T]` to break cycles.
- **Throughput:** Arenas/regions (`using arena:`) free en masse at scope-exit.
- **Async:** No `inout` borrows across `await`; escaping closures/futures are boxed & refcounted.
- **Maybe later:** An **optional, precise `gc[T]` island** for cycle-heavy graphs. The rest of the program remains non-GC.

---

## 1) Current Plan (M6 → M8)

### 1.1 Ownership & Borrows (M6)
- Parameter kinds: `T` (move), `ref T` (shared borrow), `inout T` (unique mutable borrow).
- Borrow checker (function-local in M6) enforces:
  - at most one `inout` or many `ref`;
  - no use-after-move; no mutation through shared borrows;
  - **no `inout` across `await`**.

> This eliminates most memory safety classes at compile time.

#### M6 polish (Batch-2)
- **Primitives are copy:** `int`, `float`, `bool`, `str` are treated as **copy types**. Passing them by value does **not** move the source; `DBR0004` (“moved earlier…”) should not fire for these.
- **`ref` requires an lvalue:** Calls passing a `ref` argument must use an lvalue (identifier/field/index with a named base). Non-lvalues/temporaries trigger **`DBR0005: ref argument must be an lvalue`**.
- **Alias diagnostics improved:** When `inout` aliases with another argument, **`DBR0003`** now includes a **secondary label** pointing at the *other* conflicting argument (“aliases with this argument”).

### 1.2 Deterministic Destruction (post-M6 lowering)
- The compiler will insert **drop** calls at scope exits (RAII).
- Resources (files, sockets) close promptly; FFI stays predictable (no “eventual finalizers”).

### 1.3 Opt-in Sharing with `rc/arc/weak` (M7)
- `rc[T]` (single-threaded) and `arc[T]` (atomic) smart pointers in the prelude.
- `weak[T]` to break cycles and side-step leaks.
- Lowering inserts `inc/dec` at SSA boundaries; DCAS not required (standard ARC).

**Guidance**
- Prefer `rc` for intra-thread ownership; use `arc` only for cross-thread sharing.
- Use `weak` for back-references (e.g., parent pointers in trees/graphs).

### 1.4 Arenas / Regions (M7)
- `arena` value with lexical lifetime:
```desi
using arena:
  let a = arena.alloc(Node(...))
  ...
# all memory from arena freed here
```

* Great for short-lived graphs, parsers, comprehensions; no per-object frees.

### 1.5 Strings & Small Containers

* `str` will be **immutable** with SSO + refcount (implementation detail).
* Lists/maps/sets live on stack/heap; choose **arena** or **rc/arc** based on sharing.

### 1.6 Async, Closures, Futures (M8)

* Functions lowered to state machines; lifetimes checked at suspension points.
* Capturing closures/futures that **escape** are heap-boxed and refcounted.
* Rule remains: **no `inout` across `await`**.

---

## 2) Why Not a Global GC by Default?

* **Latency & overhead:** barriers, metadata, write sets; STW pauses still happen.
* **FFI determinism:** finalizers are not prompt; external resources leak longer.
* **Zero-cost paths:** most Desi code doesn’t need GC; ownership + RC handles common cases.

---

## 3) Optional GC “Island” (Future)

> For code that is **naturally cyclic** and impractical to manage with `rc/weak`.

### 3.1 Shape

* Add `gc[T]` (opt-in wrapper). Objects allocated under `gc` live in a **precise** generational collector.
* The **rest of the program remains non-GC** (ownership/rc/arc/arenas unchanged).

### 3.2 Interop Boundaries

* Crossing **into** `gc`: `rc<T> → gc<T>` by move/clone (defined by library API).
* Crossing **out of** `gc`: expose **borrowed views** or explicit `clone()` back to `rc`/by-value.
* Avoid keeping raw pointers from non-GC land into moving `gc` objects; use handles.

### 3.3 Collector Options (phased)

* **Phase A (prototype):** Non-moving precise mark-sweep, **stop-the-world**, per-island heap. Simple write barrier for inter-island roots (if any).
* **Phase B:** Generational nursery + mark-compact or Immix; card table barrier.
* **Phase C:** Concurrent marking; STW only at short compaction/flip.

**Precision**

* Compiler will be able to emit stack maps (LLVM `gc.statepoint`) **if/when** a moving collector is chosen.
* Until then, a non-moving precise collector avoids relocating pointers.

### 3.4 Diagnostics & Budget

* Target short p99 pauses (<5ms for small heaps); **no global barrier** on non-gc paths.
* Telemetry hooks for allocation rate, promotion, and survivors.

---

## 4) Alternatives to Tracing GC for Cycles

* **Weak refs:** primary recommendation (`weak[T]`); break back-references.
* **Cycle-detecting RC:** optional library (trial deletion / deferred RC cycle detector). Slower than weak, simpler than tracing.
* **Arenas:** drop whole subgraphs; avoid arbitrary cycles.

---

## 5) FFI & ABI Considerations

* Deterministic drops mean resources are released promptly before FFI calls.
* In `gc` islands, keep foreign handles in roots or pin blocks during calls.
* Provide `unsafe` escape hatches explicitly; document invariants.

---

## 6) Testing & Benchmarks

* **Correctness suites:** borrow-checker golden tests (M6), async suspension points (M8).
* **Perf suites:** rc/arc micro-benchmarks; arena allocation throughput; optional `gc` nursery stress.
* **Latency tests:** measure tail latencies with tracing options when `gc[T]` is enabled.

---

## 7) Roadmap Hooks

* **M6:** Borrow checker (function-local) with `T/ref/inout`; async rule baked in.
* **M7:** HIR lowering inserts deterministic drops; implement `rc/arc/weak` + arenas.
* **M8:** Async state machines; verify borrow rules at suspension points.
* **M10/13:** Grow prelude; consider adding optional `gc[T]` as a separate package/runtime if real workloads demand it.

---

## 8) API Sketch (illustrative)

```desi
# Sharing
let p = rc(Node(...))
let q = weak(p)           # break cycles
match q.upgrade():
  Some(r):  print(r)
  None:     print("gone")

# Arenas
using arena:
  let a = arena.alloc(Node("a"))
  let b = arena.alloc(Node("b"))
# both freed here

# Optional GC island (future)
let g = gc(Graph())
g.insert(a, b)            # tracing-managed
```

---

## 9) Non-Goals (for now)

* No global, mandatory GC.
* No conservative stack scanning (precision is a design goal).
* No language-level finalizers; use RAII + explicit `close()` semantics.
