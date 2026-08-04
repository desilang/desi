# Desi vs C Benchmarks

Paired programs — each `<name>.desi` has an equivalent `<name>.c` doing the
same work with the same algorithm — measuring both **wall time** and **peak
memory** of the compiled executables.

## Running

```powershell
# Windows (requires bin\desic.exe built — run ..\build.ps1 first)
.\benchmarks\run_benchmarks.ps1            # both sides -O0
.\benchmarks\run_benchmarks.ps1 -Release   # both sides -O2
```

```sh
# macOS / Linux (requires make having been run)
./benchmarks/run_benchmarks.sh             # both sides -O0
./benchmarks/run_benchmarks.sh --release   # both sides -O2
```

Release mode maps to `desic build --release` (alias for `-O2`): the emitted
IR runs the full clang -O2 pipeline instead of -O0, and the C sides compile
with -O2 so parity holds.

Each program is run 3 times; the best wall time and the OS-reported peak
memory (PeakWorkingSet64 on Windows, max RSS via /usr/bin/time on Unix) are
reported. The runner warns if the Desi and C outputs differ.

## Fairness rules

- Both sides go through the **same clang at the same optimization level**:
  Desi's IR is compiled with clang's default (-O0), so the C sources are
  built with an explicit `-O0`. Neither side gets optimizations the other
  doesn't.
- The C code mirrors the Desi program's actual work — e.g. `string_churn.c`
  performs the same int→string conversion, concatenation, and free that the
  Desi compiler's hybrid memory management emits for transient string
  temporaries.
- Outputs must match exactly; arithmetic stays inside 32-bit range on both
  sides (Desi `int` is i32).

## How to read the table

Every row is a **whole-process wall time**, so every row includes the cost of
starting a process. The `startup` row measures exactly that and nothing else —
subtract it from both columns before comparing anything.

It matters more than it sounds. Measured on 2026-08-01:

| | Desi | C |
|---|---|---|
| Linux (WSL Ubuntu, -O2) | 1 ms | 1 ms |
| Windows (x64, -O2 + LTO) | 19.4 ms | 24.1 ms |

Two consequences:

- **Desi's startup is not a cost.** It matches C's on both platforms, so
  `__desi_runtime_init` is not what any gap is made of.
- **The Windows figures are mostly process creation.** A ~20 ms floor against
  totals of 25–50 ms leaves little room for the workload, so small differences
  there are noise. Prefer Linux when comparing the language; use Windows to
  catch a regression that is large enough to clear the floor.

Linux resolution is 1 ms (`date +%s%3N`), so treat any row at 1–3 ms as "at or
near parity" rather than a measured ratio.

## Benchmarks

| Name | What it measures |
|---|---|
| `startup` | The floor: process creation plus runtime init, no work at all |
| `loop_sum` | Raw arithmetic: 100M-iteration add loop |
| `fib_recursive` | Function-call overhead: naive fib(32), ~5.5M calls |
| `string_churn` | Allocation churn: 200k transient strings (str() + concat + free); peak memory must stay flat |
| `string_build` | Accumulator concat (`s := s + "x"`) — tracks the known phase-4 reassignment gap; expect Desi to lose it today |
| `alloc_churn` | 500k short-lived lists, freed per iteration by scope-exit drops |
| `list_ops` | Collections: 1M appends with growth, 1M indexed reads, scope-exit free |
| `dict_ops` | Hash map: 100k int-keyed inserts + 100k lookups vs open addressing |
| `matrix_mul` | Float math: 80x80 matrix multiply over list[list[float]] vs flat C arrays |
| `quicksort` | Sorting & In-place partitioning: 10k deterministic integers sorted with iterative QuickSort |
| `binary_tree` | Object/Class allocation & Tree traversal: 10k BST node insertion & in-order traversal |

## Reference numbers (Windows 11, x64, clang -O0)

Measured 2026-07-23 on the v0.1.0 dev branch (best of 4):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 63.9 ms | 61.3 ms | 2.8 MB / 3.2 MB |
| fib_recursive | 39.6 ms | 24.4 ms | 2.8 MB / 2.8 MB |
| string_churn | 36.6 ms | 43.7 ms | 2.8 MB / 2.8 MB |
| string_build | 18.3 ms | 19.3 ms | 3.2 MB / 2.1 MB |
| alloc_churn | 38.7 ms | 30.6 ms | 2.8 MB / 2.6 MB |
| list_ops | 29.0 ms | 24.3 ms | 2.1 MB / 2.1 MB |
| dict_ops | 24.6 ms | 24.3 ms | 2.0 MB / 6.8 MB |
| matrix_mul | 24.5 ms | 19.2 ms | 3.9 MB / 2.2 MB |
| quicksort | 20.9 ms | 21.2 ms | 2.5 MB / 2.0 MB |
| binary_tree | 20.6 ms | 20.6 ms | 2.0 MB / 2.0 MB |

Numbers vary by machine and run — the point is the *ratio*. Desi wins or
ties the arithmetic and string benchmarks outright and now uses LESS
memory than hand-rolled C on dict operations; the remaining gaps have
understood causes:

- **fib_recursive**: each Desi call runs the recursion-depth guard
  (`__desi_call_enter`/`__desi_call_exit`), which makes stack exhaustion
  a catchable RuntimeError — C has no equivalent safety.
- **alloc_churn**: a Desi list is header + data (single malloc, then the
  element array) plus a runtime call per append vs C's inlined store.
- **matrix_mul memory**: `list[list[float]]` boxes elements in 8-byte
  slots vs C's packed doubles — the planned packed-element-storage work
  closes this.

## Reference numbers (macOS 14.6, Apple Silicon, M-series, clang -O0)

Measured 2026-07-24 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 219.0 ms | 108.0 ms | 1.5 MB / 1.4 MB |
| fib_recursive | 47.0 ms | 24.0 ms | 1.5 MB / 1.4 MB |
| string_churn | 32.0 ms | 38.0 ms | 1.6 MB / 1.5 MB |
| string_build | 15.0 ms | 13.0 ms | 1.6 MB / 1.5 MB |
| alloc_churn | 35.0 ms | 25.0 ms | 1.5 MB / 1.5 MB |
| list_ops | 20.0 ms | 16.0 ms | 9.2 MB / 5.3 MB |
| dict_ops | 18.0 ms | 15.0 ms | 12.4 MB / 5.5 MB |
| matrix_mul | 17.0 ms | 15.0 ms | 2.1 MB / 1.6 MB |
| quicksort | 15.0 ms | 14.0 ms | 3.8 MB / 1.5 MB |
| binary_tree | 15.0 ms | 16.0 ms | 2.5 MB / 1.6 MB |

## Reference numbers (macOS 14.6, Apple Silicon, M-series, release -O2)

Measured 2026-07-24 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 13.0 ms | 13.0 ms | 1.5 MB / 1.4 MB |
| fib_recursive | 48.0 ms | 19.0 ms | 1.6 MB / 1.4 MB |
| string_churn | 39.0 ms | 38.0 ms | 1.7 MB / 1.5 MB |
| string_build | 14.0 ms | 13.0 ms | 1.6 MB / 1.5 MB |
| alloc_churn | 36.0 ms | 13.0 ms | 1.6 MB / 1.4 MB |
| list_ops | 19.0 ms | 14.0 ms | 9.2 MB / 5.3 MB |
| dict_ops | 18.0 ms | 14.0 ms | 12.4 MB / 5.5 MB |
| matrix_mul | 18.0 ms | 13.0 ms | 1.9 MB / 1.6 MB |
| quicksort | 15.0 ms | 14.0 ms | 1.7 MB / 1.5 MB |
| binary_tree | 15.0 ms | 14.0 ms | 1.7 MB / 1.6 MB |

## Reference numbers (Windows 11, x64, -O2 both sides + LTO hot set)

Measured 2026-07-23 on the v0.1.0 dev branch (best of 4):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 17.4 ms | 17.7 ms | 3.2 MB / 3.0 MB |
| fib_recursive | 35.5 ms | 22.6 ms | 2.8 MB / 2.8 MB |
| string_churn | 35.2 ms | 44.6 ms | 2.8 MB / 2.8 MB |
| string_build | 18.9 ms | 17.8 ms | 2.8 MB / 3.2 MB |
| alloc_churn | 42.8 ms | 20.9 ms | 2.8 MB / 2.8 MB |
| list_ops | 24.1 ms | 21.1 ms | 5.9 MB / 5.7 MB |
| dict_ops | 23.9 ms | 23.3 ms | 5.2 MB / 6.8 MB |
| matrix_mul | 18.6 ms | 17.7 ms | 3.3 MB / 3.2 MB |
| quicksort | 19.1 ms | 19.2 ms | 3.0 MB / 2.9 MB |
| binary_tree | 23.6 ms | 23.1 ms | 3.1 MB / 2.9 MB |

At -O2 Desi ties or beats C on loop_sum, string_churn, quicksort, and
matrix_mul, and holds within ~1.1x on the collection benchmarks — while
using less memory than C on dict_ops. string_build, once the worst
result in the suite at 43.5 MB leaked, is now at parity (the string
accumulator rewrite, below). The remaining time gaps are alloc_churn
(list allocation overhead) and fib_recursive (the recursion guard).

### String accumulators (why string_build stopped leaking)

The lowerer proves when a mutable string local is only ever used in
borrowing positions (concat/compare operands, f-strings, len, print,
return) and, only then, gives it owned semantics: the literal init
becomes a heap copy (`__desi_str_new`) and `s := s + x` lowers to
`__desi_str_append_free` — a realloc-based append that frees the old
value. One realloc per append instead of a fresh full-copy allocation,
and no leaked intermediates. Anything the analysis can't prove
(aliasing, user-call arguments, collection stores, `str(s)`, lambda
captures) disqualifies the variable and it keeps the leak-safe default
lowering — a wrong qualification would free memory something still
references, so every unknown is a "no" (`compiler/internal/lower/str_accum.go`).

### Allocator design (why dict wins and lists got cheaper)

- **Dicts**: entries are carved from pooled 64-entry blocks (one malloc
  per block, not per insert), values <= 8 bytes live inline in the entry
  (no value box), and the table rehashes at 0.75 load factor. An insert
  that used to cost 2 mallocs now costs ~1/64th of one. Entries never
  move (blocks are stable), so value pointers survive rehashing.
- **Lists**: the header and initial capacity share ONE malloc (data
  points just past the header); first growth moves data to its own
  block. Short-lived lists — the common case under scope-exit drops —
  cost one malloc/free instead of two.

### Call overhead (why fib_recursive closed)

Every user function carries a guard so that running out of stack raises a
`RuntimeError` instead of faulting. It used to be an exact frame counter —
a thread-local incremented on entry and decremented on return — and on a
benchmark that is nothing but calls, that guard *was* the gap: `fib_recursive`
spent about 70% of its time in it.

The counter is now a stack headroom check, which is stateless, so there is
no work to undo on return and the exit hook is gone entirely. Measured on
`fib(32)` at `-O2`, the guard costs 16.8 ms as a counter and 2.5 ms as a
headroom check. Three things mattered, in order:

- **Dropping the exit hook.** Most of it. Nothing is changed on entry, so
  nothing needs undoing.
- **Emitting the check inline** instead of calling a runtime helper. Worth
  ~4 ms on Linux, where no bitcode hot set exists to inline it away.
- **`thread_local(initialexec)`.** ELF's default general-dynamic model
  resolves a thread-local through a call to `__tls_get_addr`, which puts a
  call back in the prologue. Another ~4 ms on Linux.

`fib_recursive` went from 23 ms to 9 ms on Linux (C: 6 ms) and reached parity
on Windows. Nothing else in the table moved, which is what places the
remaining gaps — `alloc_churn`, `list_ops` — in allocation rather than call
overhead.

The check is also stricter than what it replaced: it measures the resource
that actually runs out, so large frames can no longer exhaust the stack
inside the frame budget, and a raised `set_recursion_limit` can no longer
authorise recursing past the end of the stack. See
[docs/contributing/compiler/limits.md](../docs/contributing/compiler/limits.md).

### How release builds work

`build.ps1` compiles a curated **LTO hot set** of runtime files
(recursion guard, dict/set, strings, rc, arena, print) a second time as
LLVM bitcode into `build/lto/`. Release links list those objects before
`libdesi.lib` (explicit objects win symbol resolution; the native archive
members from the same sources are never pulled) and pass
`-flto -fuse-ld=lld`, so the optimizer inlines hot runtime calls into
user code. Two lessons already encoded in the set:

- **Hot/cold splitting matters**: inlining the recursion guard originally
  *regressed* fib — its cold-path 256-byte message buffer landed in every
  caller's frame. The cold path is now a `noinline` function (limits.c).
- **Membership is benchmark-driven**: list.c measurably regressed when
  inlined (append/get bloat hot loop bodies beyond the call they save),
  so it stays native. Re-measure before changing the set in build.ps1.

### Measurement notes

The runner measures memory and time in SEPARATE runs: peak-working-set
sampling needs a polling loop whose sleep quantum (~15 ms on Windows)
would distort wall time, so timing runs use a bare `WaitForExit`. The
first (memory) run also absorbs Defender's first-launch scan of freshly
linked executables. Reported time is best of 4.

These benchmarks have already caught real bugs: dict_ops found both a
missing dict index-assignment lowering (access violation) and a hash
table that never rehashed (278x slowdown at 100k entries).
