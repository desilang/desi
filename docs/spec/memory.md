# Memory & Ownership (Stage-0)

Desi’s Stage-0 memory model is **value semantics with moves** and **ARC** (Automatic Reference Counting) for heap objects. It aims for predictable performance now, with room to evolve into a borrow-checked model later.

---

## Goals
- Deterministic, easy-to-reason lifetime.
- No hidden global GC pauses.
- Familiar “resource at scope end” behavior via `defer`.
- Portable lowering to C and later LLVM.

---

## Ownership & moves
- **Values move by default.** Assigning or passing a non-Copy value transfers ownership.
- **Scalars are Copy** (bitwise copy, no ownership transfer): `bool, i32, i64, u32, u64, u8, f64`.
- **Heap/backed types are Move** (owned): `str`, `Vec[T]`, `[]T` (slice), user `struct`/`enum` containing Move fields.

### Init vs assign
- `let x = expr` **initializes** a new binding.
- `x := expr` **reassigns** an existing binding.
- On reassignment, the previous value of `x` is **dropped** (ARC release if needed) before `x` takes ownership of the RHS.

---

## Passing & returning
- **By value** parameters move ownership for Move types.
- Returns transfer ownership to the caller.
- Copy types are duplicated on pass/return.

---

## Destruction & `defer`
- Each scope keeps a **cleanup stack**; `defer` pushes a thunk that runs on scope exit (LIFO).
- Lowering: the compiler emits calls to deferred thunks before every control-flow exit (return, break, error-prop via `?`, end of block).
- Dropping a value:
  - Copy types: no-op.
  - Move types: call type-specific destroyer (usually ARC `release`).

Example:
```desi
def use_file(path: str) -> Result[Unit, IoError]:
  let f = fs.open(path)?         # acquires resource
  defer f.close()                # guaranteed on every exit
  do_stuff(f)?
  Ok(())
````

---

## ARC (Automatic Reference Counting)

* Heap objects carry a refcount header. Stage-0 uses **atomic** increments/decrements for thread safety.
* The compiler inserts `retain/release` around ownership-affecting operations.
* **Escape analysis** (basic) removes redundant retains/releases for provably local values and stack-allocates short-lived aggregates when safe.

### Strings

* `str` is immutable; concatenation allocates a new buffer.
* Implementation: `{hdr*, u8* ptr, u32 len}` with shared buffers via ARC.

### Vectors

* `Vec[T] = {hdr*, T* data, u32 len, u32 cap}` (owning).
* Methods that grow may reallocate; elements are moved.

### Slices

* `[]T` is a **non-owning view** into contiguous memory. To keep the view valid:

  * Stage-0 slices carry a hidden **owner handle** (ARC retain on creation, release on drop) *or* they’re only formed from owners that outlive the slice (compiler can choose either strategy per lowering; the portable C backend uses the owner-handle form).
* Representation (C lowering): `{owner_rc*, T* ptr, u32 len}`.

---

## Mutation & aliasing (Stage-0 discipline)

* Mutation requires a **mutable binding**: `let mut v = ...`.
* Stage-0 does **not** enforce a full borrow checker. Guidelines:

  * Do not keep multiple mutable aliases to the same object alive.
  * Passing a mutable owner to functions should be structured to avoid aliasing.
* The compiler will implement **simple flow checks** and warn on obvious alias-then-mutate patterns; stricter checks come later.

---

## Closures & captures

* Closures capture by **move**. The environment is an owned struct.
* If a captured value is Move, the closure becomes Move.
* Dropping the closure drops its environment (ARC releases for heap members).

---

## Pattern matching drops

* In `match`, temporaries created for arm evaluation are dropped at the end of the arm.
* Bound names in a pattern own what they bind (subject to Copy/Move rules).

---

## Concurrency & memory

* ARC uses **atomic** refcounts so `str`/`Vec` can cross threads safely.
* Message passing transfers ownership of payloads into mailboxes (move semantics).
* Shared mutable state across tasks is discouraged in Stage-0; use channels/actors.

---

## FFI & `repr` (forward look)

* Stage-0 does not expose a stable memory layout for FFI.
* A future `repr(C)` will pin layouts for interop.

---

## Undefined behavior avoidance (lowering rules)

* Generated C never does pointer arithmetic outside valid objects.
* Strict aliasing pitfalls are avoided by using dedicated wrapper types and `memcpy` patterns where necessary.
* Bounds checks are emitted for slice/vec indexing in Stage-0 (subject to later optimization under LLVM).
