# Desi vs C Benchmarks

Paired programs — each `<name>.desi` has an equivalent `<name>.c` doing the
same work with the same algorithm — measuring both **wall time** and **peak
memory** of the compiled executables.

## Running

```powershell
# Windows (requires bin\desic.exe built — run ..\build.ps1 first)
.\benchmarks\run_benchmarks.ps1
```

```sh
# macOS / Linux (requires make having been run)
./benchmarks/run_benchmarks.sh
```

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
| `string_churn` | Allocation churn: 200k transient strings (str() + concat + free); peak memory must stay flat |
| `list_ops` | Collections: 1M appends with growth, 1M indexed reads, scope-exit free |

## Reference numbers (Windows 11, x64, clang -O0)

Measured 2026-07-17 on the v0.1.0 dev branch (best of 3):

| Benchmark | Desi | C | Peak memory (Desi / C) |
|---|---|---|---|
| loop_sum | 64.3 ms | 66.1 ms | 3.2 MB / 3.2 MB |
| string_churn | 43.1 ms | 47.8 ms | 3.2 MB / 3.2 MB |
| list_ops | 30.6 ms | 23.6 ms | 11.9 MB / 7.8 MB |

Numbers vary by machine — the point is the *ratio*: Desi's generated code
tracks C (loop_sum and string_churn are within noise of each other), and
churn-heavy programs hold a flat working set thanks to scope-exit drops.
list_ops' extra ~4 MB is the list representation: elements are stored in
8-byte `void*` slots vs C's packed 4-byte ints, plus `list_append` call
overhead vs an inlined array store.
