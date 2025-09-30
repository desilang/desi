# Roadmap

This roadmap covers what we’ll build, when, and how—stage by stage. It keeps Desi’s surface simple (Python-y) while moving toward Rust-grade safety and modern tooling.

## Horizon view

* **Now** (M11–M12): async/await, channels/spawn, keep C backend.
* **Next** (M16–M20): docstrings + docgen, multiline strings, borrowing surface (`ref`/`inout`), ARC shim, LLVM backend (experimental).
* **Later** (M21+): toolchains/venv, package manager, fuller stdlib, self-hosting.

---

## Stage 1 — Frontend + C backend (current)

1. **Lexer/Parser (indentation aware) — done/ongoing**

  * Current grammar + M1–M10 features.
  * Continue expanding with async (M11), channels (M12).

2. **AST + Type Checker**

  * Structs, enums, imports, visibility ✅.
  * Add async/await lowering stubs & typing (M11).
  * Add channel types and `spawn` semantics (M12).

3. **C Emitter + Minimal Runtime**

  * Keep emitting portable C.
  * Strings/stdio helpers in `desi_std.h`.
  * Add async runtime shims (single-threaded executor) and basic channel impl (M11–M12).

---

## Stage 1.5 — Async & Concurrency

### M11 — `async` / `await` (minimal)

**What**

* Syntax: `async def f(...) -> T` and `await expr`.
* Transform `async def` into a state machine type + `poll` function.
* Add `Future<T>` protocol internally; user surface stays ergonomic.

**How**

* Parser: recognize `async def`.
* Checker: mark async functions; ensure `await` only in async contexts.
* Codegen (C): generate a small struct for state + a `switch(state)` poll; add an executor with a ready queue and timers.
* Std runtime: `io.sleep(ms)`, `io.read`, `io.write` async adapters.

**Acceptance**

* Examples compile/run: simple awaits, parallel awaits with `spawn` + `await join`.

### M12 — Channels & `spawn`

**What**

* `spawn f(args)` to run an async task.
* `chan[T]` (MPSC first), `send/recv` as async ops.
* Minimal backpressure (bounded channels later).

**How**

* Checker: type `chan[T]`, ensure `await ch.recv()`.
* Runtime: executor + channel implementation (lock-free MPSC or simple ring buffer).

**Acceptance**

* Ping-pong example; fan-in on a channel; producer/consumer with `await`.

---

## Stage 2 — Documentation & Language Ergonomics

### M16 — Docstrings & `desic doc`

**What**

* **Docstrings**: bare string literal immediately after module/def/struct/enum headers is lifted into `Doc`.
* `desic doc <entry>` generates Markdown (and `--json` for tools).

**How**

* AST: `Doc string` on `File`, `FuncDecl`, `StructDecl`, `EnumDecl`.
* Parser: lift first bare `StrLit` in each context; do not emit it as a statement.
* JSON: include `"doc"` fields.
* CLI: print Markdown grouped by sections.

**Acceptance**

* Examples produce clean docs; no codegen side-effects.

### M17 — Multiline strings

**What**

* Triple-quoted `""" ... """` literals for nicer docs/examples.

**How**

* Lexer: token for multiline strings with raw/newline support.
* Parser: treat as `StrLit`; doc extractor just works.

**Acceptance**

* Multiline docstrings preserved in JSON & docgen.

---

## Stage 3 — Ownership without Rust sigils

### M18 — Borrowing surface (frontend only)

**What**

* Keep syntax simple; **no `&`** at callsites.
* Function parameter annotations:

  * `T` — **owned** (move).
  * `ref T` — **shared borrow** (read-only).
  * `inout T` — **unique mutable borrow**.
* Frontend borrow rules:

  * Many `ref` **or** one `inout` active; not both.
  * Moved-from variables are invalid.
  * Returning borrows allowed only if they outlive the function (inferred).

**How**

* Type/kind system: `Owned<T>`, `Ref<T>`, `Inout<T>`.
* Checker: function-local borrow checker; track states {uninit, init, moved, borrowed(ro/rw)}; lifetimes via simple region inference.
* Diagnostics: “moved here”, “borrowed as inout here”, “still borrowed when mutated,” etc.

**Acceptance**

* Examples: `print_buf(ref)`, `clear(inout)`, move-then-use error; return borrowed parameter ok; return borrowed temporary rejected.

### M19 — ARC backend shim (C)

**What**

* Deterministic memory management without GC yet.
* Owned values perform retain/release on assignments/scope exit.
* Borrows do **not** retain; checker guarantees safety.

**How**

* Extend runtime with `desi_retain`, `desi_release` for heap-backed aggregates (strings, vec, future user types).
* Codegen inserts inc/dec for owned moves/copies; elide where provably unnecessary.

**Acceptance**

* Leak tests stable; no double-free; common paths optimized.

### M20 — LLVM backend (experimental)

**What**

* SSA lowering for better optimization & lifetime management.
* Same surface; codegen to LLVM IR (`llc` to native).

**How**

* IR builder for functions/blocks/alloca/promotions.
* Map borrow checker results to stack vs heap allocations efficiently.
* Hook sanitizers for early UB catches.

**Acceptance**

* Subset compiles with both backends; LLVM faster on microbenches.

---

## Stage 4 — Toolchains, Packages, and Stdlib

### M21 — Toolchains (“virtual env”)

**What**

* Hermetic, per-project toolchain/versioning à la `rustup` + Python venv.

**How**

* `desi.toml` (name, version, deps, `toolchain = "x.y.z"`, `std = "x.y"`).
* `desi.lock` (resolved versions + hashes).
* `desi use <ver>` pins toolchain into `.desi/` symlink.
* Home store: `~/.desi/toolchains/<ver>/...`.
* Env vars: `DESI_HOME`, `DESI_PATH`.

**Acceptance**

* Project pins a compiler; two projects can use different versions on the same machine.

### M22 — Package manager

**What**

* `desi pkg add util/json@^0.3`
* Cached in `~/.desi/cache/pkg/<name>/<ver>/`.
* Imports map to semantic packages, not raw paths.

**How**

* Semver resolver; lockfile write/read; integrity check via hashes.
* Simple registry API (local file index first; remote later).

**Acceptance**

* Basic add/update/remove; reproducible builds from lockfile.

### M23+ — Standard library growth (tracked with toolchain)

**Core**

* `prelude` (int/bool/str/result/option, traits later)
* `str` (len, concat, slicing, UTF-8 iter)
* `vec`, `map` (Robin Hood hashmap or SwissTable later)
* `mem` (hidden behind ownership rules)

**Systems**

* `io`, `fs`, `os`, `time`
* `json` (DOM first; streaming/serde-style later)
* `net` (tcp/udp/http minimal)
* Concurrency: `chan`, `task`, `runtime`, `timer`

**Acceptance**

* Each lib shipped versioned with toolchain; APIs doc-generated via `desic doc`.

---

## Stage 5 — Self-hosting compiler

**Goal**

* Stage-2 compiler written in Desi, bootstrapped by Stage-1 (Go).

**Approach**

* Start with a strict subset (no async, simplified ownership rules).
* Target C first; switch to LLVM once M20 is mature.
* CI builds both compilers; Stage-1 remains the reference bootstrap.

**Acceptance**

* Stage-2 compiles a meaningful subset of Desi sources (including itself progressively).
* Round-trip builds reproducible across platforms.

---

## Deliverables checklist per milestone

For each milestone above:

* **Grammar/AST** patches (when applicable).
* **Checker** features + diagnostics (with codes).
* **Codegen/runtime** changes.
* **Examples** under `examples/`.
* **Docs**: update `docs/goals/milestones.md` + new docs if needed.
* **Smoke tests** (CI or `scripts/smoke_examples.sh`).

---

## Sequencing summary (short)

* **M11** async/await
* **M12** channels + spawn
* **M16** docstrings + `desic doc`
* **M17** multiline strings
* **M18** `ref` / `inout` + borrow checker (frontend)
* **M19** ARC shim (C)
* **M20** LLVM backend (exp)
* **M21** toolchains/venv
* **M22** packages
* **M23+** stdlib growth
* **Self-host** when LLVM + borrowing are stable

