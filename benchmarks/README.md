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

## Benchmarks

| Name | What it measures |
|---|---|
| `loop_sum` | Raw arithmetic: 100M-iteration add loop |
| `fib_recursive` | Function-call overhead: naive fib(32), ~5.5M calls |
| `string_churn` | Allocation churn: 200k transient strings (str() + concat + free); peak memory must stay flat |
| `string_build` | Accumulator concat (`s := s + "x"`) — tracks the known phase-4 reassignment gap; expect Desi to lose it today |
| `alloc_churn` | 500k short-lived lists, freed per iteration by scope-exit drops |
| `list_ops` | Collections: 1M appends with growth, 1M indexed reads, scope-exit free |
| `dict_ops` | Hash map: 100k int-keyed inserts + 100k lookups vs open addressing |
| `matrix_mul` | Float math: 80x80 matrix multiply over list[list[float]] (boxed elements) vs flat C arrays |

## Reference numbers (Windows 11, x64, clang -O0)

Measured 2026-07-17 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 62.2 ms | 69.5 ms | 3.2 MB / 3.2 MB |
| fib_recursive | 43.4 ms | 30.7 ms | 3.2 MB / 3.2 MB |
| string_churn | 44.6 ms | 47.4 ms | 3.2 MB / 3.2 MB |
| string_build | 46.0 ms | 18.6 ms | 51.2 MB / 3.2 MB |
| alloc_churn | 54.1 ms | 36.7 ms | 2.8 MB / 3.2 MB |
| list_ops | 28.0 ms | 23.0 ms | 11.9 MB / 7.9 MB |
| dict_ops | 38.0 ms | 27.9 ms | 12.4 MB / 6.8 MB |
| matrix_mul | 30.0 ms | 21.0 ms | 3.8 MB / 3.2 MB |

Numbers vary by machine and run — the point is the *ratio*. Desi wins or
ties the arithmetic and string-churn benchmarks outright; the collection
benchmarks sit within 1.2-1.5x of hand-rolled C with understood causes:

- **list/dict memory**: elements live in 8-byte generic slots vs C's
  packed types, and every op is a runtime call vs an inlined store.
  Typed element storage and codegen fast paths are the planned fixes.
- **fib_recursive**: each Desi call runs the recursion-depth guard
  (`__desi_call_enter`/`__desi_call_exit`); C has no equivalent safety.
- **alloc_churn**: a Desi list is two allocations (header + data) vs
  C's one malloc.
- **string_build**: the accumulator pattern reallocates the whole string
  per append and keeps intermediates alive until scope exit — the
  tracked hybrid-MM phase-4 item. This benchmark exists to watch that
  gap close.
## Reference numbers (macOS 14.6, Apple Silicon, M-series, clang -O0)

Measured 2026-07-18 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 224.0 ms | 108.0 ms | 1.5 MB / 1.4 MB |
| fib_recursive | 48.0 ms | 24.0 ms | 1.5 MB / 1.4 MB |
| string_churn | 33.0 ms | 40.0 ms | 1.5 MB / 1.5 MB |
| string_build | 20.0 ms | 13.0 ms | 51.8 MB / 1.5 MB |
| alloc_churn | 45.0 ms | 25.0 ms | 1.6 MB / 1.5 MB |
| list_ops | 23.0 ms | 16.0 ms | 9.2 MB / 5.4 MB |
| dict_ops | 20.0 ms | 14.0 ms | 13.2 MB / 5.5 MB |
| matrix_mul | 19.0 ms | 15.0 ms | 2.0 MB / 1.6 MB |

## Reference numbers (macOS 14.6, Apple Silicon, M-series, release -O2)

Measured 2026-07-18 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 14.0 ms | 13.0 ms | 1.5 MB / 1.5 MB |
| fib_recursive | 48.0 ms | 19.0 ms | 1.5 MB / 1.5 MB |
| string_churn | 32.0 ms | 37.0 ms | 1.5 MB / 1.5 MB |
| string_build | 20.0 ms | 14.0 ms | 51.8 MB / 1.5 MB |
| alloc_churn | 45.0 ms | 14.0 ms | 1.6 MB / 1.4 MB |
| list_ops | 19.0 ms | 14.0 ms | 9.2 MB / 5.3 MB |
| dict_ops | 20.0 ms | 14.0 ms | 12.0 MB / 5.4 MB |
| matrix_mul | 18.0 ms | 13.0 ms | 1.9 MB / 1.6 MB |

## Release mode (Windows 11, -O2 both sides + LTO hot set, same machine/day)

| Benchmark | Desi | C | Peak memory (Desi / C) | |
|---|---|---|---|---|
| loop_sum | 20.6 ms | 19.7 ms | 2.8 / 2.8 MB | parity |
| matrix_mul | 22.8 ms | 22.0 ms | 3.3 / 2.9 MB | parity |
| string_churn | 42.1 ms | 46.8 ms | 2.8 / 2.8 MB | **Desi wins** |
| dict_ops | 31.3 ms | 24.5 ms | **4.4** / 6.8 MB | Desi uses less memory |
| list_ops | 28.0 ms | 22.8 ms | 7.9 / 7.9 MB | 8-byte element slots |
| fib_recursive | 41.9 ms | 26.6 ms | 2.8 / 3.1 MB | recursion-guard cost |
| alloc_churn | 51.8 ms | 19.9 ms | 2.8 / 2.8 MB | 2 mallocs per list vs 1 |
| string_build | 46.5 ms | 18.7 ms | 51.2 / 2.9 MB | phase-4 tracker |

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
