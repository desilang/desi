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

## Release mode (-O2 both sides, same machine/day)

| Benchmark | Desi | C | |
|---|---|---|---|
| loop_sum | 30.5 ms | 19.3 ms | 2.1x faster than Desi -O0 |
| string_churn | 48.1 ms | 54.2 ms | Desi wins |
| matrix_mul | 23.3 ms | 24.4 ms | Desi wins |
| dict_ops | 33.8 ms | 28.3 ms | |
| list_ops | 29.6 ms | 23.9 ms | |
| fib_recursive | 43.4 ms | 28.5 ms | recursion guard; -O2 can't remove it |
| alloc_churn | 58.5 ms | 17.8 ms | runtime calls opaque without LTO |
| string_build | 56.0 ms | 18.0 ms | phase-4 tracker |

The -O2 column shows exactly where the next levers are: benchmarks bound
by program-side loops (loop_sum, matrix_mul, string_churn) improve or win
outright, while benchmarks bound by *runtime calls* (alloc_churn, list,
dict) barely move — the optimizer can't see through calls into the
separately-compiled libdesi. Cross-module LTO (runtime built as LLVM
bitcode) is the planned fix for that class.

These benchmarks have already caught real bugs: dict_ops found both a
missing dict index-assignment lowering (access violation) and a hash
table that never rehashed (278x slowdown at 100k entries).
